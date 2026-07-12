//go:build !windows && !plan9

package internal

import (
	"golang.org/x/sys/unix"
)

// Stat_t is the Linux Stat_t.
type Stat_t = unix.Stat_t
