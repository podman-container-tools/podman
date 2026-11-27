//go:build !linux

package protectedroot

import "os"

func (pr PathRoot) openFileRacy(name string, flag int, perm os.FileMode) (*os.File, error) {
	joined, err := pr.Join(name)
	if err != nil {
		return nil, err
	}

	return os.OpenFile(joined, flag, perm)
}

func (pr PathRoot) openRacy(name string) (*os.File, error) {
	joined, err := pr.Join(name)
	if err != nil {
		return nil, err
	}

	return os.Open(joined)
}
