//go:build linux

package driver

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// layerFiles gives access to the files of one layer through the layer's
// diff directory.
type layerFiles struct {
	diffDir *os.File
	// flatNames maps a tar entry name to the name its content is stored
	// under, for a layer stored in the flat layout composefs uses.  nil
	// for a layer that stores files under their own names.
	flatNames map[string]string
}

// NewLayerFiles delegates access to a layer through dir, taking ownership of
// the open directory. flatNames maps tar entry names to their stored paths,
// or is nil when files are stored under their original names.
func NewLayerFiles(dir *os.File, flatNames map[string]string) LayerFiles {
	return &layerFiles{diffDir: dir, flatNames: flatNames}
}

// DirFDs implements LayerFiles.
func (f *layerFiles) DirFDs() []*os.File {
	return []*os.File{f.diffDir}
}

// Close implements LayerFiles.
func (f *layerFiles) Close() error {
	if f.diffDir == nil {
		return nil
	}
	err := f.diffDir.Close()
	f.diffDir = nil
	return err
}

// Lookup implements LayerFiles.  The diff directory is the only directory
// handed to the consumer, so the index is always 0.
func (f *layerFiles) Lookup(name string, size int64, onlyWorldReadable bool) (int, string, bool, error) {
	var openPath string
	if f.flatNames != nil {
		flat, ok := f.flatNames[filepath.Clean(name)]
		if !ok {
			return 0, "", false, nil
		}
		openPath = flat
	} else {
		// Resolve name as if the diff directory were the root, as
		// RESOLVE_IN_ROOT would: tar entry names can be absolute, or go
		// above the root, in an image converted from another format.  The
		// consumer resolves the name it is given beneath the directory,
		// so it has to be relative and without "..", and symlinks are
		// refused below, so resolving it lexically gives the same result.
		openPath = strings.TrimPrefix(filepath.Clean("/"+name), "/")
		if openPath == "" {
			return 0, "", false, nil
		}
	}

	fd, err := unix.Openat2(int(f.diffDir.Fd()), openPath, &unix.OpenHow{
		Flags:   unix.O_CLOEXEC | unix.O_PATH,
		Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_BENEATH,
	})
	if err != nil {
		// EXDEV: a flat name that does not stay beneath the diff
		// directory.
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ELOOP) || errors.Is(err, unix.EXDEV) {
			return 0, "", false, nil
		}
		return 0, "", false, fmt.Errorf("opening %q: %w", name, err)
	}
	var st unix.Stat_t
	statErr := unix.Fstat(fd, &st)
	unix.Close(fd)
	if statErr != nil {
		return 0, "", false, fmt.Errorf("stat of %q: %w", name, statErr)
	}

	if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Size != size {
		return 0, "", false, nil
	}
	if onlyWorldReadable {
		if st.Mode&unix.S_IROTH == 0 {
			return 0, "", false, nil
		}
		// The consumer opens the file relative to diffDir, so every
		// parent up to and including diffDir must be searchable by everyone.
		for parent := filepath.Dir(openPath); ; parent = filepath.Dir(parent) {
			if parent == "." {
				statErr = unix.Fstat(int(f.diffDir.Fd()), &st)
			} else {
				// Any symlink in the path is rejected, and the file sent inline.
				parentFD, err := unix.Openat2(int(f.diffDir.Fd()), parent, &unix.OpenHow{
					Flags:   unix.O_CLOEXEC | unix.O_PATH | unix.O_DIRECTORY,
					Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_BENEATH,
				})
				if err != nil {
					if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ELOOP) || errors.Is(err, unix.EXDEV) || errors.Is(err, unix.ENOTDIR) {
						return 0, "", false, nil
					}
					return 0, "", false, fmt.Errorf("opening parent %q of %q: %w", parent, name, err)
				}
				statErr = unix.Fstat(parentFD, &st)
				unix.Close(parentFD)
			}
			if statErr != nil {
				return 0, "", false, fmt.Errorf("stat of parent %q of %q: %w", parent, name, statErr)
			}
			if st.Mode&unix.S_IXOTH == 0 {
				return 0, "", false, nil
			}
			if parent == "." {
				break
			}
		}
	}
	return 0, openPath, true, nil
}
