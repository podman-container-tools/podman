package system

import (
	"golang.org/x/sys/unix"
)

const (
	// Value is larger than the maximum size allowed
	E2BIG unix.Errno = unix.E2BIG

	// Operation not supported
	ENOTSUP unix.Errno = unix.ENOTSUP

	// Not in x/sys/unix as of v0.40.0.
	O_RESOLVE_BENEATH = 0x00001000
)
