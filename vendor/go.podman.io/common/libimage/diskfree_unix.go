//go:build !remote && !windows

package libimage

import "golang.org/x/sys/unix"

// diskFree returns the number of bytes available to unprivileged users on
// the file system containing path.
func diskFree(path string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil //nolint:unconvert // Statfs_t field types differ across platforms
}
