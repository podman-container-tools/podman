//go:build netbsd || freebsd || darwin

package archive

import (
	"archive/tar"
	"os"

	"golang.org/x/sys/unix"
)

// syscallMode maps a Go os.FileMode to chmod(2) bits, preserving the
// setuid/setgid/sticky bits that a raw uint32(FileMode) cast silently drops.
func syscallMode(i os.FileMode) (o uint32) {
	o = uint32(i.Perm())
	if i&os.ModeSetuid != 0 {
		o |= unix.S_ISUID
	}
	if i&os.ModeSetgid != 0 {
		o |= unix.S_ISGID
	}
	if i&os.ModeSticky != 0 {
		o |= unix.S_ISVTX
	}
	return o
}

func handleLChmod(_ *tar.Header, path string, hdrInfo os.FileInfo, forceMask *os.FileMode) error {
	permissionsMask := hdrInfo.Mode()
	if forceMask != nil {
		permissionsMask = *forceMask
	}
	return unix.Fchmodat(unix.AT_FDCWD, path, syscallMode(permissionsMask), unix.AT_SYMLINK_NOFOLLOW)
}
