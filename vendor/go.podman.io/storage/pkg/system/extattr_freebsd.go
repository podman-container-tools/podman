//go:build freebsd

package system

import (
	"go.podman.io/storage/internal/xattrs"
	"golang.org/x/sys/unix"
)

const (
	EXTATTR_NAMESPACE_EMPTY  = unix.EXTATTR_NAMESPACE_EMPTY
	EXTATTR_NAMESPACE_USER   = unix.EXTATTR_NAMESPACE_USER
	EXTATTR_NAMESPACE_SYSTEM = unix.EXTATTR_NAMESPACE_SYSTEM
)

// ExtattrGetLink retrieves the value of the extended attribute identified by attrname
// in the given namespace and associated with the given path in the file system.
// If the path is a symbolic link, the extended attribute is retrieved from the link itself.
// Returns a []byte slice if the extattr is set and nil otherwise.
func ExtattrGetLink(path string, attrnamespace int, attrname string) ([]byte, error) {
	return xattrs.ExtattrGetLink(path, attrnamespace, attrname)
}

// ExtattrSetLink sets the value of extended attribute identified by attrname
// in the given namespace and associated with the given path in the file system.
// If the path is a symbolic link, the extended attribute is set on the link itself.
func ExtattrSetLink(path string, attrnamespace int, attrname string, data []byte) error {
	return xattrs.ExtattrSetLink(path, attrnamespace, attrname, data)
}

// ExtattrListLink lists extended attributes associated with the given path
// in the specified namespace. If the path is a symbolic link, the attributes
// are listed from the link itself.
func ExtattrListLink(path string, attrnamespace int) ([]string, error) {
	return xattrs.ExtattrListLink(path, attrnamespace)
}
