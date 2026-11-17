//go:build !remote && (linux || freebsd)

package compat

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"

	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
	"go.podman.io/podman/v6/pkg/domain/filters"
	"go.podman.io/podman/v6/pkg/util"
)

func PruneContainers(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filtersMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	filterFuncs := make([]libpod.ContainerFilter, 0, len(*filtersMap))
	for k, v := range *filtersMap {
		generatedFunc, err := filters.GeneratePruneContainerFilterFuncs(k, v, runtime)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		filterFuncs = append(filterFuncs, generatedFunc)
	}

	report, err := PruneContainersHelper(r, filterFuncs)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	// Libpod response differs
	if utils.IsLibpodRequest(r) {
		utils.WriteResponse(w, http.StatusOK, report)
		return
	}

	var payload handlers.ContainersPruneReport
	var errorMsg bytes.Buffer
	for _, pr := range report {
		if pr.Err != nil {
			// Docker stops on first error vs. libpod which keeps going. Given API constraints, concatenate all errors
			// and return that string.
			errorMsg.WriteString(pr.Err.Error())
			errorMsg.WriteString("; ")
			continue
		}
		payload.ContainersDeleted = append(payload.ContainersDeleted, pr.Id)
		payload.SpaceReclaimed += pr.Size
	}
	if errorMsg.Len() > 0 {
		utils.InternalServerError(w, errors.New(errorMsg.String()))
		return
	}

	utils.WriteResponse(w, http.StatusOK, payload)
}

func PruneContainersHelper(r *http.Request, filterFuncs []libpod.ContainerFilter) ([]*reports.PruneReport, error) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	report, err := runtime.PruneContainers(filterFuncs)
	if err != nil {
		return nil, err
	}
	return report, nil
}
