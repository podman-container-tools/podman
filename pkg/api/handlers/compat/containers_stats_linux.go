//go:build !remote

package compat

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moby/moby/api/types/container"
	runccgroups "github.com/opencontainers/cgroups"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/cgroups"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/storage/pkg/system"
)

// getSystemCPUUsage returns total CPU time (including idle) in nanoseconds,
// matching Docker's system_cpu_usage semantics.
func getSystemCPUUsage() (uint64, error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, fmt.Errorf("unable to open /proc/stat: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return 0, fmt.Errorf("unable to read /proc/stat")
	}

	parts := strings.Fields(scanner.Text())
	if len(parts) < 2 || parts[0] != "cpu" {
		return 0, fmt.Errorf("unexpected /proc/stat format")
	}

	// Docker limits to the first 7 CPU fields (user, nice, system, idle,
	// iowait, irq, softirq). Fields 8+ (guest, guest_nice) overlap with
	// user/nice and would double-count.
	// https://github.com/moby/moby/blob/v28.0.0/daemon/stats_unix.go#L337-L347
	end := min(len(parts), 8)
	var totalTicks uint64
	for _, s := range parts[1:end] {
		v, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("unable to parse /proc/stat CPU field %q: %w", s, err)
		}
		totalTicks += v
	}

	// ClkTck is the number of clock ticks per second, which is virtually
	// always 100 on Linux. Hard-coded to avoid a cgo dependency, matching
	// Docker's approach.
	// https://github.com/moby/moby/blob/v28.0.0/daemon/stats_unix.go#L296-L301
	const (
		clkTck      = 100
		nsPerSecond = 1_000_000_000
	)
	return totalTicks * (nsPerSecond / clkTck), nil
}

func getPreCPUStats(stats *define.ContainerStats) (CPUStats, error) {
	systemUsage, err := getSystemCPUUsage()
	if err != nil {
		return CPUStats{}, fmt.Errorf("unable to get pre-CPU system usage: %w", err)
	}
	return CPUStats{
		CPUUsage: container.CPUUsage{
			TotalUsage:        stats.CPUNano,
			UsageInKernelmode: stats.CPUSystemNano,
			UsageInUsermode:   stats.CPUNano - stats.CPUSystemNano,
		},
		CPU:            stats.CPU,
		SystemUsage:    systemUsage,
		OnlineCPUs:     0,
		ThrottlingData: container.ThrottlingData{},
	}, nil
}

func statsContainerJSON(ctnr *libpod.Container, stats *define.ContainerStats, preCPUStats CPUStats, onlineCPUs int) (StatsJSON, error) {
	inspect, err := ctnr.Inspect(false)
	if err != nil {
		return StatsJSON{}, fmt.Errorf("unable to inspect container: %w", err)
	}
	// Second cgroup read for memory (MaxUsage), blkio, and PIDs details
	// not available in define.ContainerStats. CPU values come from stats
	// (populated by GetContainerStats above) to avoid a timing gap.
	cgroupPath, err := ctnr.CgroupPath()
	if err != nil {
		return StatsJSON{}, fmt.Errorf("unable to get cgroup path of container: %w", err)
	}
	cgroup, err := cgroups.Load(cgroupPath)
	if err != nil {
		return StatsJSON{}, fmt.Errorf("unable to load cgroup: %w", err)
	}
	cgroupStat, err := cgroup.Stat()
	if err != nil {
		return StatsJSON{}, fmt.Errorf("unable to get cgroup stats: %w", err)
	}

	net := make(map[string]container.NetworkStats)
	for netName, netStats := range stats.Network {
		net[netName] = container.NetworkStats{
			RxBytes:    netStats.RxBytes,
			RxPackets:  netStats.RxPackets,
			RxErrors:   netStats.RxErrors,
			RxDropped:  netStats.RxDropped,
			TxBytes:    netStats.TxBytes,
			TxPackets:  netStats.TxPackets,
			TxErrors:   netStats.TxErrors,
			TxDropped:  netStats.TxDropped,
			EndpointID: inspect.NetworkSettings.EndpointID,
			InstanceID: "",
		}
	}

	resources := ctnr.LinuxResources()
	memoryLimit := cgroupStat.MemoryStats.Usage.Limit
	if resources != nil && resources.Memory != nil && resources.Memory.Limit != nil && *resources.Memory.Limit > 0 {
		memoryLimit = uint64(*resources.Memory.Limit)
	}

	memInfo, err := system.ReadMemInfo()
	if err != nil {
		return StatsJSON{}, fmt.Errorf("unable to get memory info: %w", err)
	}
	// cap the memory limit to the available memory.
	if memInfo.MemTotal > 0 && memoryLimit > uint64(memInfo.MemTotal) {
		memoryLimit = uint64(memInfo.MemTotal)
	}

	systemUsage, err := getSystemCPUUsage()
	if err != nil {
		return StatsJSON{}, fmt.Errorf("unable to get system CPU usage: %w", err)
	}

	return StatsJSON{
		Stats: Stats{
			Read: time.Now(),
			PidsStats: container.PidsStats{
				Current: cgroupStat.PidsStats.Current,
				Limit:   0,
			},
			BlkioStats: container.BlkioStats{
				IoServiceBytesRecursive: toBlkioStatEntry(cgroupStat.BlkioStats.IoServiceBytesRecursive),
				IoServicedRecursive:     nil,
				IoQueuedRecursive:       nil,
				IoServiceTimeRecursive:  nil,
				IoWaitTimeRecursive:     nil,
				IoMergedRecursive:       nil,
				IoTimeRecursive:         nil,
				SectorsRecursive:        nil,
			},
			CPUStats: CPUStats{
				CPUUsage: container.CPUUsage{
					TotalUsage:        stats.CPUNano,
					PercpuUsage:       cgroupStat.CpuStats.CpuUsage.PercpuUsage,
					UsageInKernelmode: stats.CPUSystemNano,
					UsageInUsermode:   stats.CPUNano - stats.CPUSystemNano,
				},
				CPU:         stats.CPU,
				SystemUsage: systemUsage,
				OnlineCPUs:  uint32(onlineCPUs),
				ThrottlingData: container.ThrottlingData{
					Periods:          0,
					ThrottledPeriods: 0,
					ThrottledTime:    0,
				},
			},
			PreCPUStats: preCPUStats,
			MemoryStats: container.MemoryStats{
				Usage:             cgroupStat.MemoryStats.Usage.Usage,
				MaxUsage:          cgroupStat.MemoryStats.Usage.MaxUsage,
				Stats:             nil,
				Failcnt:           0,
				Limit:             memoryLimit,
				Commit:            0,
				CommitPeak:        0,
				PrivateWorkingSet: 0,
			},
		},
		Name:     stats.Name,
		ID:       stats.ContainerID,
		Networks: net,
	}, nil
}

func toBlkioStatEntry(entries []runccgroups.BlkioStatEntry) []container.BlkioStatEntry {
	results := make([]container.BlkioStatEntry, len(entries))
	for i, e := range entries {
		bits, err := json.Marshal(e)
		if err != nil {
			logrus.Errorf("Unable to marshal blkio stats: %q", err)
		}
		if err := json.Unmarshal(bits, &results[i]); err != nil {
			logrus.Errorf("Unable to unmarshal blkio stats: %q", err)
		}
	}
	return results
}
