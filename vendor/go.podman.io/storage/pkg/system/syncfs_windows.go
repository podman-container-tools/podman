//go:build windows

package system

import "errors"

// Syncfs is not supported on Windows.
func Syncfs(path string) error {
	return errors.New("syncfs is not supported on Windows")
}
