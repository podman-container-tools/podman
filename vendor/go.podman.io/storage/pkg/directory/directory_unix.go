//go:build !windows

package directory

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// Size walks a directory tree and returns its total size in bytes.
func Size(dir string) (size int64, err error) {
	usage, err := Usage(dir)
	if err != nil {
		return 0, err
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
	data := make(map[uint64]struct{})
	err = fs.WalkDir(root.FS(), ".", func(fsPath string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}

		fileInfo, err := entry.Info()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}

		// Check inode to only count the sizes of files with multiple hard links once.
		inode := fileInfo.Sys().(*syscall.Stat_t).Ino
		if _, exists := data[inode]; exists {
			return nil
		}

		data[inode] = struct{}{}
		// Ignore directory sizes
		if entry.IsDir() {
			return nil
		}

		usage.Size += fileInfo.Size()

		return nil
	})
	// inode count is the number of unique inode numbers we saw
	usage.InodeCount = int64(len(data))
	return usage, err
}
