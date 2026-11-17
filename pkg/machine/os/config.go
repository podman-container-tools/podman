//go:build amd64 || arm64

package os

import (
	"context"

	"github.com/blang/semver/v4"
	"github.com/opencontainers/go-digest"
)

// Manager is the interface for operations on a Podman machine's OS
type Manager interface {
	// Apply machine OS changes from an OCI image.
	Apply(image string, opts ApplyOptions) error
	// Upgrade the machine OS
	Upgrade(ctx context.Context, opts UpgradeOptions) error
}

// ApplyOptions are the options for applying an image into a Podman machine VM
type ApplyOptions struct {
	Image string
}

type UpgradeOptions struct {
	MachineVersion semver.Version
	DryRun         bool
	Format         string
	ClientVersion  semver.Version
}

type Host struct {
	Version semver.Version `json:"version"`
}

type Machine struct {
	CurrentHash            digest.Digest  `json:"current_hash"`
	NewHash                digest.Digest  `json:"new_hash"`
	Version                semver.Version `json:"version"`
	InBandUpgradeAvailable bool           `json:"inband_update_available"`
}

// UpgradeOutput is an output struct and is only exported so json tags work
// correctly
type UpgradeOutput struct {
	Host    Host    `json:"host"`
	Machine Machine `json:"machine"`
}
