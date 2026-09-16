package archive

import (
	"archive/tar"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"go.podman.io/storage/pkg/idtools"
	"go.podman.io/storage/pkg/system"
	"golang.org/x/sys/unix"
)

func getOverlayOpaqueXattrName() string {
	return GetOverlayXattrName("opaque")
}

func getWhiteoutConverter(format WhiteoutFormat, data any, options *TarOptions) tarWhiteoutConverter {
	if format == OverlayWhiteoutFormat {
		var roLayers []string = nil
		if rolayers, ok := data.([]string); ok && len(rolayers) > 0 {
			roLayers = rolayers
		}
		return overlayWhiteoutConverter{
			rolayers:               roLayers,
			runningInMinimalChroot: options != nil && options.InternalRunningInMinimalChroot,
		}
	}
	return nil
}

type overlayWhiteoutConverter struct {
	rolayers []string
	// runningInMinimalChroot indicates that we are confined to a fairly strict chroot,
	// so we don’t need to worry about Lgetxattr / Llistxattr escaping the source directory.
	//
	// We need this because:
	// - getxattrat() would be ideal, but requires Linux 6.13, and as of 2026-05 that might still be too new
	// - Our fallback is to open "/proc/self/fd/$fd" of an O_PATH file handle, but /proc is not available in these chroots.
	runningInMinimalChroot bool
}

func (o overlayWhiteoutConverter) ConvertWrite(hdr *tar.Header, path string, fi os.FileInfo) (*tar.Header, error) {
	return o.convertWriteWithGetxattr(hdr, fi, func(attrName string) ([]byte, error) {
		return system.Lgetxattr(path, attrName)
	})
}

func (o overlayWhiteoutConverter) convertWrite(hdr *tar.Header, root *os.Root, fsPath string, fi os.FileInfo) (*tar.Header, error) {
	if !o.runningInMinimalChroot {
		return o.convertWriteWithGetxattr(hdr, fi, func(attrName string) ([]byte, error) {
			return system.RootLgetxattr(root, fsPath, attrName)
		})
	} else {
		return o.convertWriteWithGetxattr(hdr, fi, func(attrName string) ([]byte, error) {
			return system.Lgetxattr(filepath.Join(root.Name(), filepath.FromSlash(fsPath)), attrName)
		})
	}
}

func (o overlayWhiteoutConverter) convertWriteWithGetxattr(hdr *tar.Header, fi os.FileInfo, getxattr func(attrName string) ([]byte, error)) (*tar.Header, error) {
	// convert whiteouts to AUFS format
	if fi.Mode()&os.ModeCharDevice != 0 && hdr.Devmajor == 0 && hdr.Devminor == 0 {
		// we just rename the file and make it normal
		dir, filename := filepath.Split(hdr.Name)
		hdr.Name = filepath.Join(dir, WhiteoutPrefix+filename)
		hdr.Mode = 0
		hdr.Typeflag = tar.TypeReg
		hdr.Size = 0
	}

	if fi.Mode()&os.ModeDir != 0 {
		// convert opaque dirs to AUFS format by writing an empty file with the whiteout prefix
		opaque, err := getxattr(getOverlayOpaqueXattrName())
		if err != nil {
			return nil, err
		}
		if len(opaque) == 1 && opaque[0] == 'y' {
			if hdr.PAXRecords != nil {
				delete(hdr.PAXRecords, PaxSchilyXattr+getOverlayOpaqueXattrName())
			}
			// If there are no lower layers, then it can't have been deleted in this layer.
			if len(o.rolayers) == 0 {
				return nil, nil //nolint: nilnil
			}
			// At this point, we have a directory that's opaque.  If it appears in one of the lower
			// layers, then it was newly-created here, so it wasn't also deleted here.
			for _, rolayer := range o.rolayers {
				// FIXME: This resolves symlinks for hdr.Name within rolayer; presumably paths with symlinks
				// don’t participate in whiteout lookups?!
				//
				// For now, just use securejoin/pathrs to restrict the paths to be within the layer.
				// (We can’t use os.Root because that one fails with an untyped error if it encounters a symlink to a parent to the root,
				// and those symlinks are valid inside containers.)
				// This should, almost certainly, instead do the checks for parent directories
				// in the parent->child order, and stop if it encounters a symlink.
				//
				// Also seriously look at deduplicating this with overlayDeletedFile.
				if done, ret, err := func() (bool, *tar.Header, error) { // A scope for defer
					layerRoot, err := os.Open(rolayer)
					if err != nil {
						return true, nil, err
					}
					defer layerRoot.Close()

					stat, statErr := pathrsStat(layerRoot, strings.TrimSuffix(hdr.Name, "/"))
					if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) && !errors.Is(statErr, syscall.ENOTDIR) {
						// Not sure what happened here.
						return true, nil, statErr
					}
					if statErr == nil {
						if stat.Mode()&os.ModeCharDevice != 0 {
							if isWhiteOut(stat) {
								return true, nil, nil //nolint: nilnil // FIXME: test coverage
							}
						}
						// It's not whiteout, so it was there in the older layer, so we need to
						// add a whiteout for this item in this layer.
						// create a header for the whiteout file
						// it should inherit some properties from the parent, but be a regular file
						wo := &tar.Header{
							Typeflag:   tar.TypeReg,
							Mode:       hdr.Mode & int64(os.ModePerm),
							Name:       filepath.Join(hdr.Name, WhiteoutOpaqueDir),
							Size:       0,
							Uid:        hdr.Uid,
							Uname:      hdr.Uname,
							Gid:        hdr.Gid,
							Gname:      hdr.Gname,
							AccessTime: hdr.AccessTime,
							ChangeTime: hdr.ChangeTime,
						}
						return true, wo, nil
					}
					for dir := filepath.Dir(hdr.Name); dir != "" && dir != "." && dir != string(os.PathSeparator); dir = filepath.Dir(dir) {
						// Check for whiteout for a parent directory in a parent layer.
						stat, statErr := pathrsStat(layerRoot, dir)
						if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) && !errors.Is(statErr, syscall.ENOTDIR) {
							// Not sure what happened here.
							return true, nil, statErr
						}
						if statErr == nil {
							if stat.Mode()&os.ModeCharDevice != 0 { // FIXME: test coverage
								// If it's whiteout for a parent directory, then the
								// original directory wasn't inherited into this layer,
								// so we don't need to emit whiteout for it.
								if isWhiteOut(stat) {
									return true, nil, nil //nolint: nilnil
								}
							}
						}
					}
					return false, nil, nil
				}(); done {
					return ret, err
				}
			}
		}
	}

	return nil, nil
}

func (overlayWhiteoutConverter) ConvertReadWithHandler(hdr *tar.Header, path string, handler TarWhiteoutHandler) (bool, error) {
	// ConvertReadWithHandler is only allowed to create files within parent(path) (whatever
	// that resolves to), and must ensure it does not follow symlinks when creating files
	// within that directory.

	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// if a directory is marked as opaque by the AUFS special file, we need to translate that to overlay
	if base == WhiteoutOpaqueDir {
		// Note that this follows symlinks: it’s up to the caller to ensure "dir" == parent(path)
		// is acceptable.
		err := handler.Setxattr(dir, getOverlayOpaqueXattrName(), []byte{'y'})
		// don't write the file itself
		return false, err
	}

	// if a file was deleted and we are using overlay, we need to create a character device
	if originalBase, ok := strings.CutPrefix(base, WhiteoutPrefix); ok {
		if isInvalidWhiteoutTargetBaseName(originalBase) {
			return false, fmt.Errorf("invalid whiteout path %q", path)
		}
		originalPath := filepath.Join(dir, originalBase)

		// Mknod fails with EEXIST if the target is a symlink, so this should be safe.
		if err := handler.Mknod(originalPath, unix.S_IFCHR, 0); err != nil {
			// If someone does:
			//     rm -rf /foo/bar
			// in an image, some tools will generate a layer with:
			//     /.wh.foo
			//     /foo/.wh.bar
			// and when doing the second mknod(), we will fail with
			// ENOTDIR, since the previous /foo was mknod()'d as a
			// character device node and not a directory.
			if isENOTDIR(err) {
				return false, nil
			}
			return false, err
		}
		if err := handler.Chown(originalPath, hdr.Uid, hdr.Gid); err != nil {
			return false, err
		}

		// don't write the file itself
		return false, nil
	}

	return true, nil
}

type directHandler struct{}

func (d directHandler) Setxattr(path, name string, value []byte) error {
	if err := unix.Setxattr(path, name, value, 0); err != nil {
		return &os.PathError{Op: "setxattr", Path: path, Err: err}
	}
	return nil
}

func (d directHandler) Mknod(path string, mode uint32, dev int) error {
	if err := unix.Mknod(path, mode, dev); err != nil {
		return &os.PathError{Op: "mknod", Path: path, Err: err}
	}
	return nil
}

func (d directHandler) Chown(path string, uid, gid int) error {
	return idtools.SafeChown(path, uid, gid)
}

func (o overlayWhiteoutConverter) ConvertRead(hdr *tar.Header, path string) (bool, error) {
	// ConvertRead is only allowed to create files within parent(path) and
	// must ensure it does not follow symlinks when creating them.

	var handler directHandler
	return o.ConvertReadWithHandler(hdr, path, handler)
}

func isWhiteOut(stat os.FileInfo) bool {
	s := stat.Sys().(*syscall.Stat_t)
	return major(uint64(s.Rdev)) == 0 && minor(uint64(s.Rdev)) == 0 //nolint:unconvert
}

func GetFileOwner(path string) (uint32, uint32, uint32, error) {
	f, err := os.Stat(path)
	if err != nil {
		return 0, 0, 0, err
	}
	s, ok := f.Sys().(*syscall.Stat_t)
	if ok {
		return s.Uid, s.Gid, s.Mode & 0o7777, nil
	}
	return 0, 0, uint32(f.Mode()), nil
}

func handleLChmod(hdr *tar.Header, path string, hardlinkTargetPath string, hdrInfo os.FileInfo, forceMask *os.FileMode) error {
	permissionsMask := hdrInfo.Mode()
	if forceMask != nil {
		permissionsMask = *forceMask
	}

	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(hardlinkTargetPath); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			if err := os.Chmod(path, permissionsMask); err != nil {
				return err
			}
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		if err := os.Chmod(path, permissionsMask); err != nil {
			return err
		}
	}
	return nil
}
