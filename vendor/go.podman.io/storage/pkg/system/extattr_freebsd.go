//go:build freebsd

package system

import (
	"os"
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	EXTATTR_NAMESPACE_EMPTY  = unix.EXTATTR_NAMESPACE_EMPTY
	EXTATTR_NAMESPACE_USER   = unix.EXTATTR_NAMESPACE_USER
	EXTATTR_NAMESPACE_SYSTEM = unix.EXTATTR_NAMESPACE_SYSTEM
)

// extattrGet is the logic underlying ExtattrGetLink and extattrGetFd.
// Returns a []byte slice if the extattr is set and nil otherwise.
func extattrGet(syscallName string, pathInError string, getSyscall func(dest uintptr, nbytes int) (int, error)) ([]byte, error) {
	size, errno := getSyscall(uintptr(unsafe.Pointer(nil)), 0)
	if errno != nil {
		if errno == unix.ENOATTR {
			return nil, nil
		}
		return nil, &os.PathError{Op: syscallName, Path: pathInError, Err: errno}
	}
	if size == 0 {
		return []byte{}, nil
	}

	dest := make([]byte, size)
	size, errno = getSyscall(uintptr(unsafe.Pointer(&dest[0])), size)
	if errno != nil {
		return nil, &os.PathError{Op: syscallName, Path: pathInError, Err: errno}
	}

	return dest[:size], nil
}

// ExtattrGetLink retrieves the value of the extended attribute identified by attrname
// in the given namespace and associated with the given path in the file system.
// If the path is a symbolic link, the extended attribute is retrieved from the link itself.
// Returns a []byte slice if the extattr is set and nil otherwise.
func ExtattrGetLink(path string, attrnamespace int, attrname string) ([]byte, error) {
	return extattrGet("extattr_get_link", path, func(dest uintptr, nbytes int) (int, error) {
		return unix.ExtattrGetLink(path, attrnamespace, attrname, dest, nbytes)
	})
}

// extattrGetFd retrieves the value of the extended attribute identified by attrname
// in the given namespace and associated with the given file descriptor.
// Returns a []byte slice if the extattr is set and nil otherwise.
func extattrGetFd(fd int, attrnamespace int, attrname string) ([]byte, error) {
	return extattrGet("extattr_get_fd", strconv.Itoa(fd), func(dest uintptr, nbytes int) (int, error) {
		return unix.ExtattrGetFd(fd, attrnamespace, attrname, dest, nbytes)
	})
}

// ExtattrSetLink sets the value of extended attribute identified by attrname
// in the given namespace and associated with the given path in the file system.
// If the path is a symbolic link, the extended attribute is set on the link itself.
func ExtattrSetLink(path string, attrnamespace int, attrname string, data []byte) error {
	if len(data) == 0 {
		data = []byte{} // ensure non-nil for empty data
	}
	if _, errno := unix.ExtattrSetLink(path, attrnamespace, attrname,
		uintptr(unsafe.Pointer(&data[0])), len(data)); errno != nil {
		return &os.PathError{Op: "extattr_set_link", Path: path, Err: errno}
	}

	return nil
}

// extattrList is the logic underlying ExtattrListLink and extattrListFd.
func extattrList(syscallName string, pathInError string, listSyscall func(dest uintptr, nbytes int) (int, error)) ([]string, error) {
	size, errno := listSyscall(uintptr(unsafe.Pointer(nil)), 0)
	if errno != nil {
		return nil, &os.PathError{Op: syscallName, Path: pathInError, Err: errno}
	}
	if size == 0 {
		return []string{}, nil
	}

	dest := make([]byte, size)
	size, errno = listSyscall(uintptr(unsafe.Pointer(&dest[0])), size)
	if errno != nil {
		return nil, &os.PathError{Op: syscallName, Path: pathInError, Err: errno}
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

// ExtattrListLink lists extended attributes associated with the given path
// in the specified namespace. If the path is a symbolic link, the attributes
// are listed from the link itself.
func ExtattrListLink(path string, attrnamespace int) ([]string, error) {
	return extattrList("extattr_list_link", path, func(dest uintptr, nbytes int) (int, error) {
		return unix.ExtattrListLink(path, attrnamespace, dest, nbytes)
	})
}

// extattrListFd lists extended attributes associated with fd
// in the specified namespace.
func extattrListFd(fd int, attrnamespace int) ([]string, error) {
	return extattrList("extattr_list_fd", strconv.Itoa(fd), func(dest uintptr, nbytes int) (int, error) {
		return unix.ExtattrListFd(fd, attrnamespace, dest, nbytes)
	})
}
