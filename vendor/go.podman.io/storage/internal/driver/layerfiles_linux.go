//go:build linux

package driver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.podman.io/storage/internal/createpath"
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

// storedPath returns the path of the file holding the content of the tar
// entry called name, relative to the diff directory, or false if there is
// none.
func (f *layerFiles) storedPath(name string) (string, bool) {
	if f.flatNames != nil {
		flat, ok := f.flatNames[filepath.Clean(name)]
		return flat, ok
	}
	openPath := strings.TrimPrefix(createpath.CleanAbsPath(name), "/")
	return openPath, openPath != ""
}

// Open implements LayerFiles.
func (f *layerFiles) Open(name string) (io.ReadCloser, error) {
	openPath, ok := f.storedPath(name)
	if !ok {
		return nil, fmt.Errorf("%q: %w", name, os.ErrNotExist)
	}
	fd, err := unix.Openat2(int(f.diffDir.Fd()), openPath, &unix.OpenHow{
		Flags:   unix.O_CLOEXEC | unix.O_RDONLY,
		Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_BENEATH,
	})
	if err != nil {
		return nil, fmt.Errorf("opening %q: %w", name, err)
	}
	return os.NewFile(uintptr(fd), openPath), nil
}

// Lookup implements LayerFiles.  The diff directory is the only directory
// handed to the consumer, so the index is always 0.
func (f *layerFiles) Lookup(name string, size int64, onlyWorldReadable bool) (int, string, bool, error) {
	openPath, ok := f.storedPath(name)
	if !ok {
		return 0, "", false, nil
	}

	fd, err := unix.Openat2(int(f.diffDir.Fd()), openPath, &unix.OpenHow{
		Flags:   unix.O_CLOEXEC | unix.O_PATH,
		Resolve: unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_BENEATH,
	})
	if err != nil {
		if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ELOOP) {
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
					if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ELOOP) || errors.Is(err, unix.ENOTDIR) {
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
