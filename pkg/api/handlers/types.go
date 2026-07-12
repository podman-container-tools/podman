package handlers

import (
	build "github.com/moby/moby/api/types/build"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/jsonstream"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/registry"
	swarm "github.com/moby/moby/api/types/swarm"
	dockerSystem "github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	"github.com/opencontainers/runtime-spec/specs-go"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

type AuthConfig struct {
	registry.AuthConfig
}

type ImageInspect struct {
	dockerImage.InspectResponse
	// When you embed a struct, the fields of the embedded struct are "promoted" to the outer struct.
	// If a field in the outer struct has the same name as a field in the embedded struct,
	// the outer struct's field will shadow or override the embedded one allowing for a clean way to
	// hide fields from the swagger spec that still exist in the libraries struct.
	//
	// Deprecated: since API v1.44, will no longer be included in API v1.45.
	// https://docs.docker.com/reference/api/engine/version-history/#v144-api-changes
	Container string `json:"Container,omitempty"`
	// Deprecated: since API v1.44, will no longer be included in API v1.45.
	// https://docs.docker.com/reference/api/engine/version-history/#v144-api-changes
	ContainerConfig *dockerContainer.Config `json:"ContainerConfig,omitempty"`
	VirtualSize     int64                   `json:"VirtualSize,omitempty"`
	Parent          string                  `json:"Parent"`
	DockerVersion   string                  `json:"DockerVersion"`
	Author          string                  `json:"Author"`
}

type ContainerConfig struct {
	dockerContainer.Config
	MacAddress string `json:"MacAddress,omitempty"`
}

type LibpodImagesPullReport struct {
	entities.ImagePullReport
}

// LibpodImagesRemoveReport is the return type for image removal via the rest
// api.
type LibpodImagesRemoveReport struct {
	entities.ImageRemoveReport
	// Image removal requires is to return data and an error.
	Errors []string
}

// LibpodImagesResolveReport includes a list of fully-qualified image references.
type LibpodImagesResolveReport struct {
	// Fully-qualified image references.
	Names []string
}

type ContainersPruneReport struct {
	dockerContainer.PruneReport
}

type ContainersPruneReportLibpod struct {
	ID             string `json:"Id"`
	SpaceReclaimed int64  `json:"Size"`
	// Error which occurred during prune operation (if any).
	// This field is optional and may be omitted if no error occurred.
	//
	// Extensions:
	// x-omitempty: true
	// x-nullable: true
	PruneError string `json:"Err,omitempty"`
}

type LibpodContainersRmReport struct {
	ID string `json:"Id"`
	// Error which occurred during Rm operation (if any).
	// This field is optional and may be omitted if no error occurred.
	//
	// Extensions:
	// x-omitempty: true
	// x-nullable: true
	RmError string `json:"Err,omitempty"`
}

// UpdateEntities used to wrap the oci resource spec in a swagger model
// swagger:model
type UpdateEntities struct {
	specs.LinuxResources
	define.UpdateHealthCheckConfig
	define.UpdateContainerDevicesLimits
	Env      []string
	UnsetEnv []string
	Rlimits  []specs.POSIXRlimit `json:"r_limits,omitempty"`
}

type Info struct {
	dockerSystem.Info
	BuildahVersion     string
	CPURealtimePeriod  bool
	CPURealtimeRuntime bool
	CgroupVersion      string
	Rootless           bool
	SwapFree           int64
	SwapTotal          int64
	Uptime             string
}

type Container struct {
	dockerContainer.Summary
}

// swagger:model LegacyImageSummary
type LegacyImageSummary struct {
	dockerImage.Summary
	VirtualSize int64 `json:"VirtualSize,omitempty"`
}

// Deprecated: kept to maintain backwards compatibility with API < v1.52
type LegacyDiskUsage struct {
	// Deprecated: kept to maintain backwards compatibility with API < v1.52, use [ImagesDiskUsage.TotalSize] instead.
	LayersSize int64 `json:"LayersSize"`

	// Deprecated: kept to maintain backwards compatibility with API < v1.52, use [ImagesDiskUsage.Items] instead.
	Images []LegacyImageSummary `json:"Images,omitzero"`

	// Deprecated: kept to maintain backwards compatibility with API < v1.52, use [ContainersDiskUsage.Items] instead.
	Containers []dockerContainer.Summary `json:"Containers,omitzero"`

	// Deprecated: kept to maintain backwards compatibility with API < v1.52, use [VolumesDiskUsage.Items] instead.
	Volumes []volume.Volume `json:"Volumes,omitzero"`

	// Deprecated: kept to maintain backwards compatibility with API < v1.52, use [BuildCacheDiskUsage.Items] instead.
	BuildCache []build.CacheRecord `json:"BuildCache,omitzero"`
}

// Deprecated: kept to maintain backwards compatibility with API < v1.52
type LegacyJSONMessage struct {
	jsonstream.Message
	// ErrorMessage contains errors encountered during the operation.
	//
	// Deprecated: this field is deprecated since docker v0.6.0 / API v1.4. Use [Error.Message] instead.
	ErrorMessage    string `json:"error,omitempty"` // deprecated
	ProgressMessage string `json:"progress,omitempty"`
}

// Deprecated: kept to maintain backwards compatibility with API < v1.52
type LegacyAddress struct {
	Addr      string
	PrefixLen int
}

// Deprecated: kept to maintain backwards compatibility with API < v1.52
type LegacyNetworkSettings struct {
	dockerContainer.NetworkSettings
	SecondaryIPAddresses   []LegacyAddress `json:"SecondaryIPAddresses,omitempty"`
	SecondaryIPv6Addresses []LegacyAddress `json:"SecondaryIPv6Addresses,omitempty"`
}

// Deprecated: kept to maintain backwards compatibility with API < v1.52
type LegacyImageInspect struct {
	dockerContainer.InspectResponse
	NetworkSettings *LegacyNetworkSettings
	Config          *ContainerConfig
}

type DiskUsage struct {
	dockerSystem.DiskUsage
}

type VolumesPruneReport struct {
	volume.PruneReport
}

type ImagesPruneReport struct {
	dockerImage.PruneReport
}

type BuildCachePruneReport struct {
	build.CachePruneReport
}

type NetworkPruneReport struct {
	network.PruneReport
}

type ConfigCreateResponse struct {
	swarm.ConfigCreateResponse
}

type BuildResult struct {
	build.Result
}

type ContainerWaitOKBody struct {
	StatusCode int
	Error      *struct {
		Message string
	}
}

// CreateContainerConfig used when compatible endpoint creates a container
// swagger:model
type CreateContainerConfig struct {
	Name                   string                     // container name
	dockerContainer.Config                            // desired container configuration
	HostConfig             dockerContainer.HostConfig // host dependent configuration for container
	NetworkingConfig       network.NetworkingConfig   // network configuration for container
	EnvMerge               []string                   // preprocess env variables from image before injecting into containers
	UnsetEnv               []string                   // unset specified default environment variables
	UnsetEnvAll            bool                       // unset all default environment variables
	MacAddress             string
}

type ContainerTopOKBody struct {
	dockerContainer.TopResponse
}

type PodTopOKBody struct {
	dockerContainer.TopResponse
}

// HistoryResponse provides details on image layers
type HistoryResponse struct {
	ID        string `json:"Id"`
	Created   int64
	CreatedBy string
	Tags      []string
	Size      int64
	Comment   string
}

type ExecCreateConfig struct {
	dockerContainer.ExecCreateRequest
}

type ExecStartConfig struct {
	Detach bool   `json:"Detach"`
	Tty    bool   `json:"Tty"`
	Height uint16 `json:"h"`
	Width  uint16 `json:"w"`
}

type ExecRemoveConfig struct {
	Force bool `json:"Force"`
}
