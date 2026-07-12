//go:build !remote && (linux || freebsd)

package compat

import (
	"net/http"
	"strings"

	"github.com/moby/moby/api/types/build"
	dockerContainer "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	dockerSystem "github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
)

func GetDiskUsage(w http.ResponseWriter, r *http.Request) {
	options := entities.SystemDfOptions{}
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	ic := abi.ContainerEngine{Libpod: runtime}
	df, err := ic.SystemDf(r.Context(), options)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	ctnrs := make([]dockerContainer.Summary, len(df.Containers))
	for i, o := range df.Containers {
		t := dockerContainer.Summary{
			ID:         o.ContainerID,
			Names:      []string{o.Names},
			Image:      o.Image,
			ImageID:    o.Image,
			Command:    strings.Join(o.Command, " "),
			Created:    o.Created.Unix(),
			Ports:      nil,
			SizeRw:     o.RWSize,
			SizeRootFs: o.Size,
			Labels:     map[string]string{},
			State:      dockerContainer.ContainerState(o.Status),
			Status:     o.Status,
			HostConfig: struct {
				NetworkMode string            `json:",omitempty"`
				Annotations map[string]string `json:",omitempty"`
			}{},
			NetworkSettings: nil,
			Mounts:          nil,
		}
		ctnrs[i] = t
	}

	vols := make([]volume.Volume, len(df.Volumes))
	for i, o := range df.Volumes {
		t := volume.Volume{
			CreatedAt:  "",
			Driver:     "",
			Labels:     map[string]string{},
			Mountpoint: "",
			Name:       o.VolumeName,
			Options:    nil,
			Scope:      "local",
			Status:     nil,
			UsageData: &volume.UsageData{
				RefCount: int64(o.Links),
				Size:     o.Size,
			},
		}
		vols[i] = t
	}

	imgs_base := make([]image.Summary, len(df.Images))
	for i, o := range df.Images {
		imgs_base[i] = image.Summary{
			Containers:  int64(o.Containers),
			Created:     o.Created.Unix(),
			ID:          o.ImageID,
			Labels:      map[string]string{},
			ParentID:    "",
			RepoDigests: nil,
			RepoTags:    []string{o.Tag},
			SharedSize:  o.SharedSize,
			Size:        o.Size,
		}
	}

	// Legacy response.
	if _, err := apiutil.SupportedVersion(r, "<1.52.0"); err == nil {
		legacy := make([]handlers.LegacyImageSummary, len(imgs_base))

		needVirtual := false
		if _, err := apiutil.SupportedVersion(r, "<1.44.0"); err == nil {
			needVirtual = true
		}

		for i := range imgs_base {
			legacy[i] = handlers.LegacyImageSummary{Summary: imgs_base[i]}
			if needVirtual {
				legacy[i].VirtualSize = df.Images[i].Size - df.Images[i].UniqueSize
			}
		}

		utils.WriteResponse(w, http.StatusOK, handlers.LegacyDiskUsage{ //nolint:staticcheck // LegacyDiskUsage is deprecated but kept for Docker API compat < v1.52
			LayersSize: df.ImagesSize,
			Images:     legacy,
			Containers: ctnrs,
			Volumes:    vols,
			BuildCache: []build.CacheRecord{},
		})
		return
	}

	// Non-legacy response.
	utils.WriteResponse(w, http.StatusOK, handlers.DiskUsage{DiskUsage: dockerSystem.DiskUsage{
		ImageUsage: &image.DiskUsage{
			TotalSize: df.ImagesSize,
			Items:     imgs_base,
		},
		ContainerUsage: &dockerContainer.DiskUsage{Items: ctnrs},
		VolumeUsage:    &volume.DiskUsage{Items: vols},
		BuildCacheUsage: &build.DiskUsage{
			Items: nil,
		},
	}})
}
