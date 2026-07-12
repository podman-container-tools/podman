//go:build !remote && (linux || freebsd)

package compat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
)

const defaultStatsPeriod = 5 * time.Second

func StatsContainer(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)

	query := struct {
		Stream  bool `schema:"stream"`
		OneShot bool `schema:"one-shot"` // added schema for one shot
	}{
		Stream: true,
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	if query.Stream && query.OneShot { // mismatch. one-shot can only be passed with stream=false
		utils.Error(w, http.StatusBadRequest, define.ErrInvalidArg)
		return
	}

	name := utils.GetName(r)
	ctnr, err := runtime.LookupContainer(name)
	if err != nil {
		utils.ContainerNotFound(w, name, err)
		return
	}

	stats, err := ctnr.GetContainerStats(nil)
	if err != nil {
		utils.InternalServerError(w, fmt.Errorf("failed to obtain Container %s stats: %w", name, err))
		return
	}

	coder := json.NewEncoder(w)
	// Write header and content type.
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	// Set up JSON encoder for streaming.
	coder.SetEscapeHTML(true)
	var preRead time.Time
	var preCPUStats CPUStats
	if query.Stream {
		preRead = time.Now()
		preCPUStats = getPreCPUStats(stats)
	}

	onlineCPUs, err := libpod.GetOnlineCPUs(ctnr)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

streamLabel: // A label to flatten the scope
	select {
	case <-r.Context().Done():
		logrus.Debugf("Client connection (container stats) cancelled")

	default:
		stats, err = ctnr.GetContainerStats(stats)
		if err != nil {
			logrus.Errorf("Unable to get container stats: %v", err)
			return
		}
		s, err := statsContainerJSON(ctnr, stats, preCPUStats, onlineCPUs)
		if err != nil {
			return
		}
		s.Stats.PreRead = preRead

		var jsonOut any
		if utils.IsLibpodRequest(r) {
			jsonOut = s
		} else {
			jsonOut = DockerStatsJSON(s)
		}

		if err := coder.Encode(jsonOut); err != nil {
			logrus.Errorf("Unable to encode stats: %v", err)
			return
		}
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}

		if !query.Stream || query.OneShot {
			return
		}

		preRead = s.Read
		bits, err := json.Marshal(s.CPUStats)
		if err != nil {
			logrus.Errorf("Unable to marshal cpu stats: %q", err)
		}
		if err := json.Unmarshal(bits, &preCPUStats); err != nil {
			logrus.Errorf("Unable to unmarshal previous stats: %q", err)
		}
		time.Sleep(defaultStatsPeriod)
		goto streamLabel
	}
}
