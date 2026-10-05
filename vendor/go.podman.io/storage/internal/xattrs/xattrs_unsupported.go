//go:build !linux && !darwin && !freebsd

package xattrs

import (
	"os"
	"syscall"

	"go.podman.io/storage/internal/rootlookupcache"
)

const (
	// Value is larger than the maximum size allowed
	E2BIG syscall.Errno = syscall.Errno(0)

	// Operation not supported
	ENOTSUP syscall.Errno = syscall.Errno(0)

	// Value is too small or too large for maximum size allowed
	EOVERFLOW syscall.Errno = syscall.Errno(0)
)

// XattrHandle allows efficient llistxattr / llistxattr operations on a single file.
type Handle struct {
}

// NewLHandle creates a Handle for fsBasename in parentRoot, which was the one last obtained from rootCache.
// If fsBasename is a symbolic link, it refers to the symbolic link, not to the target.
//
// The handle must be closed using .Close().
func NewLHandle(parentRoot *os.Root, fsBasename string, rootCache *rootlookupcache.Cache) (*Handle, error) {
	// This is not implemented, but we don’t fail, so that callers don’t need to add an extra ErrNotSupportedPlatform check;
	// Those checks need to exist in the individual operations anyway.
	return &Handle{}, nil
}

func (h *Handle) Close() error {
	return nil
}

// Lgetxattr is not supported on platforms other than linux.
func Lgetxattr(path string, attr string) ([]byte, error) {
	return nil, ErrNotSupportedPlatform
}

// Getxattr retrieves the value of the extended attribute identified by attr.
// Returns a []byte slice if the xattr is set and nil otherwise.
func (h *Handle) Getxattr(attr string) ([]byte, error) {
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

// Listxattr lists extended attributes associated with the given handle.
func (h *Handle) Listxattr() ([]string, error) {
	return nil, ErrNotSupportedPlatform
}

// RootLlistxattr lists extended attributes associated with
// fsPath (per fs.ValidPath) under root.
func RootLlistxattr(root *os.Root, fsPath string) ([]string, error) {
	return nil, ErrNotSupportedPlatform
}
