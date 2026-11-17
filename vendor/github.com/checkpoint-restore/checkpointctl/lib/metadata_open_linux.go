// SPDX-License-Identifier: Apache-2.0

//go:build linux

package metadata

import (
	"fmt"
	"os"

	pathrs "github.com/cyphar/filepath-securejoin/pathrs-lite"
	"golang.org/x/sys/unix"
)

// openFile opens path without activating the inode, verifies that the opened
// inode is a regular file, and then reopens that same inode for reading.
func openFile(path string) (*os.File, error) {
	handle, err := os.OpenFile(path, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	handleInfo, err := handle.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if !handleInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is %w", path, errNotRegularFile)
	}

	return reopenFile(handle, path, handleInfo)
}

func reopenFile(handle *os.File, path string, handleInfo os.FileInfo) (*os.File, error) {
	// Linux openat(2) does not support AT_EMPTY_PATH. Reopen the pinned O_PATH
	// descriptor using pathrs, which protects against unsafe procfs mounts.
	f, err := pathrs.Reopen(handle, unix.O_RDONLY)
	if err != nil {
		return nil, fmt.Errorf("reopen %s: %w", path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("stat reopened %s: %w", path, err)
	}
	if !os.SameFile(handleInfo, info) {
		_ = f.Close()
		return nil, fmt.Errorf("reopened file does not match %s", path)
	}

	return f, nil
}
