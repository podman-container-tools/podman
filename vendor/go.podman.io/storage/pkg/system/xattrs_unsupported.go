//go:build !linux && !darwin && !freebsd

package system

import (
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
