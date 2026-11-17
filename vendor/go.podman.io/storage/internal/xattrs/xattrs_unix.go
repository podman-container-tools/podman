//go:build linux || darwin || freebsd

package xattrs

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// XattrHandle allows efficient llistxattr / llistxattr operations on a single file.
type Handle struct {
	fd int

	// Only for error reporting
	errorRoot *os.Root
	errorPath string // Within errorRoot
}

func (h *Handle) Close() error {
	return unix.Close(h.fd)
}

func (h *Handle) pathInError() string {
	return filepath.Join(h.errorRoot.Name(), h.errorPath)
}

// syscallConnControl calls fn with the file descriptor of fd,
// simplifying the boilerplate of f.SyscallConn().Control().
func syscallConnControl[T any](fd *os.File, fn func(uintptr) (T, error)) (T, error) {
	var zeroRes T
	conn, err := fd.SyscallConn()
	if err != nil {
		return zeroRes, err
	}
	var res T
	var resErr error
	if err := conn.Control(func(fd uintptr) {
		res, resErr = fn(fd)
	}); err != nil {
		return zeroRes, err
	}
	if resErr != nil {
		return zeroRes, resErr
	}
	return res, nil
}
