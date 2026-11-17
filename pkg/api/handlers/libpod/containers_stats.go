//go:build !remote && (linux || freebsd)

package libpod

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/schema"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
)

func StatsContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		Containers []string `schema:"containers"`
		Stream     bool     `schema:"stream"`
		Interval   int      `schema:"interval"`
		All        bool     `schema:"all"`
	}{
		Stream:   true,
		Interval: 5,
		All:      false,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// Reduce code duplication and use the local/abi implementation of
	// container stats.
	containerEngine := abi.ContainerEngine{Libpod: runtime}

	statsOptions := entities.ContainerStatsOptions{
		Stream:   query.Stream,
		Interval: query.Interval,
		All:      query.All,
	}

	// Stats will stop if the connection is closed.
	statsChan, err := containerEngine.ContainerStats(r.Context(), query.Containers, statsOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	wroteContent := false
	// Set up JSON encoder for streaming.
	coder := json.NewEncoder(w)
	coder.SetEscapeHTML(true)

	for stats := range statsChan {
		if !wroteContent {
			if stats.Error != nil {
				utils.ContainerNotFound(w, "", stats.Error)
				return
			}
			// Write header and content type.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			wroteContent = true
		}

		if err := coder.Encode(stats); err != nil {
			// Note: even when streaming, the stats goroutine will
			// be notified (and stop) as the connection will be
			// closed.
			logrus.Errorf("Unable to encode stats: %v", err)
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
