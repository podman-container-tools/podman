//go:build linux

package system

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// Syncfs synchronizes the filesystem containing the given path.
func Syncfs(path string) error {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open for syncfs: %w", err)
	}
	defer unix.Close(fd)

	if err := unix.Syncfs(fd); err != nil {
		return fmt.Errorf("syncfs: %w", err)
	}
	return nil
}
