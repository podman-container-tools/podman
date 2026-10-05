package xattrs

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"go.podman.io/storage/internal/rootlookupcache"
	"golang.org/x/sys/unix"
)

// O_PATH value on freebsd. We must define O_PATH ourselves
// until https://github.com/golang/go/issues/54355 is fixed.
const o_PATH = 0x00400000 //nolint:staticcheck // ST1003: should not use ALL_CAPS

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

func xattrToExtattr(xattr string) (namespace int, extattr string, err error) {
	namespaceName, extattr, found := strings.Cut(xattr, ".")
	if !found {
		return -1, "", unix.ENOTSUP
	}

	namespace, ok := namespaceMap[namespaceName]
	if !ok {
		return -1, "", unix.ENOTSUP
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

// Getxattr retrieves the value of the extended attribute identified by attr.
// Returns a []byte slice if the xattr is set and nil otherwise.
func (h *Handle) Getxattr(attr string) ([]byte, error) {
	namespace, extattr, err := xattrToExtattr(attr)
	if err != nil {
		return nil, err
	}
	return extattrGetFd(h.fd, h.pathInError, namespace, extattr)
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

// Listxattr lists extended attributes associated with the given handle.
func (h *Handle) Listxattr() ([]string, error) {
	return listxattr(func(namespace int) ([]string, error) {
		return extattrListFd(h.fd, h.pathInError, namespace)
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
