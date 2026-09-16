package system

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	// Value is larger than the maximum size allowed
	E2BIG unix.Errno = unix.E2BIG

	// Operation not supported
	ENOTSUP unix.Errno = unix.ENOTSUP

	// Value is too small or too large for maximum size allowed
	EOVERFLOW unix.Errno = unix.EOVERFLOW
)

var namespaceMap = map[string]int{
	"user":   EXTATTR_NAMESPACE_USER,
	"system": EXTATTR_NAMESPACE_SYSTEM,
}

func xattrToExtattr(xattr string) (namespace int, extattr string, err error) {
	namespaceName, extattr, found := strings.Cut(xattr, ".")
	if !found {
		return -1, "", ENOTSUP
	}

	namespace, ok := namespaceMap[namespaceName]
	if !ok {
		return -1, "", ENOTSUP
	}
	return namespace, extattr, nil
}

// Lgetxattr retrieves the value of the extended attribute identified by attr
// and associated with the given path in the file system.
// Returns a []byte slice if the xattr is set and nil otherwise.
func Lgetxattr(path string, attr string) ([]byte, error) {
	namespace, extattr, err := xattrToExtattr(attr)
	if err != nil {
		return nil, err
	}
	return ExtattrGetLink(path, namespace, extattr)
}

// RootLgetxattr retrieves the value of the extended attribute identified by attr
// in fsPath (per fs.ValidPath) under root.
// Returns a []byte slice if the xattr is set and nil otherwise.
func RootLgetxattr(root *os.Root, fsPath string, attr string) ([]byte, error) {
	// O_PATH value on freebsd. We must define O_PATH ourselves
	// until https://github.com/golang/go/issues/54355 is fixed.
	const O_PATH = 0x00400000 //nolint:staticcheck // ST1003: should not use ALL_CAPS

	// We can’t use root.Open(fsPath) because it follows trailing symlinks.
	parentDir, err := root.Open(path.Dir(fsPath))
	if err != nil {
		return nil, err
	}
	defer parentDir.Close()
	// A path per fs.ValidPath should not contain a ".."; reject it so that we can ensure no escape from parentDir.
	fsBase := path.Base(fsPath)
	if fsBase == ".." {
		return nil, fmt.Errorf("trailing .. in RootLgetxattr(%q)", fsPath)
	}
	fd, err := syscallConnControl(parentDir, func(parentDir uintptr) (int, error) {
		return unix.Openat(int(parentDir), filepath.FromSlash(fsBase), O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	})
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	namespace, extattr, err := xattrToExtattr(attr)
	if err != nil {
		return nil, err
	}
	return extattrGetFd(fd, namespace, extattr)
}

// Lsetxattr sets the value of the extended attribute identified by attr
// and associated with the given path in the file system.
func Lsetxattr(path string, attr string, value []byte, flags int) error {
	if flags != 0 {
		// FIXME: Flags are not supported on FreeBSD, but we can implement
		// them mimicking the behavior of the Linux implementation.
		// See lsetxattr(2) on Linux for more information.
		return ENOTSUP
	}

	namespace, extattr, err := xattrToExtattr(attr)
	if err != nil {
		return err
	}
	return ExtattrSetLink(path, namespace, extattr, value)
}

// listxattr is the logic underlying Llistxattr and RootLlistxattr.
func listxattr(listOperation func(namespace int) ([]string, error)) ([]string, error) {
	attrs := []string{}

	for namespaceName, namespace := range namespaceMap {
		namespaceAttrs, err := listOperation(namespace)
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
	return listxattr(func(namespace int) ([]string, error) {
		return ExtattrListLink(path, namespace)
	})
}

// RootLlistxattr lists extended attributes associated with
// fsPath (per fs.ValidPath) under root.
func RootLlistxattr(root *os.Root, fsPath string) ([]string, error) {
	// O_PATH value on freebsd. We must define O_PATH ourselves
	// until https://github.com/golang/go/issues/54355 is fixed.
	const O_PATH = 0x00400000 //nolint:staticcheck // ST1003: should not use ALL_CAPS

	// We can’t use root.Open(fsPath) because it follows trailing symlinks.
	parentDir, err := root.Open(path.Dir(fsPath))
	if err != nil {
		return nil, err
	}
	defer parentDir.Close()
	// A path per fs.ValidPath should not contain a ".."; reject it so that we can ensure no escape from parentDir.
	fsBase := path.Base(fsPath)
	if fsBase == ".." {
		return nil, fmt.Errorf("trailing .. in RootLlistxattr(%q)", fsPath)
	}
	fd, err := syscallConnControl(parentDir, func(parentDir uintptr) (int, error) {
		return unix.Openat(int(parentDir), filepath.FromSlash(fsBase), O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	})
	if err != nil {
		return nil, err
	}
	defer unix.Close(fd)

	return listxattr(func(namespace int) ([]string, error) {
		return extattrListFd(fd, namespace)
	})
}
