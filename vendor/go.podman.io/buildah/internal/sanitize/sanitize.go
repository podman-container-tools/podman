package sanitize

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	directoryTransport "go.podman.io/image/v5/directory"
	dockerTransport "go.podman.io/image/v5/docker"
	dockerArchiveTransport "go.podman.io/image/v5/docker/archive"
	ociArchiveTransport "go.podman.io/image/v5/oci/archive"
	ociLayoutTransport "go.podman.io/image/v5/oci/layout"
	openshiftTransport "go.podman.io/image/v5/openshift"
	"go.podman.io/image/v5/pkg/compression"
	"go.podman.io/storage/pkg/archive"
	"go.podman.io/storage/pkg/chrootarchive"
)

// create a temporary file to use as a destination archive
func newArchiveDestination(tmpdir string) (tw *tar.Writer, f *os.File, err error) {
	if f, err = os.CreateTemp(tmpdir, "buildah-archive-"); err != nil {
		return nil, nil, fmt.Errorf("creating temporary copy of base image: %w", err)
	}
	tw = tar.NewWriter(f)
	return tw, f, nil
}

// create a temporary directory to use as a new OCI layout or "image in a directory" image
func newDirectoryDestination(tmpdir string) (string, error) {
	newDirectory, err := os.MkdirTemp(tmpdir, "buildah-layout-")
	if err != nil {
		return "", fmt.Errorf("creating temporary copy of base image: %w", err)
	}
	return newDirectory, nil
}

// create an archive containing a single item from the build context
func newSingleItemArchive(contextDir, archiveSource string) (io.ReadCloser, error) {
	for {
		// try to make sure the archiver doesn't get thrown by relative prefixes
		if strings.HasPrefix(archiveSource, "/") && archiveSource != "/" {
			archiveSource = strings.TrimPrefix(archiveSource, "/")
			continue
		} else if strings.HasPrefix(archiveSource, "./") && archiveSource != "./" {
			archiveSource = strings.TrimPrefix(archiveSource, "./")
			continue
		}
		break
	}
	// grab only that one file, ignore anything and everything else
	tarOptions := &archive.TarOptions{
		IncludeFiles:    []string{path.Clean(archiveSource)},
		ExcludePatterns: []string{"**"},
	}
	return chrootarchive.Tar(contextDir, tarOptions, contextDir)
}

// Write this header/content combination to a tar writer, making sure that it
// doesn't include any symbolic links that point to something which hasn't
// already been seen in this archive.  Overwrites the contents of `*hdr`.
func writeToArchive(tw *tar.Writer, hdr *tar.Header, content io.Reader, seenEntries map[string]struct{}, convertSymlinksToHardlinks bool) error {
	// write to the archive writer
	hdr.Name = path.Clean("/" + hdr.Name)
	if hdr.Name != "/" {
		hdr.Name = strings.TrimPrefix(hdr.Name, "/")
	}
	seenEntries[hdr.Name] = struct{}{}
	switch hdr.Typeflag {
	case tar.TypeDir, tar.TypeReg, tar.TypeLink:
		// all good
	case tar.TypeSymlink:
		// resolve the target of the symlink
		linkname := hdr.Linkname
		if !path.IsAbs(linkname) {
			linkname = path.Join("/"+path.Dir(hdr.Name), linkname)
		}
		linkname = path.Clean(linkname)
		if linkname != "/" {
			linkname = strings.TrimPrefix(linkname, "/")
		}
		if _, validTarget := seenEntries[linkname]; !validTarget {
			return fmt.Errorf("invalid symbolic link from %q to %q (%q) in archive", hdr.Name, hdr.Linkname, linkname)
		}
		rel, err := filepath.Rel(filepath.FromSlash(path.Dir("/"+hdr.Name)), filepath.FromSlash("/"+linkname))
		if err != nil {
			return fmt.Errorf("computing relative path from %q to %q in archive", hdr.Name, linkname)
		}
		rel = filepath.ToSlash(rel)
		if convertSymlinksToHardlinks {
			// rewrite as a hard link for oci-archive, which gets
			// extracted into a temporary directory to be read, but
			// not for docker-archive, which is read directly from
			// the unextracted archive file, in a way which doesn't
			// understand hard links
			hdr.Typeflag = tar.TypeLink
			hdr.Linkname = linkname
		} else {
			// ensure it's a relative symlink inside of the tree
			// for docker-archive
			hdr.Linkname = rel
		}
	default:
		return fmt.Errorf("rewriting archive of base image: disallowed entry type %c", hdr.Typeflag)
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("rewriting archive of base image: %w", err)
	}
	if hdr.Typeflag == tar.TypeReg {
		n, err := io.Copy(tw, content)
		if err != nil {
			return fmt.Errorf("copying content for %q in base image: %w", hdr.Name, err)
		}
		if n != hdr.Size {
			return fmt.Errorf("short write writing %q in base image: %d != %d", hdr.Name, n, hdr.Size)
		}
	}
	return nil
}

// write this header and possible content to a directory tree
func writeToDirectory(root string, hdr *tar.Header, content io.Reader) error {
	var err error
	// write this item directly to a directory tree. the reader won't care
	// much about permissions or datestamps, so don't bother setting them
	hdr.Name = path.Clean("/" + hdr.Name)
	newName := filepath.Join(root, filepath.FromSlash(hdr.Name))
	switch hdr.Typeflag {
	case tar.TypeDir:
		err = os.Mkdir(newName, 0o700)
	case tar.TypeReg:
		err = func() error {
			var f *os.File
			f, err := os.OpenFile(newName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return fmt.Errorf("copying content for %q in base image: %w", hdr.Name, err)
			}
			n, err := io.Copy(f, content)
			if err != nil {
				f.Close()
				return fmt.Errorf("copying content for %q in base image: %w", hdr.Name, err)
			}
			if n != hdr.Size {
				f.Close()
				return fmt.Errorf("short write writing %q in base image: %d != %d", hdr.Name, n, hdr.Size)
			}
			return f.Close()
		}()
	case tar.TypeLink:
		linkName := path.Clean("/" + hdr.Linkname)
		oldName := filepath.Join(root, filepath.FromSlash(linkName))
		err = os.Link(oldName, newName)
	case tar.TypeSymlink: // convert it to a hard link or absolute symlink
		var oldName string
		if !path.IsAbs(path.Clean("/" + hdr.Linkname)) {
			linkName := path.Join("/"+path.Dir(hdr.Name), hdr.Linkname)
			oldName = filepath.Join(root, filepath.FromSlash(linkName))
		} else {
			oldName = filepath.Join(root, filepath.FromSlash(path.Clean("/"+hdr.Linkname)))
		}
		err = os.Link(oldName, newName)
		if err != nil && errors.Is(err, os.ErrPermission) { // EPERM could mean it's a directory
			if oldInfo, err2 := os.Stat(oldName); err2 == nil && oldInfo.IsDir() {
				err = os.Symlink(oldName, newName)
			}
		}
	default:
		return fmt.Errorf("extracting archive of base image: disallowed entry type %c", hdr.Typeflag)
	}
	if err != nil {
		return fmt.Errorf("creating %q: %w", newName, err)
	}
	return nil
}

// ImageName limits which image transports we'll accept.  For those it accepts
// which refer to filesystem objects, where relative path names are evaluated
// relative to "contextDir", it will create a copy of the original image, under
// "tmpdir", which contains no symbolic links.  It it returns a parseable
// reference to the image which should be used.
func ImageName(transportName, restOfImageName, contextDir, tmpdir string) (newFrom string, err error) {
	seenEntries := make(map[string]struct{})
	// we're going to try to create a temporary directory or file, but if
	// we fail, make sure that they get removed immediately
	newImageDestination := ""
	succeeded := false
	defer func() {
		if !succeeded && newImageDestination != "" {
			if err := os.RemoveAll(newImageDestination); err != nil {
				logrus.Warnf("removing temporary copy of base image in %q: %v", newImageDestination, err)
			}
		}
	}()

	// create an archive of the base image, whatever kind it is, chrooting into
	// the build context directory while doing so, to be sure that it can't
	// be tricked into including anything from outside of the context directory
	isEmbeddedArchive := false
	var f *os.File
	var tw *tar.Writer
	var archiveSource string
	var imageArchive io.ReadCloser
	switch transportName {
	case dockerTransport.Transport.Name(), "docker-daemon", openshiftTransport.Transport.Name(): // ok, these are all remote
		return transportName + ":" + restOfImageName, nil
	case dockerArchiveTransport.Transport.Name(), ociArchiveTransport.Transport.Name(): // these are, basically, tarballs
		// these take the form path[:stuff]
		transportRef := restOfImageName
		as, refLeftover, ok := strings.Cut(transportRef, ":")
		archiveSource = as
		if ok {
			refLeftover = ":" + refLeftover
		}
		// create a temporary file to use as our new archive
		tw, f, err = newArchiveDestination(tmpdir)
		if err != nil {
			return "", fmt.Errorf("creating temporary copy of base image: %w", err)
		}
		newImageDestination = f.Name()
		defer func() {
			if tw != nil {
				if err := tw.Close(); err != nil {
					logrus.Warnf("wrapping up writing copy of base image to archive %q: %v", newImageDestination, err)
				}
			}
			if f != nil {
				if err := f.Close(); err != nil {
					logrus.Warnf("closing copy of base image in archive file %q: %v", newImageDestination, err)
				}
			}
		}()
		// archive only the archive file for copying to the new archive file
		imageArchive, err = newSingleItemArchive(contextDir, archiveSource)
		isEmbeddedArchive = true
		// generate the new reference using the temporary file's name
		newFrom = transportName + ":" + newImageDestination + refLeftover
	case ociLayoutTransport.Transport.Name(): // this is a directory tree
		// this takes the form path[:stuff]
		transportRef := restOfImageName
		as, refLeftover, ok := strings.Cut(transportRef, ":")
		archiveSource = as
		if ok {
			refLeftover = ":" + refLeftover
		}
		// create a new directory to use as our new layout directory
		if newImageDestination, err = newDirectoryDestination(tmpdir); err != nil {
			return "", fmt.Errorf("creating temporary copy of base image: %w", err)
		}
		// archive the entire layout directory for copying to the new layout directory
		tarOptions := &archive.TarOptions{}
		imageArchive, err = chrootarchive.Tar(filepath.Join(contextDir, archiveSource), tarOptions, contextDir)
		// generate the new reference using the directory
		newFrom = transportName + ":" + newImageDestination + refLeftover
	case directoryTransport.Transport.Name(): // this is also a directory tree
		// this takes the form of just a path
		transportRef := restOfImageName
		// create a new directory to use as our new image directory
		if newImageDestination, err = newDirectoryDestination(tmpdir); err != nil {
			return "", fmt.Errorf("creating temporary copy of base image: %w", err)
		}
		// archive the entire directory for copying to the new directory
		archiveSource = transportRef
		tarOptions := &archive.TarOptions{}
		imageArchive, err = chrootarchive.Tar(filepath.Join(contextDir, archiveSource), tarOptions, contextDir)
		// generate the new reference using the directory
		newFrom = transportName + ":" + newImageDestination
	default:
		return "", fmt.Errorf("unexpected container image transport %q", transportName)
	}
	if err != nil {
		return "", fmt.Errorf("error archiving source at %q under %q", archiveSource, contextDir)
	}

	// start reading the archived content
	defer func() {
		if err := imageArchive.Close(); err != nil {
			logrus.Warn(err)
		}
	}()
	tr := tar.NewReader(imageArchive)
	hdr, err := tr.Next()
	for err == nil {
		// if the archive we're parsing is expected to have an archive as its only item,
		// assume it's the first (and hopefully, only) item, and switch to stepping through
		// it as the archive
		if isEmbeddedArchive {
			if hdr.Typeflag != tar.TypeReg {
				return "", fmt.Errorf("internal error passing archive contents: embedded archive type was %c instead of %c", hdr.Typeflag, tar.TypeReg)
			}
			decompressed, _, decompressErr := compression.AutoDecompress(tr)
			if decompressErr != nil {
				return "", fmt.Errorf("error decompressing-if-necessary archive %q: %w", archiveSource, decompressErr)
			}
			defer func() {
				if err := decompressed.Close(); err != nil {
					logrus.Warn(err)
				}
			}()
			tr = tar.NewReader(decompressed)
			hdr, err = tr.Next()
			isEmbeddedArchive = false
			continue
		}
		// write this item from the source archive to either the new archive or the new
		// directory, which ever one we're doing
		var writeError error
		if tw != nil {
			writeError = writeToArchive(tw, hdr, io.LimitReader(tr, hdr.Size), seenEntries, transportName == ociArchiveTransport.Transport.Name())
		} else {
			writeError = writeToDirectory(newImageDestination, hdr, io.LimitReader(tr, hdr.Size))
		}
		if writeError != nil {
			return "", fmt.Errorf("writing copy of image to %q: %w", newImageDestination, writeError)
		}
		hdr, err = tr.Next()
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("reading archive of base image: %w", err)
	}
	if isEmbeddedArchive {
		logrus.Warnf("expected to have archived a copy of %q, missed it", archiveSource)
	}
	if tw != nil {
		if err := tw.Close(); err != nil {
			return "", fmt.Errorf("wrapping up writing copy of base image to archive %q: %w", newImageDestination, err)
		}
		tw = nil
	}
	if f != nil {
		if err := f.Close(); err != nil {
			return "", fmt.Errorf("closing copy of base image in archive file %q: %w", newImageDestination, err)
		}
		f = nil
	}
	succeeded = true
	return newFrom, nil
}
