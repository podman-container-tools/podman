//go:build !remote

package util

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/rlimit"
	"go.podman.io/podman/v6/pkg/rootless"
	"golang.org/x/sys/unix"
)

// GetHostRlimits snapshots all supported host limits.
func GetHostRlimits() ([]specs.POSIXRlimit, error) {
	names := slices.Sorted(maps.Keys(hostRlimitResources))
	limits := make([]specs.POSIXRlimit, 0, len(names))
	for _, name := range names {
		var limit unix.Rlimit
		if err := unix.Getrlimit(hostRlimitResources[name], &limit); err != nil {
			return nil, fmt.Errorf("getting host %s limit: %w", name, err)
		}
		u := specs.POSIXRlimit{Type: strings.TrimPrefix(strings.ToLower(name), "rlimit_"), Soft: rlimitMaxToUint64(limit.Cur), Hard: rlimitMaxToUint64(limit.Max)}
		if name == "RLIMIT_NOFILE" {
			var err error
			u.Soft, u.Hard, err = rlimit.GetOriginalNofile()
			if err != nil {
				return nil, fmt.Errorf("getting original host nofile limit: %w", err)
			}
		}
		limits = append(limits, u)
	}
	return limits, nil
}

// FormatRlimits formats the rlimits to the runtime spec format
func FormatRlimits(rlimits []specs.POSIXRlimit) []specs.POSIXRlimit {
	formatted := make([]specs.POSIXRlimit, len(rlimits))
	for i, rlimit := range rlimits {
		typ := strings.ToUpper(rlimit.Type)
		if !strings.HasPrefix(typ, "RLIMIT_") {
			typ = "RLIMIT_" + typ
		}
		rlimit.Type = typ
		formatted[i] = ClampRlimitToHost(rlimit)
	}
	return formatted
}

// ClampRlimitToHost translates Hard or soft limits of -1 to the current
// processes Max limit
func ClampRlimitToHost(u specs.POSIXRlimit) specs.POSIXRlimit {
	if !rootless.IsRootless() || (int64(u.Hard) != -1 && int64(u.Soft) != -1) {
		return u
	}

	rlimitName := strings.TrimPrefix(strings.ToUpper(u.Type), "RLIMIT_")
	resource, ok := hostRlimitResources["RLIMIT_"+rlimitName]
	if !ok {
		logrus.Warnf("Failed to check %s ulimit: unknown resource", u.Type)
		return u
	}

	var rlimit unix.Rlimit
	if err := unix.Getrlimit(resource, &rlimit); err != nil {
		logrus.Warnf("Failed to return %s ulimit %q", u.Type, err)
		return u
	}
	maxLimit := rlimitMaxToUint64(rlimit.Max)
	if int64(u.Hard) == -1 {
		u.Hard = maxLimit
	}
	if int64(u.Soft) == -1 {
		u.Soft = maxLimit
	}
	return u
}

// rlimitMaxToUint64 converts unix.Rlimit.Max to uint64.
// On Linux, Max is uint64; on Darwin/BSD, Max is int64.
func rlimitMaxToUint64(max any) uint64 {
	switch v := max.(type) {
	case uint64:
		return v
	case int64:
		return uint64(v)
	default:
		panic(fmt.Sprintf("unexpected type for rlimit.Max: %T (expected uint64 or int64)", v))
	}
}
