package xattrs

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"go.podman.io/storage/internal/rootlookupcache"
	"golang.org/x/sys/unix"
)

// newLHandle creates a Handle for parentFile/fsBasename (using errorRoot/errorPath for error reporting).
// If fsBasename is a symbolic link, it refers to the symbolic link, not to the target.
func newLHandle(parentFile *os.File, fsBasename string, errorRoot *os.Root, errorPath string) (*Handle, error) {
	// A path per fs.ValidPath should not contain a ".."; reject it so that we can ensure no escape from parentFile.
	if fsBasename == ".." {
		return nil, fmt.Errorf("trailing .. in newLhandle in %q", errorPath)
	}
	fd, err := syscallConnControl(parentFile, func(parentDir uintptr) (int, error) {
		return unix.Openat(int(parentDir), filepath.FromSlash(fsBasename), unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	})
	if err != nil {
		return nil, err
	}

	return &Handle{
		fd:        fd,
		errorRoot: errorRoot,
		errorPath: errorPath,
	}, nil
}

// NewLHandle creates a Handle for fsBasename in parentRoot, which was the one last obtained from rootCache.
// If fsBasename is a symbolic link, it refers to the symbolic link, not to the target.
//
// The handle must be closed using .Close().
func NewLHandle(parentRoot *os.Root, fsBasename string, rootCache *rootlookupcache.Cache) (*Handle, error) {
	parentFile, err := rootCache.FileForRoot(parentRoot)
	if err != nil {
		return nil, err
	}
	return newLHandle(parentFile, fsBasename, parentRoot, fsBasename)
}

// getxattr is the logic underlying Lgetxattr and RootLgetxattr.
// Returns a []byte slice if the xattr is set and nil otherwise.
func getxattr(syscallName string, pathInError func() string, getSyscall func(dest []byte) (int, error)) ([]byte, error) {
	// Start with a 128 length byte array
	dest := make([]byte, 128)
	sz, errno := getSyscall(dest)

	for errno == unix.ERANGE {
		// Buffer too small, use zero-sized buffer to get the actual size
		sz, errno = getSyscall([]byte{})
		if errno != nil {
			return nil, &os.PathError{Op: syscallName, Path: pathInError(), Err: errno}
		}
		dest = make([]byte, sz)
		sz, errno = getSyscall(dest)
	}

	switch {
	case errno == unix.ENODATA:
		return nil, nil
	case errno != nil:
		return nil, &os.PathError{Op: syscallName, Path: pathInError(), Err: errno}
	}

	return dest[:sz], nil
}

// Lgetxattr retrieves the value of the extended attribute identified by attr
// and associated with the given path in the file system.
// Returns a []byte slice if the xattr is set and nil otherwise.
func Lgetxattr(path string, attr string) ([]byte, error) {
	return getxattr("lgetxattr", func() string { return path }, func(dest []byte) (int, error) {
		return unix.Lgetxattr(path, attr, dest)
	})
}

// Getxattr retrieves the value of the extended attribute identified by attr.
// Returns a []byte slice if the xattr is set and nil otherwise.
func (h *Handle) Getxattr(attr string) ([]byte, error) {
	// Ideally we would use getxattrat() here, but as of 2026-05 that might be too recent.
	return getxattr("Handle.Getxattr", h.pathInError, func(dest []byte) (int, error) {
		return unix.Getxattr(fmt.Sprintf("/proc/self/fd/%d", h.fd), attr, dest)
	})
}

// RootLgetxattr retrieves the value of the extended attribute identified by attr
// in fsPath (per fs.ValidPath) under root.
// Returns a []byte slice if the xattr is set and nil otherwise.
func RootLgetxattr(root *os.Root, fsPath string, attr string) ([]byte, error) {
	// We can use neither root.Open nor pathrs.OpenatInRoot to get the target file descriptor because they follow trailing symlinks.
	parentDir, err := root.Open(path.Dir(fsPath))
	if err != nil {
		return nil, err
	}
	defer parentDir.Close()

	handle, err := newLHandle(parentDir, path.Base(fsPath), root, fsPath)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	return handle.Getxattr(attr)
}

// Lsetxattr sets the value of the extended attribute identified by attr
// and associated with the given path in the file system.
func Lsetxattr(path string, attr string, data []byte, flags int) error {
	if err := unix.Lsetxattr(path, attr, data, flags); err != nil {
		return &os.PathError{Op: "lsetxattr", Path: path, Err: err}
	}

	return nil
}

// listxattr is the logic underlying Llistxattr and RootLlistxattr.
func listxattr(syscallName string, pathInError func() string, listSyscall func(dest []byte) (int, error)) ([]string, error) {
	dest := make([]byte, 128)
	sz, errno := listSyscall(dest)

	for errno == unix.ERANGE {
		// Buffer too small, use zero-sized buffer to get the actual size
		sz, errno = listSyscall([]byte{})
		if errno != nil {
			return nil, &os.PathError{Op: syscallName, Path: pathInError(), Err: errno}
		}

		dest = make([]byte, sz)
		sz, errno = listSyscall(dest)
	}
	if errno != nil {
		return nil, &os.PathError{Op: syscallName, Path: pathInError(), Err: errno}
	}

	var attrs []string
	for token := range bytes.SplitSeq(dest[:sz], []byte{0}) {
		if len(token) > 0 {
			attrs = append(attrs, string(token))
		}
	}

	return attrs, nil
}

// Llistxattr lists extended attributes associated with the given path
// in the file system.
func Llistxattr(path string) ([]string, error) {
	return listxattr("llistxattr", func() string { return path }, func(dest []byte) (int, error) {
		return unix.Llistxattr(path, dest)
	})
}

// Listxattr lists extended attributes associated with the given handle.
func (h *Handle) Listxattr() ([]string, error) {
	// Ideally we would use listxattrat() here, but as of 2026-05 that might be too recent.
	return listxattr("Handle.Listxattr", h.pathInError, func(dest []byte) (int, error) {
		return unix.Listxattr(fmt.Sprintf("/proc/self/fd/%d", h.fd), dest)
	})
}

// RootLlistxattr lists extended attributes associated with
// fsPath (per fs.ValidPath) under root.
func RootLlistxattr(root *os.Root, fsPath string) ([]string, error) {
	// We can use neither root.Open nor pathrs.OpenatInRoot to get the target file descriptor because they follow trailing symlinks.
	parentDir, err := root.Open(path.Dir(fsPath))
	if err != nil {
		return nil, err
	}
	defer parentDir.Close()

	handle, err := newLHandle(parentDir, path.Base(fsPath), root, fsPath)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	return handle.Listxattr()
}
