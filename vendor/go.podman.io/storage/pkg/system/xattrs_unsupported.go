//go:build !linux && !darwin && !freebsd

package system

import (
	"os"
	"syscall"
)

const (
	// Value is larger than the maximum size allowed
	E2BIG syscall.Errno = syscall.Errno(0)

	// Operation not supported
	ENOTSUP syscall.Errno = syscall.Errno(0)

	// Value is too small or too large for maximum size allowed
	EOVERFLOW syscall.Errno = syscall.Errno(0)
)

// Lgetxattr is not supported on platforms other than linux.
func Lgetxattr(path string, attr string) ([]byte, error) {
	return nil, ErrNotSupportedPlatform
}

// RootLgetxattr retrieves the value of the extended attribute identified by attr
// in fsPath (per fs.ValidPath) under root.
// Returns a []byte slice if the xattr is set and nil otherwise.
func RootLgetxattr(root *os.Root, fsPath string, attr string) ([]byte, error) {
	return nil, ErrNotSupportedPlatform
}

// Lsetxattr is not supported on platforms other than linux.
func Lsetxattr(path string, attr string, data []byte, flags int) error {
	return ErrNotSupportedPlatform
}

// Llistxattr is not supported on platforms other than linux.
func Llistxattr(path string) ([]string, error) {
	return nil, ErrNotSupportedPlatform
}

// RootLlistxattr lists extended attributes associated with
// fsPath (per fs.ValidPath) under root.
func RootLlistxattr(root *os.Root, fsPath string) ([]string, error) {
	return nil, ErrNotSupportedPlatform
}
