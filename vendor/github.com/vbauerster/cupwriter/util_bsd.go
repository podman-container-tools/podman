//go:build darwin || dragonfly || freebsd || netbsd || openbsd

package cupwriter

import "golang.org/x/sys/unix"

const ioctlReadTermios = unix.TIOCGETA
