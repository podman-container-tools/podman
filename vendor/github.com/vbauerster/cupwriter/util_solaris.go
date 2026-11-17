//go:build solaris

package cupwriter

import "golang.org/x/sys/unix"

const ioctlReadTermios = unix.TCGETA
