//go:build linux

package protectedroot

import (
	"os"

	"golang.org/x/sys/unix"
)

func (pr PathRoot) OpenFileConcurrent(name string, flag int, perm os.FileMode) (*os.File, error) {
	pr.ensureInitialized()

	root, err := os.Open(pr.path)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	fd, err := unix.Openat2(int(root.Fd()), name, &unix.OpenHow{
		Flags:   uint64(flag) | unix.O_CLOEXEC,
		Mode:    uint64(perm),
		Resolve: unix.RESOLVE_IN_ROOT | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, &os.PathError{Op: "openat2", Path: name, Err: err}
	}

	return os.NewFile(uintptr(fd), name), nil
}

func (pr PathRoot) openFileRacy(name string, flag int, perm os.FileMode) (*os.File, error) {
	return pr.OpenFileConcurrent(name, flag, perm)
}

func (pr PathRoot) OpenConcurrent(name string) (*os.File, error) {
	return pr.OpenFileConcurrent(name, os.O_RDONLY, 0)
}

func (pr PathRoot) openRacy(name string) (*os.File, error) {
	return pr.OpenConcurrent(name)
}
