//go:build linux

package cupwriter

import "golang.org/x/sys/unix"

const ioctlReadTermios = unix.TCGETS
