//go:build !remote && freebsd

package util

import "golang.org/x/sys/unix"

var hostRlimitResources = map[string]int{
	"RLIMIT_AS":      unix.RLIMIT_AS,
	"RLIMIT_CORE":    unix.RLIMIT_CORE,
	"RLIMIT_CPU":     unix.RLIMIT_CPU,
	"RLIMIT_DATA":    unix.RLIMIT_DATA,
	"RLIMIT_FSIZE":   unix.RLIMIT_FSIZE,
	"RLIMIT_MEMLOCK": unix.RLIMIT_MEMLOCK,
	"RLIMIT_NOFILE":  unix.RLIMIT_NOFILE,
	"RLIMIT_NPROC":   unix.RLIMIT_NPROC,
	"RLIMIT_RSS":     unix.RLIMIT_RSS,
	"RLIMIT_STACK":   unix.RLIMIT_STACK,
}
