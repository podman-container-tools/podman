package libpod

import (
	"fmt"
	"maps"
	"strings"

	spec "github.com/opencontainers/runtime-spec/specs-go"
)

// mergeBlockIOUpdate preserves empty lists that JSON's omitempty drops.
func mergeBlockIOUpdate(saved, update *spec.LinuxBlockIO) {
	if update.WeightDevice != nil {
		saved.WeightDevice = update.WeightDevice
	}
	if update.ThrottleReadBpsDevice != nil {
		saved.ThrottleReadBpsDevice = update.ThrottleReadBpsDevice
	}
	if update.ThrottleWriteBpsDevice != nil {
		saved.ThrottleWriteBpsDevice = update.ThrottleWriteBpsDevice
	}
	if update.ThrottleReadIOPSDevice != nil {
		saved.ThrottleReadIOPSDevice = update.ThrottleReadIOPSDevice
	}
	if update.ThrottleWriteIOPSDevice != nil {
		saved.ThrottleWriteIOPSDevice = update.ThrottleWriteIOPSDevice
	}
}

// blockIORuntimeUpdate adds cgroup v2 reset directives for rules removed from
// BlockIO. They are only sent to the runtime, never persisted in the OCI spec.
func blockIORuntimeUpdate(previous, desired *spec.LinuxResources) *spec.LinuxResources {
	if previous == nil || previous.BlockIO == nil {
		return desired
	}
	old := previous.BlockIO
	current := &spec.LinuxBlockIO{}
	if desired != nil && desired.BlockIO != nil {
		current = desired.BlockIO
	}
	var maxResets, weightResets []string
	weights := make(map[spec.LinuxBlockIODevice]struct{}, len(current.WeightDevice))
	for _, entry := range current.WeightDevice {
		weights[entry.LinuxBlockIODevice] = struct{}{}
	}
	for _, entry := range old.WeightDevice {
		if _, kept := weights[entry.LinuxBlockIODevice]; !kept && entry.Weight != nil && *entry.Weight != 0 {
			weightResets = append(weightResets, fmt.Sprintf("%d:%d default", entry.Major, entry.Minor))
		}
	}
	for _, list := range []struct {
		key     string
		old     []spec.LinuxThrottleDevice
		current []spec.LinuxThrottleDevice
	}{
		{"rbps", old.ThrottleReadBpsDevice, current.ThrottleReadBpsDevice},
		{"wbps", old.ThrottleWriteBpsDevice, current.ThrottleWriteBpsDevice},
		{"riops", old.ThrottleReadIOPSDevice, current.ThrottleReadIOPSDevice},
		{"wiops", old.ThrottleWriteIOPSDevice, current.ThrottleWriteIOPSDevice},
	} {
		kept := make(map[spec.LinuxBlockIODevice]struct{}, len(list.current))
		for _, device := range list.current {
			kept[device.LinuxBlockIODevice] = struct{}{}
		}
		for _, device := range list.old {
			if _, ok := kept[device.LinuxBlockIODevice]; !ok && device.Rate != 0 {
				maxResets = append(maxResets, fmt.Sprintf("%d:%d %s=max", device.Major, device.Minor, list.key))
			}
		}
	}
	if len(maxResets)+len(weightResets) == 0 {
		return desired
	}
	var live spec.LinuxResources
	if desired != nil {
		live = *desired
		live.Unified = maps.Clone(desired.Unified)
	}
	if live.Unified == nil {
		live.Unified = make(map[string]string, 2)
	}
	for file, resets := range map[string][]string{"io.max": maxResets, "io.bfq.weight": weightResets} {
		if len(resets) > 0 {
			if original := live.Unified[file]; original != "" {
				resets = append(resets, original)
			}
			live.Unified[file] = strings.Join(resets, "\n")
		}
	}
	return &live
}
