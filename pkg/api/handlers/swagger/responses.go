//go:build !remote

//nolint:unused // these types are used to wire generated swagger to API code
package swagger

import (
	"github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	dockerRegistry "github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/api/types/volume"
	"go.podman.io/common/libnetwork/types"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
	"go.podman.io/podman/v6/pkg/inspect"
)

// Image Tree
// swagger:response
type treeResponse struct {
	// in:body
	Body entities.ImageTreeReport
}

// Image History
// swagger:response
type history struct {
	// in:body
	Body handlers.HistoryResponse
}

// Image Inspect
// swagger:response
type imageInspect struct {
	// in:body
	Body handlers.ImageInspect
}

// Distribution Inspect
// swagger:response
type distributionInspectResponse struct {
	// in:body
	Body dockerRegistry.DistributionInspect
}

// Image Load
// swagger:response
type imagesLoadResponseLibpod struct {
	// in:body
	Body entities.ImageLoadReport
}

// Image Scp
// swagger:response
type imagesScpResponseLibpod struct {
	// in:body
	Body reports.ScpReport
}

// Image Import
// swagger:response
type imagesImportResponseLibpod struct {
	// in:body
	Body entities.ImageImportReport
}

// Image Pull. Errors may be detected later even if this returns HTTP status 200, and in that case, the error description will be in the `error` field.
// swagger:response
type imagesPullResponseLibpod struct {
	// in:body
	Body handlers.LibpodImagesPullReport
}

// Image Remove
// swagger:response
type imagesRemoveResponseLibpod struct {
	// in:body
	Body handlers.LibpodImagesRemoveReport
}

// PlayKube response
// swagger:response
type playKubeResponseLibpod struct {
	// in:body
	Body entities.PlayKubeReport
}

// Image Delete
// swagger:response
type imageDeleteResponse struct {
	// in:body
	Body []struct {
		Untagged []string `json:"untagged"`
		Deleted  string   `json:"deleted"`
	}
}

// Registry Search
// swagger:response
type registrySearchResponse struct {
	// in:body
	Body struct {
		// Index is the image index
		// example: quay.io
		Index string
		// Name is the canonical name of the image
		// example: docker.io/library/alpine"
		Name string
		// Description of the image.
		Description string
		// Stars is the number of stars of the image.
		Stars int
		// Official indicates if it's an official image.
		Official string
		// Automated indicates if the image was created by an automated build.
		Automated string
		// Tag is the image tag
		Tag string
	}
}

// Inspect Image
// swagger:response
type inspectImageResponseLibpod struct {
	// in:body
	Body inspect.ImageData
}

// Inspect container
// swagger:response
type containerInspectResponse struct {
	// in:body
	Body container.InspectResponse
}

// List processes in container
// swagger:response
type containerTopResponse struct {
	// in:body
	Body handlers.ContainerTopOKBody
}

// List processes in pod
// swagger:response
type podTopResponse struct {
	// in:body
	Body handlers.PodTopOKBody
}

// Pod Statistics
// swagger:response
type podStatsResponse struct {
	// in:body
	Body []entities.PodStatsReport
}

// Inspect container
// swagger:response
type containerInspectResponseLibpod struct {
	// in:body
	Body define.InspectContainerData
}

// List pods
// swagger:response
type podsListResponse struct {
	// in:body
	Body []entities.ListPodsReport
}

// Inspect pod
// swagger:response
type podInspectResponse struct {
	// in:body
	Body define.InspectPodData
}

// Volume details
// swagger:response
type volumeCreateResponse struct {
	// in:body
	Body entities.VolumeConfigResponse
}

// Healthcheck Results
// swagger:response
type healthCheck struct {
	// in:body
	Body define.HealthCheckResults
}

// Version
// swagger:response
type versionResponse struct {
	// in:body
	Body entities.ComponentVersion
}

// Check
// swagger:response
type systemCheckResponse struct {
	// in:body
	Body entities.SystemCheckReport
}

// Disk usage
// swagger:response
type systemDiskUsage struct {
	// in:body
	Body entities.SystemDfReport
}

// System Prune results
// swagger:response
type systemPruneResponse struct {
	// in:body
	Body entities.SystemPruneReport
}

// Auth response
// swagger:response
type systemAuthResponse struct {
	// in:body
	Body entities.AuthReport
}

// Exec Session Inspect
// swagger:response
type execSessionInspect struct {
	// in:body
	Body define.InspectExecSession
}

// Image summary for compat API
// swagger:response
type imageList struct {
	// in:body
	Body []dockerImage.Summary
}

// Image summary for libpod API
// swagger:response
type imageListLibpod struct {
	// in:body
	Body []entities.ImageSummary
}

// List Containers
// swagger:response
type containersList struct {
	// in:body
	Body []handlers.Container
}

// This response definition is used for both the create and inspect endpoints
// swagger:response
type volumeInspect struct {
	// in:body
	Body volume.Volume
}

// Volume prune
// swagger:response
type volumePruneResponse struct {
	// in:body
	Body volume.PruneReport
}

// Volume List
// swagger:response
type volumeList struct {
	// in:body
	Body volume.ListResponse
}

// Volume list
// swagger:response
type volumeListLibpod struct {
	// in:body
	Body []entities.VolumeConfigResponse
}

// Image Prune
// swagger:response
type imagesPruneLibpod struct {
	// in:body
	Body []reports.PruneReport
}

// Remove Containers
// swagger:response
type containerRemoveLibpod struct {
	// in: body
	Body []handlers.LibpodContainersRmReport
}

// Prune Containers
// swagger:response
type containersPrune struct {
	// in: body
	Body []handlers.ContainersPruneReport
}

// Prune Containers
// swagger:response
type containersPruneLibpod struct {
	// in: body
	Body []handlers.ContainersPruneReportLibpod
}

// Get stats for one or more containers
// swagger:response
type containerStats struct {
	// in:body
	Body define.ContainerStats
}

// Volume Prune
// swagger:response
type volumePruneLibpod struct {
	// in:body
	Body []reports.PruneReport
}

// Create container
// swagger:response
type containerCreateResponse struct {
	// in:body
	Body entities.ContainerCreateResponse
}

// Update container
// swagger:response
type containerUpdateResponse struct {
	// in:body
	Body struct {
		ID string
	}
}

// Wait container
// swagger:response
type containerWaitResponse struct {
	// in:body
	Body struct {
		// container exit code
		StatusCode int
		Error      struct {
			Message string
		}
	}
}

// Network inspect
// swagger:response
type networkInspectCompat struct {
	// in:body
	Body network.Inspect
}

// Network list
// swagger:response
type networkListCompat struct {
	// in:body
	Body []network.Summary
}

// List Containers
// swagger:response
type containersListLibpod struct {
	// in:body
	Body []entities.ListContainer
}

// Inspect Manifest
// swagger:response
type manifestInspect struct {
	// in:body
	Body manifest.Schema2List
}

// Kill Pod
// swagger:response
type podKillResponse struct {
	// in:body
	Body entities.PodKillReport
}

// Pause pod
// swagger:response
type podPauseResponse struct {
	// in:body
	Body entities.PodPauseReport
}

// Unpause pod
// swagger:response
type podUnpauseResponse struct {
	// in:body
	Body entities.PodUnpauseReport
}

// Stop pod
// swagger:response
type podStopResponse struct {
	// in:body
	Body entities.PodStopReport
}

// Restart pod
// swagger:response
type podRestartResponse struct {
	// in:body
	Body entities.PodRestartReport
}

// Start pod
// swagger:response
type podStartResponse struct {
	// in:body
	Body entities.PodStartReport
}

// Prune pod
// swagger:response
type podPruneResponse struct {
	// in:body
	Body entities.PodPruneReport
}

// Rm pod
// swagger:response
type podRmResponse struct {
	// in:body
	Body entities.PodRmReport
}

// Info
// swagger:response
type infoResponse struct {
	// in:body
	Body define.Info
}

// Network Delete
// swagger:response
type networkRmResponse struct {
	// in:body
	Body []entities.NetworkRmReport
}

// Network inspect
// swagger:response
type networkInspectResponse struct {
	// in:body
	Body entities.NetworkInspectReport
}

// Network list
// swagger:response
type networkListLibpod struct {
	// in:body
	Body []types.Network
}

// Network create
// swagger:model
type networkCreateLibpod struct {
	// in:body
	types.Network
}

// Network create
// swagger:response
type networkCreateResponse struct {
	// in:body
	Body types.Network
}

// Network prune
// swagger:response
type networkPruneResponse struct {
	// in:body
	Body []entities.NetworkPruneReport
}

// Inspect Artifact
// swagger:response
type inspectArtifactResponse struct {
	// in:body
	Body entities.ArtifactInspectReport
}

// Artifact list
// swagger:response
type artifactListResponse struct {
	// in:body
	Body []entities.ArtifactListReport
}

// Artifact Pull
// swagger:response
type artifactPullResponse struct {
	// in:body
	Body entities.ArtifactPullReport
}

// Artifact Remove
// swagger:response
type artifactRemoveResponse struct {
	// in:body
	Body entities.ArtifactRemoveReport
}

// Artifact Add
// swagger:response
type artifactAddResponse struct {
	// in:body
	Body entities.ArtifactAddReport
}

// Artifact Push
// swagger:response
type artifactPushResponse struct {
	// in:body
	Body entities.ArtifactPushReport
}

// Quadlet list
// swagger:response
type quadletListResponse struct {
	// in:body
	Body []entities.ListQuadlet
}

// Quadlet file
// swagger:response
type quadletFileResponse struct {
	// in:body
	Body string
}

// Quadlet remove
// swagger:response
type quadletRemoveResponse struct {
	// in:body
	Body entities.QuadletRemoveReport
}
