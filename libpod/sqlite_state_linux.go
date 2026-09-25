//go:build !remote && linux

package libpod

import (
	"golang.org/x/sys/unix"

	"github.com/sirupsen/logrus"
)

// isNetworkFilesystem returns true if dir is on a filesystem incompatible with
// SQLite WAL mode. On Statfs error, returns false so verifyJournalMode() acts
// as the safety net.
//
// FUSE is excluded: most FUSE mounts are local. FUSE-over-NFS is caught by
// verifyJournalMode() after the connection is opened.
func isNetworkFilesystem(dir string) bool {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		logrus.Warnf("SQLite WAL: could not statfs %q: %v; assuming local filesystem", dir, err)
		return false
	}
	var fsName string
	switch uint64(st.Type) {
	case uint64(unix.NFS_SUPER_MAGIC):
		fsName = "NFS"
	case uint64(unix.CIFS_SUPER_MAGIC):
		fsName = "CIFS"
	case uint64(unix.SMB_SUPER_MAGIC):
		fsName = "SMB"
	case uint64(unix.SMB2_SUPER_MAGIC):
		fsName = "SMB2"
	case uint64(unix.AFS_SUPER_MAGIC):
		fsName = "AFS"
	}
	if fsName != "" {
		logrus.Infof("SQLite WAL mode disabled: %q is on %s (no POSIX mmap support)", dir, fsName)
		return true
	}
	return false
}
