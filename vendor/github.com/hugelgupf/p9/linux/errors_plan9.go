package linux

import (
	"errors"
	"io/fs"
)

func sysErrno(err error) Errno {
	for _, pair := range []struct {
		error
		Errno
	}{
		{fs.ErrNotExist, ENOENT},
		{fs.ErrPermission, EACCES},
		{fs.ErrExist, EEXIST},
	} {
		if errors.Is(err, pair.error) {
			return pair.Errno
		}
	}
	// No clue what to do with others.
	return 0
}
