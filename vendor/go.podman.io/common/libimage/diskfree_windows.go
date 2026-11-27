//go:build !remote && windows

package libimage

import "golang.org/x/sys/windows"

// diskFree returns the number of bytes available to the caller on the
// volume containing path.
func diskFree(path string) (int64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var freeToCaller uint64
	if err := windows.GetDiskFreeSpaceEx(p, &freeToCaller, nil, nil); err != nil {
		return 0, err
	}
	return int64(freeToCaller), nil
}
