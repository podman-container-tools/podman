//go:build !remote && (linux || freebsd)

package libpod

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"

	"github.com/gorilla/schema"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
	"go.podman.io/podman/v6/pkg/domain/filters"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/domain/infra/abi/parse"
	"go.podman.io/podman/v6/pkg/util"
)

func CreateVolume(w http.ResponseWriter, r *http.Request) {
	var (
		volumeOptions []libpod.VolumeCreateOption
		runtime       = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder       = r.Context().Value(api.DecoderKey).(*schema.Decoder)
	)
	query := struct{}{
		// override any golang type defaults
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	input := entities.VolumeCreateOptions{}
	if err := utils.ReadJSONFromBody(r, &input); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	if len(input.Name) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeName(input.Name))
	} else {
		volumeOptions = append(volumeOptions, libpod.WithVolumeAnonymous())
	}
	if len(input.Driver) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeDriver(input.Driver))
	}

	// Label provided for compatibility.
	labels := make(map[string]string, len(input.Label)+len(input.Labels))
	maps.Copy(labels, input.Label)
	maps.Copy(labels, input.Labels)
	if len(labels) > 0 {
		volumeOptions = append(volumeOptions, libpod.WithVolumeLabels(labels))
	}

	if len(input.Options) > 0 {
		parsedOptions, err := parse.VolumeOptions(input.Options)
		if err != nil {
			utils.InternalServerError(w, err)
			return
		}
		volumeOptions = append(volumeOptions, parsedOptions...)
	}

	if input.IgnoreIfExists {
		volumeOptions = append(volumeOptions, libpod.WithVolumeIgnoreIfExist())
	}

	if input.UID != nil {
		volumeOptions = append(volumeOptions, libpod.WithVolumeUID(*input.UID), libpod.WithVolumeNoChown())
	}
	if input.GID != nil {
		volumeOptions = append(volumeOptions, libpod.WithVolumeGID(*input.GID), libpod.WithVolumeNoChown())
	}

	vol, err := runtime.NewVolume(r.Context(), volumeOptions...)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	inspectOut, err := vol.Inspect()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	volResponse := entities.VolumeConfigResponse{
		InspectVolumeData: *inspectOut,
	}
	utils.WriteResponse(w, http.StatusCreated, volResponse)
}

func InspectVolume(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)
	vol, err := runtime.GetVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}
	inspectOut, err := vol.Inspect()
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	volResponse := entities.VolumeConfigResponse{
		InspectVolumeData: *inspectOut,
	}
	utils.WriteResponse(w, http.StatusOK, volResponse)
}

func ListVolumes(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	ic := abi.ContainerEngine{Libpod: runtime}
	volumeConfigs, err := ic.VolumeList(r.Context(), entities.VolumeListOptions{Filter: *filterMap})
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, volumeConfigs)
}

func PruneVolumes(w http.ResponseWriter, r *http.Request) {
	reports, err := pruneVolumesHelper(r)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, reports)
}

func pruneVolumesHelper(r *http.Request) ([]*reports.PruneReport, error) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		DryRun bool `schema:"dryrun"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		return nil, err
	}
	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		return nil, err
	}

	f := util.NormalizeVolumePruneFilters(url.Values(*filterMap))
	filterFuncs := []libpod.VolumeFilter{}
	for filter, filterValues := range f {
		filterFunc, err := filters.GeneratePruneVolumeFilters(filter, filterValues, runtime)
		if err != nil {
			return nil, err
		}
		filterFuncs = append(filterFuncs, filterFunc)
	}

	reports, err := runtime.PruneVolumes(r.Context(), filterFuncs, query.DryRun)
	if err != nil {
		return nil, err
	}
	return reports, nil
}

func RemoveVolume(w http.ResponseWriter, r *http.Request) {
	var (
		runtime = r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
		decoder = r.Context().Value(api.DecoderKey).(*schema.Decoder)
	)
	query := struct {
		Force   bool  `schema:"force"`
		Timeout *uint `schema:"timeout"`
	}{
		// override any golang type defaults
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusInternalServerError,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	name := utils.GetName(r)
	vol, err := runtime.LookupVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}
	if err := runtime.RemoveVolume(r.Context(), vol, query.Force, query.Timeout); err != nil {
		if errors.Is(err, define.ErrVolumeBeingUsed) {
			utils.Error(w, http.StatusConflict, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

// ExistsVolume check if a volume exists
func ExistsVolume(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	ic := abi.ContainerEngine{Libpod: runtime}
	report, err := ic.VolumeExists(r.Context(), name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	if !report.Value {
		utils.Error(w, http.StatusNotFound, define.ErrNoSuchVolume)
		return
	}
	utils.WriteResponse(w, http.StatusNoContent, "")
}

// ExportVolume exports a volume
func ExportVolume(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	vol, err := runtime.GetVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}

	contents, err := vol.Export()
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusOK, contents)
}

// ImportVolume imports a volume
func ImportVolume(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	name := utils.GetName(r)

	vol, err := runtime.GetVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}

	if r.Body == nil {
		utils.Error(w, http.StatusInternalServerError, errors.New("must provide tar file to import in request body"))
		return
	}
	defer r.Body.Close()

	if err := vol.Import(r.Body); err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, "")
}

// RenameVolume renames an existing volume.
func RenameVolume(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	name := utils.GetName(r)
	if name == "" {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("volume name must not be empty: %w", define.ErrInvalidArg))
		return
	}

	query := struct {
		NewName string `schema:"newName"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	vol, err := runtime.LookupVolume(name)
	if err != nil {
		utils.VolumeNotFound(w, name, err)
		return
	}

	if _, err := runtime.RenameVolume(r.Context(), vol, query.NewName); err != nil {
		switch {
		case errors.Is(err, define.ErrVolumeExists), errors.Is(err, define.ErrVolumeBeingUsed):
			utils.Error(w, http.StatusConflict, err)
		case errors.Is(err, define.ErrInvalidArg):
			utils.Error(w, http.StatusBadRequest, err)
		default:
			utils.InternalServerError(w, err)
		}
		return
	}

	utils.WriteResponse(w, http.StatusNoContent, nil)
}
