//go:build !remote && (linux || freebsd)

package compat

import (
	"fmt"
	"net/http"

	jsoniter "github.com/json-iterator/go"
	dockerTypes "github.com/moby/moby/api/types"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

// jsonSeqRecordSeparator starts every record of an application/json-seq stream (RFC 7464).
const jsonSeqRecordSeparator = 0x1E

// eventsStreamTypes are the media types a Docker client can ask for on GET /events.
var eventsStreamTypes = []string{
	dockerTypes.MediaTypeJSONLines,
	dockerTypes.MediaTypeNDJSON,
	dockerTypes.MediaTypeJSONSequence,
}

// eventsContentType picks the media type of the events stream like Docker does:
// the API version picks the default (application/json before v1.52,
// application/jsonl from v1.52 on), and an Accept header only overrides it when
// it explicitly asks for one of eventsStreamTypes. Libpod requests always get
// application/json.
func eventsContentType(r *http.Request) string {
	if utils.IsLibpodRequest(r) {
		return dockerTypes.MediaTypeJSON
	}
	fallback := dockerTypes.MediaTypeJSONLines
	if _, err := apiutil.SupportedVersion(r, "<1.52.0"); err == nil {
		fallback = dockerTypes.MediaTypeJSON
	}
	return apiutil.NegotiateMediaType(r, eventsStreamTypes, fallback)
}

// GetEvents endpoint serves both the docker-compatible one and the new libpod one
func GetEvents(w http.ResponseWriter, r *http.Request) {
	var (
		fromStart bool
		decoder   = utils.GetDecoder(r)
		runtime   = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		json      = jsoniter.ConfigCompatibleWithStandardLibrary // FIXME: this should happen on the package level
	)

	// NOTE: the "filters" parameter is extracted separately for backwards
	// compat via `filterFromRequest()`.
	query := struct {
		Since  string `schema:"since"`
		Until  string `schema:"until"`
		Stream bool   `schema:"stream"`
	}{
		Stream: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if len(query.Since) > 0 || len(query.Until) > 0 {
		fromStart = true
	}

	libpodFilters, err := util.FiltersFromRequest(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse filters for %s: %w", r.URL.String(), err))
		return
	}
	eventChannel := make(chan events.ReadResult)

	readOpts := events.ReadOptions{
		FromStart:    fromStart,
		Stream:       query.Stream,
		Filters:      libpodFilters,
		EventChannel: eventChannel,
		Since:        query.Since,
		Until:        query.Until,
	}
	err = runtime.Events(r.Context(), readOpts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	flush := func() {}
	if flusher, ok := w.(http.Flusher); ok {
		flush = flusher.Flush
	}

	contentType := eventsContentType(r)
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	flush()

	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-eventChannel:
			if !ok {
				return
			}
			if evt.Error != nil {
				logrus.Errorf("Unable to read event: %q", evt.Error)
				continue
			}
			if evt.Event == nil {
				continue
			}

			e := entities.ConvertToEntitiesEvent(*evt.Event)
			// Some events differ between Libpod and Docker endpoints.
			// Handle these differences for Docker-compat.
			if !utils.IsLibpodRequest(r) && e.Type == "image" && e.Action == "remove" {
				// Status is deprecated, but we still like to set it for consumers that might use it.
				e.Status = "delete" //nolint:staticcheck // deprecated field
				e.Action = "delete"
			}
			if !utils.IsLibpodRequest(r) && e.Action == "died" {
				e.Status = "die" //nolint:staticcheck // deprecated field
				e.Action = "die"
				e.Actor.Attributes["exitCode"] = e.Actor.Attributes["containerExitCode"]
			}

			// Remove fields which are not set in 1.52 and newer.
			if _, err := apiutil.SupportedVersion(r, ">=1.52.0"); err == nil && !apiutil.IsLibpodRequest(r) {
				e.Status = "" //nolint:staticcheck // deprecated field, cleared for API >= 1.52
				e.ID = ""     //nolint:staticcheck // deprecated field, cleared for API >= 1.52
				e.From = ""   //nolint:staticcheck // deprecated field, cleared for API >= 1.52
			}

			if contentType == dockerTypes.MediaTypeJSONSequence {
				if _, err := w.Write([]byte{jsonSeqRecordSeparator}); err != nil {
					logrus.Errorf("Unable to write record separator: %q", err)
					return
				}
			}
			if err := coder.Encode(e); err != nil {
				logrus.Errorf("Unable to write json: %q", err)
			}
			flush()
		}
	}
}
