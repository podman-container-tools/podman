//go:build !remote && (linux || freebsd)

package libpod

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/auth"
	"go.podman.io/podman/v6/pkg/channel"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// The duration for which we are willing to wait before starting the stream, to be able to decide the HTTP status code more accurately.
const maximalStreamingDelay = 5 * time.Second

// ImagesPull is the v2 libpod endpoint for pulling images.  Note that the
// mandatory `reference` must be a reference to a registry (i.e., of docker
// transport or be normalized to one).  Other transports are rejected as they
// do not make sense in a remote context.
func ImagesPull(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	query := struct {
		AllTags      bool   `schema:"allTags"`
		CompatMode   bool   `schema:"compatMode"`
		PullProgress bool   `schema:"pullProgress"`
		PullPolicy   string `schema:"policy"`
		Quiet        bool   `schema:"quiet"`
		Reference    string `schema:"reference"`
		Retry        uint   `schema:"retry"`
		RetryDelay   string `schema:"retrydelay"`
		TLSVerify    bool   `schema:"tlsVerify"`
		// Platform fields below:
		Arch    string `schema:"Arch"`
		OS      string `schema:"OS"`
		Variant string `schema:"Variant"`
	}{
		TLSVerify:  true,
		PullPolicy: "always",
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if query.CompatMode && query.PullProgress {
		utils.Error(w, http.StatusBadRequest, errors.New("'compatMode' and 'pullProgress' cannot be used simultaneously"))
		return
	}

	if query.CompatMode && query.Quiet {
		utils.Error(w, http.StatusBadRequest, errors.New("'compatMode' and 'quiet' cannot be used simultaneously"))
		return
	}

	if query.PullProgress && query.Quiet {
		utils.Error(w, http.StatusBadRequest, errors.New("'pullProgress' and 'quiet' cannot be used simultaneously"))
		return
	}

	if len(query.Reference) == 0 {
		utils.Error(w, http.StatusBadRequest, errors.New("reference parameter cannot be empty"))
		return
	}

	// Make sure that the reference has no transport or the docker one.
	if err := utils.IsRegistryReference(query.Reference); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	pullOptions := &libimage.PullOptions{}
	pullOptions.AllTags = query.AllTags
	pullOptions.Architecture = query.Arch
	pullOptions.OS = query.OS
	pullOptions.Variant = query.Variant

	if _, found := r.URL.Query()["tlsVerify"]; found {
		pullOptions.InsecureSkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	// Do the auth dance.
	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	pullOptions.AuthFilePath = authfile
	if authConf != nil {
		pullOptions.Username = authConf.Username
		pullOptions.Password = authConf.Password
		pullOptions.IdentityToken = authConf.IdentityToken
	}

	pullPolicy, err := config.ParsePullPolicy(query.PullPolicy)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	if _, found := r.URL.Query()["retry"]; found {
		pullOptions.MaxRetries = &query.Retry
	}

	if _, found := r.URL.Query()["retrydelay"]; found {
		duration, err := time.ParseDuration(query.RetryDelay)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		pullOptions.RetryDelay = &duration
	}

	// Let's keep thing simple when running in quiet mode and pull directly.
	if query.Quiet {
		images, err := runtime.LibimageRuntime().Pull(r.Context(), query.Reference, pullPolicy, pullOptions)
		if err != nil {
			utils.Error(w, utils.HTTPStatusFromRegistryError(err), err)
			return
		}

		var report entities.ImagePullReport
		for _, image := range images {
			report.Images = append(report.Images, image.ID())
			// Pull last ID from list and publish in 'id' stanza.  This maintains previous API contract
			report.ID = image.ID()
		}
		utils.WriteResponse(w, http.StatusOK, report)
		return
	}

	if query.CompatMode {
		utils.CompatPull(r, w, runtime, query.Reference, pullPolicy, pullOptions)
		return
	}

	writer := channel.NewWriter(make(chan []byte))
	defer writer.Close()
	pullOptions.Writer = writer

	progress := make(chan types.ProgressProperties)
	pullOptions.Progress = progress

	var pulledImages []*libimage.Image
	var pullError error
	runCtx, cancel := context.WithCancel(r.Context())
	go func() {
		defer cancel()
		pulledImages, pullError = runtime.LibimageRuntime().Pull(runCtx, query.Reference, pullPolicy, pullOptions)
	}()

	flush := func() {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)

	streamingStarted := false
	var reportBuffer []entities.ImagePullReport

	// This timer ensures that streaming is not delayed indefinitely.
	streamingDelayTimer := time.NewTimer(maximalStreamingDelay)

	// Streaming is delayed to choose the HTTP status code more accurately (e.g. if the image does not exist at all).
	// Once a message is streamed, it is no longer possible to change the status code.
	startStreaming := func() {
		if !streamingStarted {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			for _, report := range reportBuffer {
				if err := enc.Encode(report); err != nil {
					logrus.Warnf("Failed to encode json: %v", err)
				}
				flush()
			}
			streamingStarted = true
		}
	}

	reportPullProgress := func(e types.ProgressProperties) {
		var report entities.ImagePullReport
		report.Status = entities.ImagePullStatusPulling
		report.Progress = &entities.ArtifactPullProgress{
			ProgressComponentID: e.Artifact.Digest.String(),
		}
		switch e.Event {
		case types.ProgressEventNewArtifact:
			report.Stream = "Starting to pull artifact"
			report.Progress.Status = entities.ArtifactPullStatusPulling
			report.Progress.Current = e.Offset
			report.Progress.Total = e.Artifact.Size
		case types.ProgressEventRead:
			report.Stream = "Pulling artifact"
			report.Progress.Status = entities.ArtifactPullStatusPulling
			report.Progress.Current = e.Offset
			report.Progress.Total = e.Artifact.Size
		case types.ProgressEventDone:
			report.Stream = "Artifact pulled successfully"
			report.Progress.Status = entities.ArtifactPullStatusSuccess
			report.Progress.Current = e.Offset
			report.Progress.Total = e.Artifact.Size
		case types.ProgressEventSkipped:
			report.Stream = "Artifact already exists"
			report.Progress.Status = entities.ArtifactPullStatusSkipped
		}
		if err := enc.Encode(report); err != nil {
			logrus.Errorf("Error encoding pull report: %v", err)
			return
		}
		flush()
	}

	for {
		select {
		case e := <-progress:
			startStreaming() // We are reporting progress working with the image contents, so it presumably exists and it is accessible.
			if query.PullProgress {
				reportPullProgress(e)
			}
		case <-streamingDelayTimer.C:
			startStreaming() // At some point, give up on the precise error code and let the caller show an “operation in progress, no data available yet” UI.
		case s := <-writer.Chan():
			report := entities.ImagePullReport{
				Status: entities.ImagePullStatusPulling,
				Stream: string(s),
			}
			if streamingStarted {
				if err := enc.Encode(report); err != nil {
					logrus.Warnf("Failed to encode json: %v", err)
				}
				flush()
			} else {
				reportBuffer = append(reportBuffer, report)
			}
		case <-runCtx.Done():
			if !streamingStarted && pullError != nil {
				utils.Error(w, utils.HTTPStatusFromRegistryError(pullError), pullError)
				return
			}

			startStreaming()
			var report entities.ImagePullReport
			for _, image := range pulledImages {
				report.Images = append(report.Images, image.ID())
				// Pull last ID from list and publish in 'id' stanza.  This maintains previous API contract
				report.ID = image.ID()
			}
			if pullError != nil {
				report.Status = entities.ImagePullStatusError
				report.Error = pullError.Error()
			} else {
				report.Status = entities.ImagePullStatusSuccess
			}
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to encode json: %v", err)
			}
			flush()
			return
		case <-r.Context().Done():
			// Client has closed connection
			return
		}
	}
}
