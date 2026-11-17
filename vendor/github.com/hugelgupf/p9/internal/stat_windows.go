package internal

import (
	"os"
)

// NOTE: taken from amd64 Linux
type Timespec struct {
	Sec  int64
	Nsec int64
}

type Stat_t struct {
	Dev     uint64
	Ino     uint64
	Nlink   uint64
	Mode    uint32
	Uid     uint32
	Gid     uint32
	Rdev    uint64
	Size    int64
	Blksize int64
	Blocks  int64
	Atim    Timespec
	Mtim    Timespec
	Ctim    Timespec
}

// InfoToStat takes a platform native FileInfo and converts it into a 9P2000.L compatible Stat_t
func InfoToStat(fi os.FileInfo) *Stat_t {
	return &Stat_t{
		Size: fi.Size(),
		Mode: uint32(modeFromOS(fi.Mode())),
		Mtim: Timespec{
			Sec:  fi.ModTime().Unix(),
			Nsec: fi.ModTime().UnixNano(),
		},
	}

}

// TODO: copied from pkg p9
// we should probably migrate the OS methods from p9 into sys
const (
	FileModeMask        uint32 = 0170000
	ModeSocket                 = 0140000
	ModeSymlink                = 0120000
	ModeRegular                = 0100000
	ModeBlockDevice            = 060000
	ModeDirectory              = 040000
	ModeCharacterDevice        = 020000
	ModeNamedPipe              = 010000
)

func modeFromOS(mode os.FileMode) uint32 {
	m := uint32(mode.Perm())
	switch {
	case mode.IsDir():
		m |= ModeDirectory
	case mode&os.ModeSymlink != 0:
		m |= ModeSymlink
	case mode&os.ModeSocket != 0:
		m |= ModeSocket
	case mode&os.ModeNamedPipe != 0:
		m |= ModeNamedPipe
	case mode&os.ModeCharDevice != 0:
		m |= ModeCharacterDevice
	case mode&os.ModeDevice != 0:
		m |= ModeBlockDevice
	default:
		m |= ModeRegular
	}
	return m
}
