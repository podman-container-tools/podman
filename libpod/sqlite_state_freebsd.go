//go:build !remote && freebsd

package libpod

import (
	"strings"

	"golang.org/x/sys/unix"

	"github.com/sirupsen/logrus"
)

// networkFSTypePrefixes lists FreeBSD filesystem type name prefixes for
// filesystems that do not support POSIX mmap required by SQLite WAL mode.
// Prefixes match both "nfs" and "nfs4" with a single entry.
var networkFSTypePrefixes = []string{
	"nfs",   // NFSv3 and NFSv4
	"smbfs", // SMB/CIFS
	"afs",   // Andrew Filesystem
}

// isNetworkFilesystem returns true if dir is on a filesystem incompatible with
// SQLite WAL mode. FreeBSD uses Fstypename [16]byte instead of a magic number.
// On Statfs error, returns false so verifyJournalMode() acts as the safety net.
func isNetworkFilesystem(dir string) bool {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		logrus.Warnf("SQLite WAL: could not statfs %q: %v; assuming local filesystem", dir, err)
		return false
	}
	typeName := unix.ByteSliceToString(st.Fstypename[:])
	lower := strings.ToLower(typeName)
	for _, prefix := range networkFSTypePrefixes {
		if strings.HasPrefix(lower, prefix) {
			logrus.Infof("SQLite WAL mode disabled: %q is on %s (no POSIX mmap support)", dir, typeName)
			return true
		}
	}
	return false
}
