// SPDX-License-Identifier: Apache-2.0

//go:build unix && !linux

package metadata

import (
	"fmt"
	"os"
	"syscall"
)

func openFile(path string) (*os.File, error) {
	// Reject stable special files before open. O_NONBLOCK prevents a FIFO
	// replacement from blocking between this check and the descriptor check in
	// openRegularFile.
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is %w", path, errNotRegularFile)
	}

	return os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}
