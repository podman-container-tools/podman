//go:build !windows

package system

import (
	"os"
	"strconv"
	"syscall"

	"go.podman.io/storage/internal/stat"
)

// StatT type contains status of a file. It contains metadata
// like permission, owner, group, size, etc about a file.
type StatT = stat.StatT

// Stat takes a path to a file and returns
// a system.StatT type pertaining to that file.
//
// Throws an error if the file does not exist
func Stat(path string) (*StatT, error) {
	s := &syscall.Stat_t{}
	if err := syscall.Stat(path, s); err != nil {
		return nil, &os.PathError{Op: "Stat", Path: path, Err: err}
	}
	return stat.FromStatT(s), nil
}

// Fstat takes an open file descriptor and returns
// a system.StatT type pertaining to that file.
//
// Throws an error if the file descriptor is invalid
func Fstat(fd int) (*StatT, error) {
	s := &syscall.Stat_t{}
	if err := syscall.Fstat(fd, s); err != nil {
		return nil, &os.PathError{Op: "Fstat", Path: strconv.Itoa(fd), Err: err}
	}
	return stat.FromStatT(s), nil
}
