package define

import (
	"os"

	"golang.org/x/sys/windows"
)

// openShareDelete opens path for reading with FILE_SHARE_DELETE, so that
// ioutils.AtomicWriteFile can replace the file while it is open. Golang
// os.Open doesn't set FILE_SHARE_DELETE and a concurrent
// ioutils.AtomicWriteFile fails with a permission denied error.
// https://github.com/podman-container-tools/podman/issues/28873
func openShareDelete(path string) (*os.File, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	h, err := windows.CreateFile(p,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}
