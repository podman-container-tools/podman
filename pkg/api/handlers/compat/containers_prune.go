//go:build !remote && (linux || freebsd)

package compat

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
	"go.podman.io/podman/v6/pkg/domain/filters"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/util"
)

func PruneContainers(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	external := false
	if value := r.URL.Query().Get("external"); value != "" {
		var err error
		external, err = strconv.ParseBool(value)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("invalid external value: %w", err))
			return
		}
	}
	if external {
		if !utils.IsLibpodRequest(r) {
			utils.Error(w, http.StatusBadRequest, errors.New("external prune is only supported on libpod endpoints"))
			return
		}
		filtersMap, err := util.PrepareFilters(r)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
			return
		}
		engine := abi.ContainerEngine{Libpod: runtime}
		reports, err := engine.ContainerPrune(r.Context(), entities.ContainerPruneOptions{
			Filters:  url.Values(*filtersMap),
			External: true,
		})
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		utils.WriteResponse(w, http.StatusOK, reports)
		return
	}

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
