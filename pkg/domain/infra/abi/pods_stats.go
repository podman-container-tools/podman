//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/docker/go-units"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// PodStats implements printing stats about pods.
func (ic *ContainerEngine) PodStats(_ context.Context, namesOrIds []string, options entities.PodStatsOptions) ([]*entities.PodStatsReport, error) {
	// Get the (running) pods and convert them to the entities format.
	pods, err := getPodsByContext(options.All, options.Latest, namesOrIds, ic.Libpod)
	if err != nil {
		return nil, fmt.Errorf("unable to get list of pods: %w", err)
	}
	return ic.podsToStatsReport(pods)
}

// podsToStatsReport converts a slice of pods into a corresponding slice of stats reports.
func (ic *ContainerEngine) podsToStatsReport(pods []*libpod.Pod) ([]*entities.PodStatsReport, error) {
	reports := []*entities.PodStatsReport{}
	for i := range pods { // Access by index to prevent potential loop-variable leaks.
		podStats, err := pods[i].GetPodStats()
		if err != nil {
			// pod was removed, skip it
			if errors.Is(err, define.ErrNoSuchPod) {
				continue
			}
			return nil, err
		}
		podID := pods[i].ID()[:12]
		for j := range podStats {
			var podNetInput uint64
			var podNetOutput uint64
			for _, stats := range podStats[j].Network {
				podNetInput += stats.RxBytes
				podNetOutput += stats.TxBytes
			}

			r := entities.PodStatsReport{
				CPU:           floatToPercentString(podStats[j].CPU),
				MemUsage:      combineHumanValues(podStats[j].MemUsage, podStats[j].MemLimit),
				MemUsageBytes: combineBytesValues(podStats[j].MemUsage, podStats[j].MemLimit),
				Mem:           floatToPercentString(podStats[j].MemPerc),
				NetIO:         combineHumanValues(podNetInput, podNetOutput),
				BlockIO:       combineHumanValues(podStats[j].BlockInput, podStats[j].BlockOutput),
				PIDS:          pidsToString(podStats[j].PIDs),
				CID:           podStats[j].ContainerID[:12],
				Name:          podStats[j].Name,
				Pod:           podID,
			}
			reports = append(reports, &r)
		}
	}

	return reports, nil
}

func combineHumanValues(a, b uint64) string {
	if a == 0 && b == 0 {
		return "-- / --"
	}
	return fmt.Sprintf("%s / %s", units.HumanSize(float64(a)), units.HumanSize(float64(b)))
}

func combineBytesValues(a, b uint64) string {
	if a == 0 && b == 0 {
		return "-- / --"
	}
	return fmt.Sprintf("%s / %s", units.BytesSize(float64(a)), units.BytesSize(float64(b)))
}

func floatToPercentString(f float64) string {
	return fmt.Sprintf("%.2f%%", f)
}

func pidsToString(pid uint64) string {
	if pid == 0 {
		// If things go bazinga, return a safe value
		return "--"
	}
	return strconv.FormatUint(pid, 10)
}
