//go:build !remote && (linux || freebsd)

package compat

import (
	"bytes"
	"encoding/json"
	"errors"
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
		err = fmt.Errorf("failed to obtain Container %s stats: %w", name, err)
		utils.Error(w, statsErrorStatus(err), err)
		return
	}
	onlineCPUs, err := libpod.GetOnlineCPUs(ctnr)
	if err != nil {
		utils.Error(w, statsErrorStatus(err), err)
		return
	}
	wroteContent := false

	// https://github.com/containers/podman/issues/24730
	// Docker always populates precpu_stats, even with stream=false.
	// Seed it here so non-streaming clients get non-zero precpu_stats.
	preRead := time.Now()
	preCPUStats := getPreCPUStats(stats)

streamLabel: // A label to flatten the scope
	select {
	case <-r.Context().Done():
		logrus.Debugf("Client connection (container stats) cancelled")

	default:
		stats, err = ctnr.GetContainerStats(stats)
		if err != nil {
			if wroteContent {
				logrus.Errorf("Unable to get container stats: %v", err)
			} else {
				utils.Error(w, statsErrorStatus(err), err)
			}
			return
		}
		s, err := statsContainerJSON(ctnr, stats, preCPUStats, onlineCPUs)
		if err != nil {
			if wroteContent {
				logrus.Errorf("Unable to build container stats response: %v", err)
			} else {
				utils.Error(w, statsErrorStatus(err), err)
			}
			return
		}
		s.Stats.PreRead = preRead

		var jsonOut any
		if utils.IsLibpodRequest(r) {
			jsonOut = s
		} else {
			jsonOut = DockerStatsJSON(s)
		}

		var chunk bytes.Buffer
		if err := json.NewEncoder(&chunk).Encode(jsonOut); err != nil {
			if wroteContent {
				logrus.Errorf("Unable to encode stats: %v", err)
			} else {
				utils.InternalServerError(w, err)
			}
			return
		}

		// Do not commit a successful response until the complete sample has
		// been collected and encoded. For a stream, each write is one complete
		// sample, so an error can only truncate the stream between samples.
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(chunk.Bytes()); err != nil {
			logrus.Errorf("Unable to write stats: %v", err)
			return
		}
		wroteContent = true
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
			return
		}
		if err := json.Unmarshal(bits, &preCPUStats); err != nil {
			logrus.Errorf("Unable to unmarshal previous stats: %q", err)
			return
		}
		time.Sleep(defaultStatsPeriod)
		goto streamLabel
	}
}

func statsErrorStatus(err error) int {
	switch {
	case errors.Is(err, define.ErrNoSuchCtr), errors.Is(err, define.ErrCtrRemoved):
		return http.StatusNotFound
	case errors.Is(err, define.ErrCtrStopped), errors.Is(err, define.ErrCtrStateInvalid), errors.Is(err, define.ErrNoCgroups):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
