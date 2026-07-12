//go:build linux && !remote

package abi

import (
	"os"

	"github.com/cyphar/filepath-securejoin/pathrs-lite"
)

// openSymlinkPath opens the path under root using securejoin.OpenatInRoot().
func openSymlinkPath(root *os.File, unsafePath string, flags int) (*os.File, error) {
	file, err := pathrs.OpenatInRoot(root, unsafePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return pathrs.Reopen(file, flags)
}
