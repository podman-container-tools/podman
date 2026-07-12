//go:build !remote && (linux || freebsd)

package compat

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/moby/moby/api/types/jsonstream"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/libpod"
	handlers "go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/auth"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/storage"
)

// PushImage is the handler for the compat http endpoint for pushing images.
func PushImage(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	// Now use the ABI implementation to prevent us from having duplicate
	// code.
	imageEngine := abi.ImageEngine{Libpod: runtime}

	query := struct {
		All         bool   `schema:"all"`
		Compress    bool   `schema:"compress"`
		Destination string `schema:"destination"`
		Format      string `schema:"format"`
		TLSVerify   bool   `schema:"tlsVerify"`
		Tag         string `schema:"tag"`
	}{
		// This is where you can override the golang default value for one of fields
		TLSVerify: true,
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// Note that Docker's docs state "Image name or ID" to be in the path
	// parameter but it really must be a name as Docker does not allow for
	// pushing an image by ID.
	imageName := strings.TrimSuffix(utils.GetName(r), "/push") // GetName returns the entire path
	if query.Tag != "" {
		imageName += ":" + query.Tag
	}

	if _, err := utils.ParseStorageReference(imageName); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("image source %q is not a containers-storage-transport reference: %w", imageName, err))
		return
	}

	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, imageName)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}
	imageName = possiblyNormalizedName
	localImage, _, err := runtime.LibimageRuntime().LookupImage(possiblyNormalizedName, nil)
	if err != nil {
		utils.ImageNotFound(w, imageName, fmt.Errorf("failed to find image %s: %w", imageName, err))
		return
	}
	rawManifest, _, err := localImage.Manifest(r.Context())
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	authconf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)
	var username, password string
	if authconf != nil {
		username = authconf.Username
		password = authconf.Password
	}
	options := entities.ImagePushOptions{
		All:      query.All,
		Authfile: authfile,
		Compress: query.Compress,
		Format:   query.Format,
		Password: password,
		Username: username,
		Quiet:    true,
		Progress: make(chan types.ProgressProperties),
	}
	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	var destination string
	if _, found := r.URL.Query()["destination"]; found {
		destination = query.Destination
	} else {
		destination = imageName
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)

	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}

	statusWritten := false
	writeStatusCode := func(code int) {
		if !statusWritten {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			flush()
			statusWritten = true
		}
	}

	referenceWritten := false
	writeReference := func() {
		if !referenceWritten {
			var report jsonstream.Message
			report.Status = fmt.Sprintf("The push refers to repository [%s]", imageName)
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}
			flush()
			referenceWritten = true
		}
	}

	pushErrChan := make(chan error)
	var pushReport *entities.ImagePushReport
	go func() {
		var err error
		pushReport, err = imageEngine.Push(r.Context(), imageName, destination, options)
		pushErrChan <- err
	}()

loop: // break out of for/select infinite loop
	for {
		var report handlers.LegacyJSONMessage //nolint:staticcheck // LegacyJSONMessage is deprecated but kept for Docker API compat < v1.52

		select {
		case e := <-options.Progress:
			writeStatusCode(http.StatusOK)
			writeReference()
			switch e.Event {
			case types.ProgressEventNewArtifact:
				report.Status = "Preparing"
			case types.ProgressEventRead:
				report.Status = "Pushing"
				report.Progress = &jsonstream.Progress{
					Current: int64(e.Offset),
					Total:   e.Artifact.Size,
				}
				if _, err := apiutil.SupportedVersion(r, "<1.52.0"); err == nil {
					b, err := json.Marshal(&jsonstream.Message{
						Status:   report.Status,
						Progress: report.Progress,
						ID:       report.ID,
					})
					if err == nil {
						report.ProgressMessage = string(b)
					}
				}
			case types.ProgressEventSkipped:
				report.Status = "Layer already exists"
			case types.ProgressEventDone:
				report.Status = "Pushed"
			}
			report.ID = e.Artifact.Digest.Encoded()[0:12]
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}
			flush()
		case err := <-pushErrChan:
			if err != nil {
				code := http.StatusInternalServerError
				if errors.Is(err, storage.ErrImageUnknown) {
					// Image may have been removed in the meantime.
					code = http.StatusNotFound
					err = errors.New("An image does not exist locally with the tag: " + imageName)
				}
				if !statusWritten {
					utils.Error(w, code, err)
					flush()
					break loop
				}
				msg := err.Error()
				report.Error = &jsonstream.Error{
					Message: msg,
				}
				if _, err := apiutil.SupportedVersion(r, "<1.52.0"); err == nil {
					report.ErrorMessage = msg //nolint:staticcheck // deprecated field
				}
				if err := enc.Encode(report); err != nil {
					logrus.Warnf("Failed to json encode error %q", err.Error())
				}
				flush()
				break loop
			} else {
				writeStatusCode(http.StatusOK)
				writeReference() // There may not be any progress, so make sure the reference gets written
			}

			tag := query.Tag
			if tag == "" {
				tag = "latest"
			}

			var digestStr string
			if pushReport != nil {
				digestStr = pushReport.ManifestDigest
			}
			// Match Docker's behaviour: emit the manifest digest as an
			// `aux` JSON trailer at the end of the push stream so clients
			// can recover it without round-tripping the registry.
			auxPayload, auxErr := json.Marshal(struct {
				Tag    string `json:"Tag"`
				Digest string `json:"Digest"`
				Size   int    `json:"Size"`
			}{
				Tag:    tag,
				Digest: digestStr,
				Size:   len(rawManifest),
			})
			if auxErr == nil {
				raw := json.RawMessage(auxPayload)
				report.Aux = &raw
			}
			report.Status = fmt.Sprintf("%s: digest: %s size: %d", tag, digestStr, len(rawManifest))
			if err := enc.Encode(report); err != nil {
				logrus.Warnf("Failed to json encode error %q", err.Error())
			}

			flush()
			break loop // break out of for/select infinite loop
		}
	}
}
