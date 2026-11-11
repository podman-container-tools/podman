package xattrs

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"go.podman.io/storage/internal/rootlookupcache"
	"golang.org/x/sys/unix"
)

// O_PATH value on freebsd. We must define O_PATH ourselves
// until https://github.com/golang/go/issues/54355 is fixed.
const o_PATH = 0x00400000

// newLHandle creates a Handle for parentFile/fsBasename (using errorRoot/errorPath for error reporting).
// If fsBasename is a symbolic link, it refers to the symbolic link, not to the target.
func newLHandle(parentFile *os.File, fsBasename string, errorRoot *os.Root, errorPath string) (*Handle, error) {
	// A path per fs.ValidPath should not contain a ".."; reject it so that we can ensure no escape from parentFile.
	if fsBasename == ".." {
		return nil, fmt.Errorf("trailing .. in newLhandle in %q", errorPath)
	}
	fd, err := syscallConnControl(parentFile, func(parentDir uintptr) (int, error) {
		return unix.Openat(int(parentDir), filepath.FromSlash(fsBasename), o_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
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

var namespaceMap = map[string]int{
	"user":   unix.EXTATTR_NAMESPACE_USER,
	"system": unix.EXTATTR_NAMESPACE_SYSTEM,
}

// getxattr is the logic underlying Lgetxattr and RootLgetxattr.
// Returns a []byte slice if the xattr is set and nil otherwise.
func getxattr(syscallName string, pathInError func() string, getSyscall func(dest []byte) (int, error)) ([]byte, error) {
	size, errno := getSyscall(nil)
	if errno != nil {
		if errno == unix.ENOATTR {
			return nil, nil
		}
		return nil, &os.PathError{Op: syscallName, Path: pathInError(), Err: errno}
	}
	if size == 0 {
		return []byte{}, nil
	}

	dest := make([]byte, size)
	size, errno = getSyscall(dest)
	if errno != nil {
		return nil, &os.PathError{Op: syscallName, Path: pathInError(), Err: errno}
	}

	return dest[:size], nil
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
	return getxattr("Handle.Getxattr", h.pathInError, func(dest []byte) (int, error) {
		return unix.Fgetxattr(h.fd, attr, dest)
	})
}

// RootLgetxattr retrieves the value of the extended attribute identified by attr
// in fsPath (per fs.ValidPath) under root.
// Returns a []byte slice if the xattr is set and nil otherwise.
func RootLgetxattr(root *os.Root, fsPath string, attr string) ([]byte, error) {
	// We can’t use root.Open(fsPath) because it follows trailing symlinks.
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
func Lsetxattr(path string, attr string, value []byte, flags int) error {
	if flags != 0 {
		// FIXME: Flags are not supported on FreeBSD, but we can implement
		// them mimicking the behavior of the Linux implementation.
		// See lsetxattr(2) on Linux for more information.
		return unix.ENOTSUP
	}

	if err := unix.Lsetxattr(path, attr, value, 0); err != nil {
		// storage/pkg/archive.extractTarFileEntry expects ENOTSUP for unsupported operations;
		// we could extend the check there, but ENOATTR is not available on all systems, so doing
		// it here is simpler.
		if errors.Is(err, unix.ENOATTR) {
			err = unix.ENOTSUP
		}
		return &os.PathError{Op: "lsetxattr", Path: path, Err: err}
	}

	return nil
}

// listxattrNS lists extended attributes within a single namespace.
func listxattrNS(nsid int, syscallName string, pathInError func() string, listOperation func(nsid int, dest []byte) (int, error)) ([]string, error) {
	size, errno := listOperation(nsid, nil)
	if errno != nil {
		return nil, &os.PathError{Op: syscallName, Path: pathInError(), Err: errno}
	}
	if size == 0 {
		return []string{}, nil
	}

	dest := make([]byte, size)
	size, errno = listOperation(nsid, dest)
	if errno != nil {
		return nil, &os.PathError{Op: syscallName, Path: pathInError(), Err: errno}
	}

	var attrs []string
	for i := 0; i < size; {
		// Each attribute is preceded by a single byte length
		length := int(dest[i])
		i++
		if i+length > size {
			break
		}
		attrs = append(attrs, string(dest[i:i+length]))
		i += length
	}

	return attrs, nil
}

// listxattr is the logic underlying Llistxattr and RootLlistxattr.
func listxattr(syscallName string, pathInError func() string, listOperation func(nsid int, dest []byte) (int, error)) ([]string, error) {
	// This can’t use unix.Llistxattr etc. because those discard the namespace value: https://github.com/golang/go/issues/54357	.
	//
	// Instead, x/sys/unix provides *NS operations and we must iterate over namespaces ourselves.
	attrs := []string{}

	for namespaceName, namespace := range namespaceMap {
		namespaceAttrs, err := listxattrNS(namespace, syscallName, pathInError, listOperation)
		if err != nil {
			return nil, err
		}

		for _, attr := range namespaceAttrs {
			attrs = append(attrs, namespaceName+"."+attr)
		}
	}

	return attrs, nil
}

// Llistxattr lists extended attributes associated with the given path
// in the file system.
func Llistxattr(path string) ([]string, error) {
	return listxattr("LlistxattrNS", func() string { return path }, func(nsid int, dest []byte) (int, error) {
		return unix.LlistxattrNS(path, nsid, dest)
	})
}

// Listxattr lists extended attributes associated with the given handle.
func (h *Handle) Listxattr() ([]string, error) {
	return listxattr("Handle.Listxattr", h.pathInError, func(nsid int, dest []byte) (int, error) {
		return unix.FlistxattrNS(h.fd, nsid, dest)
	})
}

// RootLlistxattr lists extended attributes associated with
// fsPath (per fs.ValidPath) under root.
func RootLlistxattr(root *os.Root, fsPath string) ([]string, error) {
	// We can’t use root.Open(fsPath) because it follows trailing symlinks.
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
