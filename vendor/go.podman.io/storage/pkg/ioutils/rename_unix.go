//go:build !windows

package ioutils

import "os"

// rename renames oldpath to newpath, replacing newpath if it exists.
func rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}
