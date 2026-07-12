//go:build !remote

package compat

import (
	"time"

	"github.com/moby/moby/api/types/container"
)

// CPUStats aggregates and wraps all CPU related info of container
type CPUStats struct {
	// CPU Usage. Linux and Windows.
	CPUUsage container.CPUUsage `json:"cpu_usage"`

	// System Usage. Linux only.
	SystemUsage uint64 `json:"system_cpu_usage,omitempty"`

	// Online CPUs. Linux only.
	OnlineCPUs uint32 `json:"online_cpus,omitempty"`

	// Usage of CPU in %. Linux only.
	CPU float64 `json:"cpu"`

	// Throttling Data. Linux only.
	ThrottlingData container.ThrottlingData `json:"throttling_data"`
}

// Stats is Ultimate struct aggregating all types of stats of one container
type Stats struct {
	// Common stats
	Read    time.Time `json:"read"`
	PreRead time.Time `json:"preread"`

	// Linux specific stats, not populated on Windows.
	PidsStats  container.PidsStats  `json:"pids_stats"`
	BlkioStats container.BlkioStats `json:"blkio_stats"`

	// Windows specific stats, not populated on Linux.
	NumProcs     uint32                 `json:"num_procs"`
	StorageStats container.StorageStats `json:"storage_stats"`

	// Shared stats
	CPUStats    CPUStats              `json:"cpu_stats"`
	PreCPUStats CPUStats              `json:"precpu_stats"` // "Pre"="Previous"
	MemoryStats container.MemoryStats `json:"memory_stats"`
}

type StatsJSON struct {
	Stats

	Name string `json:"name,omitempty"`
	ID   string `json:"Id,omitempty"`

	// Networks request version >=1.21
	Networks map[string]container.NetworkStats `json:"networks,omitempty"`
}

// DockerStatsJSON is the same as StatsJSON except for the lowercase
// "id" in the JSON tag. This is needed for docker compat but we should
// not change the libpod API output for backwards compat reasons.
type DockerStatsJSON struct {
	Stats

	Name string `json:"name,omitempty"`
	ID   string `json:"id,omitempty"`
	// Networks request version >=1.21
	Networks map[string]container.NetworkStats `json:"networks,omitempty"`
}
