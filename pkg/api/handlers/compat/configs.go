//go:build !remote && (linux || freebsd)

package compat

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/util"
)

func ListConfigs(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filtersMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	listOptions := entities.ConfigListRequest{
		Filters: *filtersMap,
	}
	reports, err := ic.ConfigList(r.Context(), listOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if utils.IsLibpodRequest(r) {
		utils.WriteResponse(w, http.StatusOK, reports)
		return
	}
	compatReports := make([]entities.ConfigInfoReportCompat, 0, len(reports))
	for _, report := range reports {
		compatRep := entities.ConfigInfoReportCompat{
			ConfigInfoReport: *report,
			Version:          entities.ConfigVersion{Index: 1},
		}
		compatReports = append(compatReports, compatRep)
	}
	utils.WriteResponse(w, http.StatusOK, compatReports)
}

func InspectConfig(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	names := []string{name}
	query := struct {
		ShowConfig bool `schema:"showconfig"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	opts := entities.ConfigInspectOptions{}
	opts.ShowConfig = query.ShowConfig && utils.IsLibpodRequest(r)

	reports, errs, err := ic.ConfigInspect(r.Context(), names, opts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if len(errs) > 0 {
		utils.ConfigNotFound(w, name, errs[0])
		return
	}
	if len(reports) < 1 {
		utils.InternalServerError(w, err)
		return
	}
	if utils.IsLibpodRequest(r) {
		utils.WriteResponse(w, http.StatusOK, reports[0])
		return
	}
	compatReport := entities.ConfigInfoReportCompat{
		ConfigInfoReport: *reports[0],
		Version:          entities.ConfigVersion{Index: 1},
	}
	utils.WriteResponse(w, http.StatusOK, compatReport)
}

func RemoveConfig(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	opts := entities.ConfigRmOptions{}
	name := utils.GetName(r)
	ic := abi.ContainerEngine{Libpod: runtime}
	reports, err := ic.ConfigRm(r.Context(), []string{name}, opts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if reports[0].Err != nil {
		utils.ConfigNotFound(w, name, reports[0].Err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func CreateConfig(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	opts := entities.ConfigCreateOptions{}
	createParams := struct {
		*entities.ConfigCreateRequest
		Labels map[string]string `schema:"labels"`
	}{}

	if err := utils.ReadJSONFromBody(r, &createParams); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	if createParams.ConfigCreateRequest == nil {
		utils.Error(w, http.StatusBadRequest, errors.New("missing config name and data"))
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(createParams.Data)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("decoding config data: %w", err))
		return
	}
	reader := bytes.NewReader(decoded)
	opts.Driver = createParams.Driver.Name
	opts.Labels = createParams.Labels

	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.ConfigCreate(r.Context(), createParams.Name, reader, opts)
	if err != nil {
		if strings.Contains(err.Error(), "cannot use --ignore and --replace") {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		if strings.Contains(err.Error(), "config name in use") {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func UpdateConfig(w http.ResponseWriter, _ *http.Request) {
	utils.Error(w, http.StatusNotImplemented, errors.New("update is not supported"))
}
