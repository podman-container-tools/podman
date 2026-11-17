//go:build windows

package wsl

import (
	"io/fs"
	"math"
	"os"

	"golang.org/x/sys/windows"
)

type fileLock struct {
	file *os.File
}

// Locks a file path, creating or overwriting a file if necessary. This API only
// supports dedicated empty lock files. Locking is not advisory, once a file is
// locked, additional opens will block on read/write.
func lockFile(path string) (*fileLock, error) {
	// In the future we may want to switch this to an async open vs the win32 API
	// to bring support for timeouts, so we don't export the current underlying
	// File object.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0o644)
	if err != nil {
		return nil, &fs.PathError{
			Op:   "lock",
			Path: path,
			Err:  err,
		}
	}

	const max = uint32(math.MaxUint32)
	overlapped := new(windows.Overlapped)
	lockType := windows.LOCKFILE_EXCLUSIVE_LOCK
	// Lock largest possible length (all 64 bits (lo + hi) set)
	err = windows.LockFileEx(windows.Handle(file.Fd()), uint32(lockType), 0, max, max, overlapped)
	if err != nil {
		file.Close()
		return nil, &fs.PathError{
			Op:   "lock",
			Path: file.Name(),
			Err:  err,
		}
	}

	return &fileLock{file: file}, nil
}

func (flock *fileLock) unlock() error {
	if flock == nil || flock.file == nil {
		return nil
	}

	defer func() {
		flock.file.Close()
		flock.file = nil
	}()

	const max = uint32(math.MaxUint32)
	overlapped := new(windows.Overlapped)
	err := windows.UnlockFileEx(windows.Handle(flock.file.Fd()), 0, max, max, overlapped)
	if err != nil {
		return &fs.PathError{
			Op:   "unlock",
			Path: flock.file.Name(),
			Err:  err,
		}
	}

	return nil
}
