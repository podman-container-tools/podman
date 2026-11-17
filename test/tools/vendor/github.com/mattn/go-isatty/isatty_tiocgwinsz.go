//go:build (linux || aix || zos) && !appengine && !tinygo
// +build linux aix zos
// +build !appengine
// +build !tinygo

package isatty

import "golang.org/x/sys/unix"

// IsTerminal return true if the file descriptor is terminal.
// TIOCGWINSZ is used instead of TCGETS because TCGETS shares its ioctl
// number with SNDCTL_TMR_TIMEBASE of the OSS sound API, so it may succeed
// (and even change the device mode) on non-tty devices. musl's isatty does
// the same.
func IsTerminal(fd uintptr) bool {
	_, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	return err == nil
}

// IsCygwinTerminal return true if the file descriptor is a cygwin or msys2
// terminal. This is also always false on this environment.
func IsCygwinTerminal(fd uintptr) bool {
	return false
}
