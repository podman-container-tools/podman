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

func ListSecrets(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filtersMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	listOptions := entities.SecretListRequest{
		Filters: *filtersMap,
	}
	reports, err := ic.SecretList(r.Context(), listOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if utils.IsLibpodRequest(r) {
		utils.WriteResponse(w, http.StatusOK, reports)
		return
	}
	// Docker compat expects a version field that increments when the secret is updated
	// We currently can't update a secret, so we default the version to 1
	compatReports := make([]entities.SecretInfoReportCompat, 0, len(reports))
	for _, report := range reports {
		compatRep := entities.SecretInfoReportCompat{
			SecretInfoReport: *report,
			Version:          entities.SecretVersion{Index: 1},
		}
		compatReports = append(compatReports, compatRep)
	}
	utils.WriteResponse(w, http.StatusOK, compatReports)
}

func InspectSecret(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	names := []string{name}
	query := struct {
		ShowSecret bool `schema:"showsecret"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	ic := abi.ContainerEngine{Libpod: runtime}
	opts := entities.SecretInspectOptions{}
	opts.ShowSecret = query.ShowSecret

	reports, errs, err := ic.SecretInspect(r.Context(), names, opts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if len(errs) > 0 {
		utils.SecretNotFound(w, name, errs[0])
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
	// Docker compat expects a version field that increments when the secret is updated
	// We currently can't update a secret, so we default the version to 1
	compatReport := entities.SecretInfoReportCompat{
		SecretInfoReport: *reports[0],
		Version:          entities.SecretVersion{Index: 1},
	}
	utils.WriteResponse(w, http.StatusOK, compatReport)
}

func RemoveSecret(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	opts := entities.SecretRmOptions{}
	name := utils.GetName(r)
	ic := abi.ContainerEngine{Libpod: runtime}
	reports, err := ic.SecretRm(r.Context(), []string{name}, opts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	if reports[0].Err != nil {
		utils.SecretNotFound(w, name, reports[0].Err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, nil)
}

func CreateSecret(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	opts := entities.SecretCreateOptions{}
	createParams := struct {
		*entities.SecretCreateRequest
		Labels map[string]string `schema:"labels"`
	}{}

	if err := utils.ReadJSONFromBody(r, &createParams); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	decoded, _ := base64.StdEncoding.DecodeString(createParams.Data)
	reader := bytes.NewReader(decoded)
	opts.Driver = createParams.Driver.Name
	opts.Labels = createParams.Labels

	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.SecretCreate(r.Context(), createParams.Name, reader, opts)
	if err != nil {
		if strings.Contains(err.Error(), "secret name in use") {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, report)
}

func UpdateSecret(w http.ResponseWriter, _ *http.Request) {
	utils.Error(w, http.StatusNotImplemented, errors.New("update is not supported"))
}
