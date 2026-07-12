//go:build unix && !linux

package system

import "golang.org/x/sys/unix"

// Syncfs synchronizes the filesystem containing the given path.
// On non-Linux Unix platforms, this falls back to sync(2) which
// syncs all filesystems.
func Syncfs(path string) error {
	return unix.Sync()
}
