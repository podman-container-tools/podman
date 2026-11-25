//go:build !remote && (linux || freebsd)

package compat

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/moby/moby/api/types/build"
	dockerImage "github.com/moby/moby/api/types/image"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/util"
)

func PruneImages(w http.ResponseWriter, r *http.Request) {
	var filters []string
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	for k, v := range *filterMap {
		for _, val := range v {
			filters = append(filters, fmt.Sprintf("%s=%s", k, val))
		}
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	pruneOptions := entities.ImagePruneOptions{Filter: filters}
	imagePruneReports, err := imageEngine.Prune(r.Context(), pruneOptions)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	idr := make([]dockerImage.DeleteResponse, 0, len(imagePruneReports))
	var reclaimedSpace uint64
	var errorMsg bytes.Buffer
	for _, p := range imagePruneReports {
		if p.Err != nil {
			// Docker stops on first error vs. libpod which keeps going. Given API constraints, concatenate all errors
			// and return that string.
			errorMsg.WriteString(p.Err.Error())
			errorMsg.WriteString("; ")
			continue
		}

		idr = append(idr, dockerImage.DeleteResponse{
			Deleted: p.Id,
		})
		reclaimedSpace += p.Size
	}
	if errorMsg.Len() > 0 {
		utils.InternalServerError(w, errors.New(errorMsg.String()))
		return
	}

	payload := handlers.ImagesPruneReport{
		PruneReport: dockerImage.PruneReport{
			ImagesDeleted:  idr,
			SpaceReclaimed: reclaimedSpace,
		},
	}
	utils.WriteResponse(w, http.StatusOK, payload)
}

// buildPruneFilters maps the Docker build cache filters that are forwarded to libimage
// onto libimage filter keys. Every other key is rejected, so that a caller can't widen
// the prune beyond dangling images.
var buildPruneFilters = map[string]string{
	"until":      "until", // libimage compares against creation time, not last use
	"unused-for": "until", // deprecated synonym for until
	"id":         "id",
	"label":      "label",
	"label!":     "label!",
}

// PruneBuildCache removes dangling images left behind by builds.
func PruneBuildCache(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := utils.GetDecoder(r)
	query := struct {
		All           bool  `schema:"all"`
		KeepStorage   int64 `schema:"keep-storage"`
		ReservedSpace int64 `schema:"reserved-space"`
		MaxUsedSpace  int64 `schema:"max-used-space"`
		MinFreeSpace  int64 `schema:"min-free-space"`
	}{}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	opts := entities.ImagePruneOptions{}
	if _, err := utils.SupportedVersion(r, ">=1.48.0"); err == nil || errors.Is(err, apiutil.ErrVersionNotGiven) {
		opts.ReservedSpace = query.ReservedSpace
		if opts.ReservedSpace == 0 {
			// keep-storage is deprecated, only used when reserved-space is not set
			opts.ReservedSpace = query.KeepStorage
		}
		opts.MaxUsedSpace = query.MaxUsedSpace
		opts.MinFreeSpace = query.MinFreeSpace
	} else {
		// only keep-storage was valid before 1.48
		opts.ReservedSpace = query.KeepStorage
	}
	if opts.ReservedSpace < 0 || opts.MaxUsedSpace < 0 || opts.MinFreeSpace < 0 {
		utils.Error(w, http.StatusBadRequest, errors.New("space limits must not be negative"))
		return
	}
	hasLimits := opts.ReservedSpace != 0 || opts.MaxUsedSpace != 0 || opts.MinFreeSpace != 0

	filterMap, err := util.PrepareFilters(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse filters for %s: %w", r.URL.String(), err))
		return
	}
	if len((*filterMap)["until"]) > 0 && len((*filterMap)["unused-for"]) > 0 {
		utils.Error(w, http.StatusBadRequest, errors.New(`conflicting filters "until" and "unused-for"`))
		return
	}
	for k, vals := range *filterMap {
		libimageKey, ok := buildPruneFilters[k]
		if !ok {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("unsupported build cache prune filter %q", k))
			return
		}
		if len(vals) > 1 && k != "label" && k != "label!" {
			utils.Error(w, http.StatusBadRequest, fmt.Errorf("filter %q accepts only one value", k))
			return
		}
		for _, v := range vals {
			if strings.Contains(v, "!=") {
				utils.Error(w, http.StatusBadRequest, fmt.Errorf("invalid value %q for filter %q", v, k))
				return
			}
			opts.Filter = append(opts.Filter, libimageKey+"="+v)
		}
	}

	// Never set opts.All, that would prune tagged images. Cache mounts are
	// removed wholesale and can't be sized, so skip them when a limit is set.
	opts.BuildCache = query.All && !hasLimits

	imageEngine := abi.ImageEngine{Libpod: runtime}
	pruneReports, err := imageEngine.Prune(r.Context(), opts)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	report := build.CachePruneReport{CachesDeleted: []string{}}
	var errs []error
	for _, p := range pruneReports {
		if p.Err != nil {
			errs = append(errs, p.Err)
			continue
		}
		report.CachesDeleted = append(report.CachesDeleted, p.Id)
		report.SpaceReclaimed += p.Size
	}
	if len(errs) > 0 {
		utils.InternalServerError(w, errors.Join(errs...))
		return
	}
	utils.WriteResponse(w, http.StatusOK, handlers.BuildCachePruneReport{CachePruneReport: report})
}
