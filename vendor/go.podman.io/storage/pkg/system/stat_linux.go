package system

import (
	"syscall"

	"go.podman.io/storage/internal/stat"
)

// FromStatT converts a syscall.Stat_t type to a system.Stat_t type
// This is exposed on Linux as pkg/archive/changes uses it.
func FromStatT(s *syscall.Stat_t) (*StatT, error) {
	return stat.FromStatT(s), nil
}
