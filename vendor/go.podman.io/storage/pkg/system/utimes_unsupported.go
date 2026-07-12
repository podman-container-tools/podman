//go:build !unix

package system

import "syscall"

// LUtimesNano is only supported on Unix systems.
func LUtimesNano(path string, ts []syscall.Timespec) error {
	return ErrNotSupportedPlatform
}
