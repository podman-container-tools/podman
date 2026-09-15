package system

import (
	"bytes"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

const (
	// Value is larger than the maximum size allowed
	E2BIG unix.Errno = unix.E2BIG

	// Operation not supported
	ENOTSUP unix.Errno = unix.ENOTSUP

	// Not in x/sys/unix as of v0.40.0.
	O_RESOLVE_BENEATH = 0x00001000
)

// getxattr is the logic underlying Lgetxattr and Fgetxattr.
// Returns a []byte slice if the xattr is set and nil otherwise.
func getxattr(syscallName string, pathInError string, getSyscall func(dest []byte) (int, error)) ([]byte, error) {
	// Start with a 128 length byte array
	dest := make([]byte, 128)
	sz, errno := getSyscall(dest)

	for errno == unix.ERANGE {
		// Buffer too small, use zero-sized buffer to get the actual size
		sz, errno = getSyscall([]byte{})
		if errno != nil {
			return nil, &os.PathError{Op: syscallName, Path: pathInError, Err: errno}
		}
		dest = make([]byte, sz)
		sz, errno = getSyscall(dest)
	}

	switch {
	case errno == unix.ENOATTR:
		return nil, nil
	case errno != nil:
		return nil, &os.PathError{Op: syscallName, Path: pathInError, Err: errno}
	}

	return dest[:sz], nil
}

// Lgetxattr retrieves the value of the extended attribute identified by attr
// and associated with the given path in the file system.
// Returns a []byte slice if the xattr is set and nil otherwise.
func Lgetxattr(path string, attr string) ([]byte, error) {
	return getxattr("lgetxattr", path, func(dest []byte) (int, error) {
		return unix.Lgetxattr(path, attr, dest)
	})
}

// RootLgetxattr retrieves the value of the extended attribute identified by attr
// in fsPath (per fs.ValidPath) under root.
// Returns a []byte slice if the xattr is set and nil otherwise.
func RootLgetxattr(root *os.Root, fsPath string, attr string) ([]byte, error) {
	// We can’t use root.Open(fsPath) because it follows trailing symlinks.
	rootFD, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer rootFD.Close()
	fd, err := syscallConnControl(rootFD, func(rootFD uintptr) (int, error) {
		// macOS does not have O_PATH, so hope the user has enough permissions.
		return unix.Openat(int(rootFD), filepath.FromSlash(fsPath), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_SYMLINK|O_RESOLVE_BENEATH, 0)
	})
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	return getxattr("RootLgetxattr", fsPath, func(dest []byte) (int, error) {
		return unix.Fgetxattr(fd, attr, dest)
	})
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
func listxattr(syscallName string, pathInError string, listSyscall func(dest []byte) (int, error)) ([]string, error) {
	dest := make([]byte, 128)
	sz, errno := listSyscall(dest)

	for errno == unix.ERANGE {
		// Buffer too small, use zero-sized buffer to get the actual size
		sz, errno = listSyscall([]byte{})
		if errno != nil {
			return nil, &os.PathError{Op: syscallName, Path: pathInError, Err: errno}
		}

		dest = make([]byte, sz)
		sz, errno = listSyscall(dest)
	}
	if errno != nil {
		return nil, &os.PathError{Op: syscallName, Path: pathInError, Err: errno}
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
	return listxattr("llistxattr", path, func(dest []byte) (int, error) {
		return unix.Llistxattr(path, dest)
	})
}

// RootLlistxattr lists extended attributes associated with
// fsPath (per fs.ValidPath) under root.
func RootLlistxattr(root *os.Root, fsPath string) ([]string, error) {
	// We can’t use root.Open(fsPath) because it follows trailing symlinks.
	rootFD, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer rootFD.Close()
	// macOS does not have O_PATH, so hope the user has enough permissions.
	fd, err := syscallConnControl(rootFD, func(rootFD uintptr) (int, error) {
		return unix.Openat(int(rootFD), filepath.FromSlash(fsPath), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_SYMLINK|O_RESOLVE_BENEATH, 0)
	})
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	return listxattr("RootLlistxattr", fsPath, func(dest []byte) (int, error) {
		return unix.Flistxattr(fd, dest)
	})
}
