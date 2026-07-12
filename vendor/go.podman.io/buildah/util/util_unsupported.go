//go:build !linux

package util //nolint:revive,nolintlint

// IsCgroup2UnifiedMode returns whether we are running in cgroup 2 cgroup2 mode.
func IsCgroup2UnifiedMode() (bool, error) {
	return false, nil
}
