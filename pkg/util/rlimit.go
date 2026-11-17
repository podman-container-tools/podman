//go:build !remote

package util

import (
	"fmt"
	"strings"

	"github.com/docker/go-units"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/rootless"
	"golang.org/x/sys/unix"
)

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

	rlimitName := strings.TrimPrefix(strings.ToLower(u.Type), "rlimit_")
	ul, err := units.ParseUlimit(fmt.Sprintf("%s=%d:%d", rlimitName, int64(u.Soft), int64(u.Hard)))
	if err != nil {
		logrus.Warnf("Failed to check %s ulimit %q", u.Type, err)
		return u
	}
	rl, err := ul.GetRlimit()
	if err != nil {
		logrus.Warnf("Failed to check %s ulimit %q", u.Type, err)
		return u
	}

	var rlimit unix.Rlimit
	if err := unix.Getrlimit(rl.Type, &rlimit); err != nil {
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
