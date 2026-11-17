//go:build unix && !openbsd && !solaris

package xattr

import (
	"io/fs"
	"strings"

	"golang.org/x/sys/unix"
)

func List(p string) ([]string, error) {
	sz, err := unix.Listxattr(p, nil)
	if err != nil {
		return nil, &fs.PathError{Op: "listxattr-get-size", Path: p, Err: err}
	}

	b := make([]byte, sz)
	sz, err = unix.Listxattr(p, b)
	if err != nil {
		return nil, &fs.PathError{Op: "listxattr", Path: p, Err: err}
	}

	return strings.Split(strings.Trim(string(b[:sz]), "\000"), "\000"), nil
}

func Get(p string, attr string) ([]byte, error) {
	sz, err := unix.Getxattr(p, attr, nil)
	if err != nil {
		return nil, &fs.PathError{Op: "getxattr-get-size", Path: p, Err: err}
	}

	b := make([]byte, sz)
	sz, err = unix.Getxattr(p, attr, b)
	if err != nil {
		return nil, &fs.PathError{Op: "getxattr", Path: p, Err: err}
	}
	return b[:sz], nil
}
