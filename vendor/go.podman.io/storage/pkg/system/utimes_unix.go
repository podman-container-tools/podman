//go:build unix

package system

import (
	"syscall"

	"golang.org/x/sys/unix"
)

// LUtimesNano is used to change access and modification time of the specified path.
func LUtimesNano(path string, ts []syscall.Timespec) error {
	ts_ := [2]unix.Timespec{
		{Sec: ts[0].Sec, Nsec: ts[0].Nsec},
		{Sec: ts[1].Sec, Nsec: ts[1].Nsec},
	}
	err := unix.UtimesNanoAt(unix.AT_FDCWD, path, ts_[:], unix.AT_SYMLINK_NOFOLLOW)
	if err == unix.ENOSYS {
		err = nil
	}
	return err
}
