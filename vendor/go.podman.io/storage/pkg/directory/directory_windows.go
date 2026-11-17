//go:build windows

package directory

import (
	"errors"
	"io/fs"
	"os"
)

// Size walks a directory tree and returns its total size in bytes
func Size(dir string) (size int64, err error) {
	usage, err := Usage(dir)
	if err != nil {
		return 0, nil
	}
	return usage.Size, nil
}

// Usage walks a directory tree and returns its total size in bytes and the number of inodes.
func Usage(dir string) (*DiskUsage, error) {
	// WARNING: This is called in contexts where the contents of dir may be maliciously
	// concurrently modified.

	// os.OpenRoot requires the root to be a directory, and fails with an untyped error otherwise.
	// It also follows symlinks.
	fileInfo, err := os.Lstat(dir)
	if err == nil && !fileInfo.IsDir() {
		return &DiskUsage{
			Size:       fileInfo.Size(),
			InodeCount: 1,
		}, nil
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	usage := &DiskUsage{}
	err = fs.WalkDir(root.FS(), ".", func(fsPath string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}

		usage.InodeCount++

		// Ignore directory sizes
		if d.IsDir() {
			return nil
		}

		fileInfo, err := d.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		usage.Size += fileInfo.Size()

		return nil
	})
	return usage, err
}
