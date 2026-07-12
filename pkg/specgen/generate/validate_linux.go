//go:build !remote

package generate

import (
	"os"
	"path/filepath"

	"go.podman.io/common/pkg/cgroups"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/storage/pkg/fileutils"
)

// Verify resource limits are sanely set, removing any limits that are not
// possible with the current cgroups config.
func verifyContainerResources(s *specgen.SpecGenerator) ([]string, error) {
	warnings := []string{}

	if s.ResourceLimits == nil {
		return warnings, nil
	}

	// Memory checks
	if s.ResourceLimits.Memory != nil && s.ResourceLimits.Memory.Swap != nil {
		own, err := cgroups.GetOwnCgroup()
		if err != nil {
			return warnings, err
		}

		if own == "/" {
			// If running under the root cgroup try to create or reuse a "probe" cgroup to read memory values
			own = "podman_probe"
			_ = os.MkdirAll(filepath.Join("/sys/fs/cgroup", own), 0o755)
			_ = os.WriteFile("/sys/fs/cgroup/cgroup.subtree_control", []byte("+memory"), 0o644)
		}

		memoryMax := filepath.Join("/sys/fs/cgroup", own, "memory.max")
		memorySwapMax := filepath.Join("/sys/fs/cgroup", own, "memory.swap.max")
		errMemoryMax := fileutils.Exists(memoryMax)
		errMemorySwapMax := fileutils.Exists(memorySwapMax)
		// Differently than cgroup v1, the memory.*max files are not present in the
		// root directory, so we cannot query directly that, so as best effort use
		// the current cgroup.
		// Check whether memory.max exists in the current cgroup and memory.swap.max
		//  does not.  In this case we can be sure memory swap is not enabled.
		// If both files don't exist, the memory controller might not be enabled
		// for the current cgroup.
		if errMemoryMax == nil && errMemorySwapMax != nil {
			warnings = append(warnings, "Your kernel does not support swap limit capabilities or the cgroup is not mounted. Memory limited without swap.")
			s.ResourceLimits.Memory.Swap = nil
		}
	}

	// CPU checks
	if s.ResourceLimits.CPU != nil {
		cpu := s.ResourceLimits.CPU
		if cpu.RealtimePeriod != nil {
			warnings = append(warnings, "Realtime period not supported on cgroups V2 systems")
			cpu.RealtimePeriod = nil
		}
		if cpu.RealtimeRuntime != nil {
			warnings = append(warnings, "Realtime runtime not supported on cgroups V2 systems")
			cpu.RealtimeRuntime = nil
		}
	}
	return warnings, nil
}
