package internal

import (
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// InfoToStat takes a platform native FileInfo and converts it into a 9P2000.L compatible Stat_t
func InfoToStat(fi os.FileInfo) *Stat_t {
	nativeStat := fi.Sys().(*syscall.Stat_t)
	return &Stat_t{
		Dev:     nativeStat.Dev,
		Ino:     nativeStat.Ino,
		Nlink:   nativeStat.Nlink,
		Mode:    nativeStat.Mode,
		Uid:     nativeStat.Uid,
		Gid:     nativeStat.Gid,
		Rdev:    nativeStat.Rdev,
		Size:    nativeStat.Size,
		Blksize: int32(nativeStat.Blksize),
		Blocks:  nativeStat.Blocks,
		Atim:    unix.NsecToTimespec(syscall.TimespecToNsec(nativeStat.Atim)),
		Mtim:    unix.NsecToTimespec(syscall.TimespecToNsec(nativeStat.Mtim)),
		Ctim:    unix.NsecToTimespec(syscall.TimespecToNsec(nativeStat.Ctim)),
	}
}
