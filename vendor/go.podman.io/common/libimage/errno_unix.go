//go:build !windows

package libimage

import "syscall"

const errNoSpace = syscall.ENOSPC
