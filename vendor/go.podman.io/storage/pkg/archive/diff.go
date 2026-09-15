package archive

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/sirupsen/logrus"
	"go.podman.io/storage/internal/createpath"
	"go.podman.io/storage/pkg/fileutils"
	"go.podman.io/storage/pkg/idtools"
	"go.podman.io/storage/pkg/pools"
	"go.podman.io/storage/pkg/system"
)

// UnpackLayer unpack `layer` to a `dest`. The stream `layer` can be
// compressed or uncompressed.
// Returns the size in bytes of the contents of the layer.
func UnpackLayer(dest string, layer io.Reader, options *TarOptions) (size int64, err error) {
	tr := tar.NewReader(layer)
	trBuf := pools.BufioReader32KPool.Get(tr)
	defer pools.BufioReader32KPool.Put(trBuf)

	var dirs []*tar.Header
	unpackedPaths := make(map[string]struct{})

	if options == nil {
		options = &TarOptions{}
	}
	idMappings := idtools.NewIDMappingsFromMaps(options.UIDMaps, options.GIDMaps)

	aufsTempdir := ""
	aufsHardlinks := make(map[string]*tar.Header)
	buffer := make([]byte, 1<<20)

	// This is required because path = securejoin.SecureJoin(dest, ...) implicitly Clean()s
	// the result, and we later use a (path == dest) comparison.
	// Alternatively, we could compute filepath.Rel(dest, path) == ".", but that would
	// be more expensive (filepath.Rel starts with two Clean calls).
	dest = filepath.Clean(dest)
	// Iterate through the files in the archive.
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return 0, err
		}

		size += hdr.Size

		// Normalize name. This does NOT allow us to infer useful security properties
		// (does this escape "dest"?) from the string syntax, because the path may include
		// arbitrary (possibly escaping) symlinks.
		//
		// We continue to do this primarily to preserve the semantics of detecting whiteouts.
		// DO NOT add any more code that makes it user-visible whether we Clean hdr.Name;
		// almost all filesystem operations should instead use "path" below.
		hdr.Name = filepath.Clean(hdr.Name)

		// Windows does not support filenames with colons in them. Ignore
		// these files. This is not a problem though (although it might
		// appear that it is). Let's suppose a client is running docker pull.
		// The daemon it points to is Windows. Would it make sense for the
		// client to be doing a docker pull Ubuntu for example (which has files
		// with colons in the name under /usr/share/man/man3)? No, absolutely
		// not as it would really only make sense that they were pulling a
		// Windows image. However, for development, it is necessary to be able
		// to pull Linux images which are in the repository.
		//
		// TODO Windows. Once the registry is aware of what images are Windows-
		// specific or Linux-specific, this warning should be changed to an error
		// to cater for the situation where someone does manage to upload a Linux
		// image but have it tagged as Windows inadvertently.
		if runtime.GOOS == windows {
			if strings.Contains(hdr.Name, ":") {
				logrus.Warnf("Windows: Ignoring %s (is this a Linux image?)", hdr.Name)
				continue
			}
		}

		// This check is INSUFFICIENT to prevent breakouts if hdr.Name contains symlink parents;
		// we preserve it only to keep the existing restrictions on acceptable inputs.
		insecureRel, err := filepath.Rel(dest, filepath.Join(dest, hdr.Name))
		if err != nil {
			return 0, err
		}
		if insecureRel == ".." || strings.HasPrefix(insecureRel, ".."+string(os.PathSeparator)) {
			return 0, breakoutError(fmt.Errorf("%q is outside of %q", hdr.Name, dest))
		}

		// This does not detect attempts to break out, it just silently restricts them to dest.
		// We could use os.Root to create files instead — that fails on breakout attempts, but
		// Go does not include all operations we need as of Go 1.25, so that would be a larger
		// change — and as of Go 1.26 (which does not use openat2 and the like yet) it would
		// ultimately be more expensive, we would be repeatedly getting a handle to hdrDir in order
		// to make *at syscalls.
		hdrDir, hdrBase, err := createpath.SplitPath(hdr.Name)
		if err != nil {
			return 0, err
		}
		parentPath, err := securejoin.SecureJoin(dest, hdrDir)
		if err != nil {
			return 0, err
		}
		path := filepath.Join(parentPath, hdrBase) // Warning: this can refer to an existing (and escaping) symlink

		if path == dest {
			// The caller has probably pre-created dest as a directory; we don’t know for sure, and it doesn’t really matter
			// because SecureJoin works fine enough for non-existent paths, and because the "Not the root directory"
			// code path below would create dest if necessary.
			//
			// The one thing we MUST NOT allow is creating "dest" as a symbolic link, because SecureJoin’s operation implicitly
			// resolves that symlink before constraining the returned path.  We also must not allow replacing an existing directory
			// with a symbolic link.
			//
			// Just refuse all non-directory paths here.
			if hdr.Typeflag != tar.TypeDir {
				return 0, fmt.Errorf("refusing to act on a non-directory entry as the archive root")
			}
		} else {
			// Not the root directory, ensure that the parent directory exists.
			// This happened in some tests where an image had a tarfile without any
			// parent directories.
			if err := fileutils.Lexists(parentPath); err != nil && os.IsNotExist(err) {
				err = os.MkdirAll(parentPath, 0o755)
				if err != nil {
					return 0, err
				}
			}
		}

		// Skip AUFS metadata dirs
		if strings.HasPrefix(hdr.Name, WhiteoutMetaPrefix) {
			// Regular files inside /.wh..wh.plnk can be used as hardlink targets
			// We don't want this directory, but we need the files in them so that
			// such hardlinks can be resolved.
			if strings.HasPrefix(hdr.Name, WhiteoutLinkDir) && hdr.Typeflag == tar.TypeReg {
				// FIXME: We have already created a dest/WhiteoutMetaPrefix “parent directory”.

				// filepath.Base(hdr.Name) should be safe _if_ we set hdr.Name to filepath.Clean(hdr.Name),
				// hdr.Clean() would interpret a trailing /. or /.. by modifying the whole path,
				// leaving ".." only if there were no proper path components left — and the strings.HasPrefix
				// checks above ensure that’s not the case.
				//
				// But we don’t want to rely on that filepath.Clean().
				//
				// We could strictly enforce the path to be WhiteoutLinkDir/$basename (is that the
				// right format???); to minimize changes, and to keep the same style of handling paths, instead:
				_, basename, err := createpath.SplitPath(hdr.Name)
				if err != nil {
					return 0, err
				}
				if basename == "." { // Per the above, this should not be reachable, but account for that return value for completeness.
					return 0, fmt.Errorf("invalid AUFS plink path %q", hdr.Name)
				}
				aufsHardlinks[basename] = hdr
				if aufsTempdir == "" {
					if aufsTempdir, err = os.MkdirTemp("", "storageplnk"); err != nil {
						return 0, err
					}
					defer os.RemoveAll(aufsTempdir)
				}
				linkContentsPath := filepath.Join(aufsTempdir, basename)
				// Ensure the destination does not exist, as extractTarFileEntry expects.
				if err := os.Remove(linkContentsPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return 0, fmt.Errorf("removing duplicate whiteout-link entry %q: %w", linkContentsPath, err)
				}
				if err := extractTarFileEntry(linkContentsPath, dest, hdr, tr, true, nil, options.InUserNS, options.IgnoreChownErrors, options.ForceMask, buffer); err != nil {
					return 0, err
				}
			}

			if hdr.Name != WhiteoutOpaqueDir {
				continue
			}
		}

		if strings.HasPrefix(hdrBase, WhiteoutPrefix) {
			if hdrBase == WhiteoutOpaqueDir {
				err := fileutils.Lexists(parentPath)
				if err != nil {
					return 0, err
				}
				err = filepath.WalkDir(parentPath, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						if os.IsNotExist(err) {
							err = nil // parent was deleted
						}
						return err
					}
					if path == parentPath {
						return nil
					}
					if _, exists := unpackedPaths[path]; !exists {
						if err := resetImmutable(path, nil); err != nil {
							return err
						}
						err := os.RemoveAll(path)
						return err
					}
					return nil
				})
				if err != nil {
					return 0, err
				}
			} else {
				originalBase := hdrBase[len(WhiteoutPrefix):]
				if isInvalidWhiteoutTargetBaseName(originalBase) {
					return 0, fmt.Errorf("invalid whiteout path %q", hdr.Name)
				}
				originalPath := filepath.Join(parentPath, originalBase) // Warning: this can refer to an existing (and escaping) symlink
				if err := resetImmutable(originalPath, nil); err != nil {
					return 0, err
				}
				if err := os.RemoveAll(originalPath); err != nil {
					return 0, err
				}
			}
		} else {
			// If path exits we almost always just want to remove and replace it.
			// The only exception is when it is a directory *and* the file from
			// the layer is also a directory. Then we want to merge them (i.e.
			// just apply the metadata from the layer).
			// (Above, we have already refused to replace all of dest with a non-directory.)
			//
			// We always reset the immutable flag (if present) to allow metadata
			// changes and to allow directory modification. The flag will be
			// re-applied based on the contents of hdr either at the end for
			// directories or in extractTarFileEntry otherwise.
			if fi, err := os.Lstat(path); err == nil {
				if err := resetImmutable(path, &fi); err != nil {
					return 0, err
				}
				if !fi.IsDir() || hdr.Typeflag != tar.TypeDir {
					if err := os.RemoveAll(path); err != nil {
						return 0, err
					}
				}
			}

			trBuf.Reset(tr)
			srcData := io.Reader(trBuf)
			srcHdr := hdr

			// Hard links into /.wh..wh.plnk don't work, as we don't extract that directory, so
			// we manually retarget these into the temporary files we extracted them into
			if hdr.Typeflag == tar.TypeLink && strings.HasPrefix(filepath.Clean(hdr.Linkname), WhiteoutLinkDir) {
				linkBasename := filepath.Base(hdr.Linkname)
				// We have done more checking of the basename before setting up the aufsHardlinks
				// entry, so if we find one, it’s safe to refer to aufsTempdir/linkBasename.
				srcHdr = aufsHardlinks[linkBasename]
				if srcHdr == nil {
					return 0, fmt.Errorf("invalid aufs hardlink")
				}
				// FIXME: We _copy_ the contents of the hardlink, we don’t make a hardlink?!
				tmpFile, err := os.Open(filepath.Join(aufsTempdir, linkBasename))
				if err != nil {
					return 0, err
				}
				defer tmpFile.Close()
				srcData = tmpFile
			}

			if err := remapIDs(nil, idMappings, options.ChownOpts, srcHdr); err != nil {
				return 0, err
			}

			if err := extractTarFileEntry(path, dest, srcHdr, srcData, true, nil, options.InUserNS, options.IgnoreChownErrors, options.ForceMask, buffer); err != nil {
				return 0, err
			}

			// Directory mtimes must be handled at the end to avoid further
			// file creation in them to modify the directory mtime
			if hdr.Typeflag == tar.TypeDir {
				dirs = append(dirs, hdr)
			}
			unpackedPaths[path] = struct{}{}
		}
	}

	for _, hdr := range dirs {
		// We did create a directory at hdr.Name, but later entries in the tar archive
		// could have replaced hdr.Name or any of its parents with a different file / file kind.
		// So we don’t actually know that hdr.Name refers to a directory; in particular it might
		// be a (possibly escaping) symlink.
		hdrDir, hdrBase, err := createpath.SplitPath(hdr.Name)
		if err != nil {
			return 0, err
		}
		parentPath, err := securejoin.SecureJoin(dest, hdrDir)
		if err != nil {
			return 0, err
		}
		path := filepath.Join(parentPath, hdrBase)

		fi, err := os.Lstat(path)
		if err != nil {
			return 0, err
		}
		if !fi.IsDir() {
			continue // The directory was replaced; whatever happened here, hdr is no longer relevant.
		}
		if err := system.Chtimes(path, hdr.AccessTime, hdr.ModTime); err != nil { // Note: follows symlinks
			return 0, err
		}
		if err := WriteFileFlagsFromTarHeader(path, hdr); err != nil {
			return 0, err
		}
	}

	return size, nil
}

// ApplyLayer parses a diff in the standard layer format from `layer`,
// and applies it to the directory `dest`. The stream `layer` can be
// compressed or uncompressed.
// Returns the size in bytes of the contents of the layer.
func ApplyLayer(dest string, layer io.Reader) (int64, error) {
	return applyLayerHandler(dest, layer, &TarOptions{}, true)
}

// ApplyUncompressedLayer parses a diff in the standard layer format from
// `layer`, and applies it to the directory `dest`. The stream `layer`
// can only be uncompressed.
// Returns the size in bytes of the contents of the layer.
func ApplyUncompressedLayer(dest string, layer io.Reader, options *TarOptions) (int64, error) {
	return applyLayerHandler(dest, layer, options, false)
}

// do the bulk load of ApplyLayer, but allow for not calling DecompressStream
func applyLayerHandler(dest string, layer io.Reader, options *TarOptions, decompress bool) (int64, error) {
	dest = filepath.Clean(dest)

	// We need to be able to set any perms
	oldmask, err := system.Umask(0)
	if err != nil {
		return 0, err
	}
	defer func() {
		_, _ = system.Umask(oldmask) // Ignore err. This can only fail with ErrNotSupportedPlatform, in which case we would have failed above.
	}()

	if decompress {
		layer, err = DecompressStream(layer)
		if err != nil {
			return 0, err
		}
	}
	return UnpackLayer(dest, layer, options)
}
