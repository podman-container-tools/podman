package stat

import "syscall"

// FromStatT converts a syscall.Stat_t type to a StatT type
func FromStatT(s *syscall.Stat_t) *StatT {
	return &StatT{
		size: s.Size,
		mode: s.Mode,
		uid:  s.Uid,
		gid:  s.Gid,
		rdev: uint64(s.Rdev), //nolint:unconvert
		mtim: s.Mtim,
		dev:  uint64(s.Dev), //nolint:unconvert
	}
}
