//go:build !remote && (linux || freebsd)

package compat

import (
	"fmt"
	"net/http"

	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

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

	w.Header().Set("Content-Type", "application/json")
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
			if evt.Event.Status == events.HealthStatus && !evt.Event.HealthStatusChanged {
				// Docker will only emit event when there is a state change
				continue
			}

			e := entities.ConvertToEntitiesEvent(*evt.Event)
			// Some events differ between Libpod and Docker endpoints.
			// Handle these differences for Docker-compat.
			if !utils.IsLibpodRequest(r) && e.Type == "image" && e.Action == "remove" {
				// Status is deprecated, but we still like to set it for consumers that might use it.
				//nolint:staticcheck,nolintlint // we run the linter several times and sometimes it
				// complains about this and sometimes it doesn't thus the nolintlint
				e.Status = "delete"
				e.Action = "delete"
			}
			if !utils.IsLibpodRequest(r) && e.Action == "died" {
				//nolint:staticcheck,nolintlint // we run the linter several times and sometimes it
				// complains about this and sometimes it doesn't thus the nolintlint
				e.Status = "die"
				e.Action = "die"
				e.Actor.Attributes["exitCode"] = e.Actor.Attributes["containerExitCode"]
			}

			// Remove fields which are not set in 1.52 and newer.
			if _, err := apiutil.SupportedVersion(r, ">=1.52.0"); err == nil && !apiutil.IsLibpodRequest(r) {
				e.Status = "" //nolint:staticcheck
				e.ID = ""     //nolint:staticcheck
				e.From = ""   //nolint:staticcheck
			}

			if err := coder.Encode(e); err != nil {
				logrus.Errorf("Unable to write json: %q", err)
			}
			flush()
		}
	}
}
