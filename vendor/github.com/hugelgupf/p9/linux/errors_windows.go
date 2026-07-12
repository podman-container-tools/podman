//go:build windows

package linux

import (
	"errors"
	"syscall"
)

func sysErrno(err error) Errno {
	for _, pair := range []struct {
		error
		Errno
	}{
		{syscall.ERROR_FILE_NOT_FOUND, ENOENT},
		{syscall.ERROR_PATH_NOT_FOUND, ENOENT},
		{syscall.ERROR_ACCESS_DENIED, EACCES},
		{syscall.ERROR_FILE_EXISTS, EEXIST},
		{syscall.ERROR_INSUFFICIENT_BUFFER, ENOMEM},
	} {
		if errors.Is(err, pair.error) {
			return pair.Errno
		}
	}
	// No clue what to do with others.
	return 0
}
