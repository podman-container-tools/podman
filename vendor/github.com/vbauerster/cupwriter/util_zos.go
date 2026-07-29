//go:build zos

package cupwriter

import "golang.org/x/sys/unix"

const ioctlReadTermios = unix.TCGETS
