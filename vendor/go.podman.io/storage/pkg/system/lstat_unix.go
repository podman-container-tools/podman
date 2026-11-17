//go:build !windows

package system

import (
	"os"
	"syscall"

	"go.podman.io/storage/internal/stat"
)

// Lstat takes a path to a file and returns
// a system.StatT type pertaining to that file.
//
// Throws an error if the file does not exist
func Lstat(path string) (*StatT, error) {
	s := &syscall.Stat_t{}
	if err := syscall.Lstat(path, s); err != nil {
		return nil, &os.PathError{Op: "Lstat", Path: path, Err: err}
	}
	return stat.FromStatT(s), nil
}
