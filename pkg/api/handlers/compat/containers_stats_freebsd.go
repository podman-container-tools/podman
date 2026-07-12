//go:build !remote

package compat

import (
	"time"

	"github.com/moby/moby/api/types/container"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
)

func getPreCPUStats(stats *define.ContainerStats) CPUStats {
	return CPUStats{
		CPUUsage: container.CPUUsage{
			TotalUsage: stats.CPUNano,
		},
		CPU:            stats.CPU,
		OnlineCPUs:     0,
		ThrottlingData: container.ThrottlingData{},
	}
}

func statsContainerJSON(_ *libpod.Container, stats *define.ContainerStats, preCPUStats CPUStats, onlineCPUs int) (StatsJSON, error) {
	return StatsJSON{
		Stats: Stats{
			Read: time.Now(),
			CPUStats: CPUStats{
				CPUUsage: container.CPUUsage{
					TotalUsage: stats.CPUNano,
				},
				CPU:            stats.CPU,
				OnlineCPUs:     uint32(onlineCPUs),
				ThrottlingData: container.ThrottlingData{},
			},
			PreCPUStats: preCPUStats,
			MemoryStats: container.MemoryStats{},
		},
		Name: stats.Name,
		ID:   stats.ContainerID,
	}, nil
}
