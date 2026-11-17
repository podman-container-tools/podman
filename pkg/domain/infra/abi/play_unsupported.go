//go:build freebsd && !remote

package abi

import (
	"errors"
	"os"
)

// openSymlinkPath is not supported on this platform.
func openSymlinkPath(_ *os.File, _ string, _ int) (*os.File, error) {
	return nil, errors.New("cannot safely open symlink on this platform")
}
