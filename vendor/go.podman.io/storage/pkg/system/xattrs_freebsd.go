package system

import (
	"golang.org/x/sys/unix"
)

const (
	// Value is larger than the maximum size allowed
	E2BIG unix.Errno = unix.E2BIG

	// Operation not supported
	ENOTSUP unix.Errno = unix.ENOTSUP

	// Value is too small or too large for maximum size allowed
	EOVERFLOW unix.Errno = unix.EOVERFLOW
)
