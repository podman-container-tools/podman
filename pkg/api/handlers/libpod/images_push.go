//go:build !remote && (linux || freebsd)

package libpod

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/auth"
	"go.podman.io/podman/v6/pkg/channel"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
)

// PushImage is the handler for the compat http endpoint for pushing images.
func PushImage(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		All                    bool   `schema:"all"`
		CompressionFormat      string `schema:"compressionFormat"`
		CompressionLevel       *int   `schema:"compressionLevel"`
		ForceCompressionFormat bool   `schema:"forceCompressionFormat"`
		Destination            string `schema:"destination"`
		Format                 string `schema:"format"`
		RemoveSignatures       bool   `schema:"removeSignatures"`
		Retry                  uint   `schema:"retry"`
		RetryDelay             string `schema:"retryDelay"`
		TLSVerify              bool   `schema:"tlsVerify"`
		Quiet                  bool   `schema:"quiet"`
	}{
		TLSVerify: true,
		// #14971: older versions did not sent *any* data, so we need
		//         to be quiet by default to remain backwards compatible
		Quiet: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	source := strings.TrimSuffix(utils.GetName(r), "/push") // GetName returns the entire path
	if _, err := utils.ParseStorageReference(source); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	destination := query.Destination
	if destination == "" {
		destination = source
	}

	if err := utils.IsRegistryReference(destination); err != nil {
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
		All:                    query.All,
		Authfile:               authfile,
		CompressionFormat:      query.CompressionFormat,
		CompressionLevel:       query.CompressionLevel,
		ForceCompressionFormat: query.ForceCompressionFormat,
		Format:                 query.Format,
		Password:               password,
		Quiet:                  query.Quiet,
		RemoveSignatures:       query.RemoveSignatures,
		RetryDelay:             query.RetryDelay,
		Username:               username,
	}

	if _, found := r.URL.Query()["retry"]; found {
		options.Retry = &query.Retry
	}

	if _, found := r.URL.Query()["compressionFormat"]; found {
		if _, foundForceCompression := r.URL.Query()["forceCompressionFormat"]; !foundForceCompression {
			// If `compressionFormat` is set and no value for `forceCompressionFormat`
			// is selected then default has to be `true`.
			options.ForceCompressionFormat = true
		}
	}

	if _, found := r.URL.Query()["tlsVerify"]; found {
		options.SkipTLSVerify = types.NewOptionalBool(!query.TLSVerify)
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	// Let's keep thing simple when running in quiet mode and push directly.
	if query.Quiet {
		_, err := imageEngine.Push(r.Context(), source, destination, options)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("pushing image %q: %w", destination, err))
			return
		}
		utils.WriteResponse(w, http.StatusOK, "")
		return
	}

	writer := channel.NewWriter(make(chan []byte))
	defer writer.Close()
	options.Writer = writer

	pushCtx, pushCancel := context.WithCancel(r.Context())
	var pushError error
	var pushReport *entities.ImagePushReport
	go func() {
		defer pushCancel()
		pushReport, pushError = imageEngine.Push(pushCtx, source, destination, options)
	}()

	flush := func() {
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	flush()

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	for {
		var stream entities.ImagePushStream
		select {
		case s := <-writer.Chan():
			stream.Stream = string(s)
			if err := enc.Encode(stream); err != nil {
				logrus.Warnf("Failed to encode json: %v", err)
			}
			flush()
		case <-pushCtx.Done():
			if pushReport != nil {
				stream.ManifestDigest = pushReport.ManifestDigest
			}
			if pushError != nil {
				stream.Error = pushError.Error()
			}
			if err := enc.Encode(stream); err != nil {
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
