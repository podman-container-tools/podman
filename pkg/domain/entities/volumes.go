package entities

import (
	"io"
	"net/url"

	"go.podman.io/podman/v6/pkg/domain/entities/types"
)

// VolumeCreateOptions provides details for creating volumes
type VolumeCreateOptions = types.VolumeCreateOptions

type VolumeConfigResponse = types.VolumeConfigResponse

type VolumeRmOptions struct {
	All     bool
	Force   bool
	Ignore  bool
	Timeout *uint
}

type VolumeRmReport = types.VolumeRmReport

type VolumeInspectReport = types.VolumeInspectReport

// VolumePruneOptions describes the options needed to prune volumes.
// Behavior is determined only by filters (Docker API 1.42+):
// - when filter "all" is not set or not truthy, only anonymous unused volumes are pruned (default);
// - when filter "all" is true, all unused volumes are pruned.
type VolumePruneOptions struct {
	Filters url.Values `json:"filters" schema:"filters"`
	DryRun  bool       `json:"dry_run" schema:"dryrun"`
}

type VolumeListOptions struct {
	Filter map[string][]string
}

type VolumeListReport = types.VolumeListReport

// VolumeReloadReport describes the response from reload volume plugins
type VolumeReloadReport = types.VolumeReloadReport

// VolumeMountReport describes the response from volume mount
type VolumeMountReport = types.VolumeMountReport

// VolumeUnmountReport describes the response from umounting a volume
type VolumeUnmountReport = types.VolumeUnmountReport

// VolumeExportOptions describes the options required to export a volume.
type VolumeExportOptions struct {
	Output io.Writer
}

// VolumeRenameOptions describes the options for renaming a volume.
type VolumeRenameOptions struct {
	NewName string
}

// VolumeImportOptions describes the options required to import a volume
type VolumeImportOptions struct {
	// Input will be closed upon being fully consumed
	Input io.Reader
}
