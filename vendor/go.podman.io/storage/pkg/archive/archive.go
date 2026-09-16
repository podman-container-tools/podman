package archive

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/bzip2"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	gzip "github.com/klauspost/pgzip"
	"github.com/sirupsen/logrus"
	"github.com/ulikunitz/xz"
	"go.podman.io/storage/internal/createpath"
	"go.podman.io/storage/pkg/fileutils"
	"go.podman.io/storage/pkg/idtools"
	"go.podman.io/storage/pkg/pools"
	"go.podman.io/storage/pkg/promise"
	"go.podman.io/storage/pkg/system"
	"go.podman.io/storage/pkg/unshare"
)

type (
	// Compression is the state represents if compressed or not.
	Compression int
	// WhiteoutFormat is the format of whiteouts unpacked
	WhiteoutFormat int

	// TarOptions wraps the tar options.
	TarOptions struct {
		IncludeFiles      []string
		ExcludePatterns   []string
		Compression       Compression
		NoLchown          bool
		UIDMaps           []idtools.IDMap
		GIDMaps           []idtools.IDMap
		IgnoreChownErrors bool
		ChownOpts         *idtools.IDPair
		IncludeSourceDir  bool
		// WhiteoutFormat is the expected on disk format for whiteout files.
		// This format will be converted to the standard format on pack
		// and from the standard format on unpack.
		WhiteoutFormat WhiteoutFormat
		// This is additional data to be used by the converter.  It will
		// not survive a round trip through JSON, so it's primarily
		// intended for generating archives (i.e., converting writes).
		WhiteoutData any
		// When unpacking, specifies whether overwriting a directory with a
		// non-directory is allowed and vice versa.
		NoOverwriteDirNonDir bool
		// For each include when creating an archive, the included name will be
		// replaced with the matching name from this map.
		RebaseNames map[string]string
		InUserNS    bool
		// CopyPass indicates that the contents of any archive we're creating
		// will instantly be extracted and written to disk, so we can deviate
		// from the traditional behavior/format to get features like subsecond
		// precision in timestamps.
		CopyPass bool
		// ForceMask, if set, indicates the permission mask used for created files.
		ForceMask *os.FileMode
		// Timestamp, if set, will be set in each header as create/mod/access time
		Timestamp *time.Time
		// InternalRunningInMinimalChroot is an internal implementation detail of storage/pkg/chrootarchive calling TarWithOptions.
		// It will hopefully be removed in the future, and must not be set by any other callers.
		InternalRunningInMinimalChroot bool
	}
)

const PaxSchilyXattr = "SCHILY.xattr."

const (
	tarExt  = "tar"
	windows = "windows"
	darwin  = "darwin"
)

var xattrsToIgnore = map[string]any{
	"security.selinux": true,
}

// Archiver allows the reuse of most utility functions of this package with a
// pluggable Untar function.  To facilitate the passing of specific id mappings
// for untar, an archiver can be created with maps which will then be passed to
// Untar operations.  If ChownOpts is set, its values are mapped using
// UntarIDMappings before being used to create files and directories on disk.
type Archiver struct {
	Untar           func(io.Reader, string, *TarOptions) error
	TarIDMappings   *idtools.IDMappings
	ChownOpts       *idtools.IDPair
	UntarIDMappings *idtools.IDMappings
}

// NewDefaultArchiver returns a new Archiver without any IDMappings
func NewDefaultArchiver() *Archiver {
	return &Archiver{Untar: Untar, TarIDMappings: &idtools.IDMappings{}, UntarIDMappings: &idtools.IDMappings{}}
}

// breakoutError is used to differentiate errors related to breaking out
// When testing archive breakout in the unit tests, this error is expected
// in order for the test to pass.
type breakoutError error

// overwriteError is used to differentiate errors related to attempting to
// overwrite a directory with a non-directory or vice-versa.  When testing
// copying a file over a directory, this error is expected in order for the
// test to pass.
type overwriteError error

const (
	// Uncompressed represents the uncompressed.
	Uncompressed Compression = iota
	// Bzip2 is bzip2 compression algorithm.
	Bzip2
	// Gzip is gzip compression algorithm.
	Gzip
	// Xz is xz compression algorithm.
	Xz
	// Zstd is zstd compression algorithm.
	Zstd
)

const (
	// AUFSWhiteoutFormat is the default format for whiteouts
	AUFSWhiteoutFormat WhiteoutFormat = iota
	// OverlayWhiteoutFormat formats whiteout according to the overlay
	// standard.
	OverlayWhiteoutFormat
)

// IsArchivePath checks if the (possibly compressed) file at the given path
// starts with a tar file header.
func IsArchivePath(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	rdr, err := DecompressStream(file)
	if err != nil {
		return false
	}
	defer rdr.Close()
	r := tar.NewReader(rdr)
	_, err = r.Next()
	return err == nil
}

// DetectCompression detects the compression algorithm of the source.
func DetectCompression(source []byte) Compression {
	for compression, m := range map[Compression][]byte{
		Bzip2: {0x42, 0x5A, 0x68},
		Gzip:  {0x1F, 0x8B, 0x08},
		Xz:    {0xFD, 0x37, 0x7A, 0x58, 0x5A, 0x00},
		Zstd:  {0x28, 0xb5, 0x2f, 0xfd},
	} {
		if len(source) < len(m) {
			logrus.Debug("Len too short")
			continue
		}
		if bytes.Equal(m, source[:len(m)]) {
			return compression
		}
	}
	return Uncompressed
}

// DecompressStream decompresses the archive and returns a ReaderCloser with the decompressed archive.
func DecompressStream(archive io.Reader) (_ io.ReadCloser, Err error) {
	p := pools.BufioReader32KPool
	buf := p.Get(archive)
	bs, err := buf.Peek(10)

	defer func() {
		if Err != nil {
			// In the normal case, the buffer is embedded in the ReadCloser return.
			p.Put(buf)
		}
	}()

	if err != nil && err != io.EOF {
		// Note: we'll ignore any io.EOF error because there are some odd
		// cases where the layer.tar file will be empty (zero bytes) and
		// that results in an io.EOF from the Peek() call. So, in those
		// cases we'll just treat it as a non-compressed stream and
		// that means just create an empty layer.
		// See Issue 18170
		return nil, err
	}

	compression := DetectCompression(bs)
	switch compression {
	case Uncompressed:
		readBufWrapper := p.NewReadCloserWrapper(buf, buf)
		return readBufWrapper, nil
	case Gzip:
		cleanup := func() {
			p.Put(buf)
		}
		if rc, canUse := tryProcFilter([]string{"pigz", "-d"}, buf, cleanup); canUse {
			return rc, nil
		}
		gzReader, err := gzip.NewReader(buf)
		if err != nil {
			return nil, err
		}
		readBufWrapper := p.NewReadCloserWrapper(buf, gzReader)
		return readBufWrapper, nil
	case Bzip2:
		bz2Reader := bzip2.NewReader(buf)
		readBufWrapper := p.NewReadCloserWrapper(buf, bz2Reader)
		return readBufWrapper, nil
	case Xz:
		xzReader, err := xz.NewReader(buf)
		if err != nil {
			return nil, err
		}
		readBufWrapper := p.NewReadCloserWrapper(buf, xzReader)
		return readBufWrapper, nil
	case Zstd:
		cleanup := func() {
			p.Put(buf)
		}
		if rc, canUse := tryProcFilter([]string{"zstd", "-d"}, buf, cleanup); canUse {
			return rc, nil
		}
		return zstdReader(buf)
	default:
		return nil, fmt.Errorf("unsupported compression format %s", (&compression).Extension())
	}
}

// CompressStream compresses the dest with specified compression algorithm.
func CompressStream(dest io.Writer, compression Compression) (_ io.WriteCloser, Err error) {
	p := pools.BufioWriter32KPool
	buf := p.Get(dest)

	defer func() {
		if Err != nil {
			p.Put(buf)
		}
	}()

	switch compression {
	case Uncompressed:
		writeBufWrapper := p.NewWriteCloserWrapper(buf, buf)
		return writeBufWrapper, nil
	case Gzip:
		gzWriter := gzip.NewWriter(dest)
		writeBufWrapper := p.NewWriteCloserWrapper(buf, gzWriter)
		return writeBufWrapper, nil
	case Zstd:
		return zstdWriter(dest)
	case Bzip2, Xz:
		// archive/bzip2 does not support writing, and there is no xz support at all
		// However, this is not a problem as docker only currently generates gzipped tars
		return nil, fmt.Errorf("unsupported compression format %s", (&compression).Extension())
	default:
		return nil, fmt.Errorf("unsupported compression format %s", (&compression).Extension())
	}
}

// TarModifierFunc is a function that can be passed to ReplaceFileTarWrapper to
// modify the contents or header of an entry in the archive. If the file already
// exists in the archive the TarModifierFunc will be called with the Header and
// a reader which will return the files content. If the file does not exist both
// header and content will be nil.
type TarModifierFunc func(path string, header *tar.Header, content io.Reader) (*tar.Header, []byte, error)

// ReplaceFileTarWrapper converts inputTarStream to a new tar stream. Files in the
// tar stream are modified if they match any of the keys in mods.
func ReplaceFileTarWrapper(inputTarStream io.ReadCloser, mods map[string]TarModifierFunc) io.ReadCloser {
	pipeReader, pipeWriter := io.Pipe()

	go func() {
		tarReader := tar.NewReader(inputTarStream)
		tarWriter := tar.NewWriter(pipeWriter)
		defer inputTarStream.Close()
		defer tarWriter.Close()

		modify := func(name string, original *tar.Header, modifier TarModifierFunc, tarReader io.Reader) error {
			header, data, err := modifier(name, original, tarReader)
			switch {
			case err != nil:
				return err
			case header == nil:
				return nil
			}

			header.Name = name
			header.Size = int64(len(data))
			if err := tarWriter.WriteHeader(header); err != nil {
				return err
			}
			if len(data) != 0 {
				if _, err := tarWriter.Write(data); err != nil {
					return err
				}
			}
			return nil
		}

		var err error
		var originalHeader *tar.Header
		for {
			originalHeader, err = tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				pipeWriter.CloseWithError(err)
				return
			}

			modifier, ok := mods[originalHeader.Name]
			if !ok {
				// No modifiers for this file, copy the header and data
				if err := tarWriter.WriteHeader(originalHeader); err != nil {
					pipeWriter.CloseWithError(err)
					return
				}
				if _, err := pools.Copy(tarWriter, tarReader); err != nil {
					pipeWriter.CloseWithError(err)
					return
				}
				continue
			}
			delete(mods, originalHeader.Name)

			if err := modify(originalHeader.Name, originalHeader, modifier, tarReader); err != nil {
				pipeWriter.CloseWithError(err)
				return
			}
		}

		// Apply the modifiers that haven't matched any files in the archive
		for name, modifier := range mods {
			if err := modify(name, nil, modifier, nil); err != nil {
				pipeWriter.CloseWithError(err)
				return
			}
		}

		pipeWriter.Close()
	}()
	return pipeReader
}

// Extension returns the extension of a file that uses the specified compression algorithm.
func (compression *Compression) Extension() string {
	switch *compression {
	case Uncompressed:
		return tarExt
	case Bzip2:
		return tarExt + ".bz2"
	case Gzip:
		return tarExt + ".gz"
	case Xz:
		return tarExt + ".xz"
	case Zstd:
		return tarExt + ".zst"
	}
	return ""
}

// nosysFileInfo hides the system-dependent info of the wrapped FileInfo to
// prevent tar.FileInfoHeader from introspecting it and potentially calling into
// glibc.
type nosysFileInfo struct {
	os.FileInfo
}

func (fi nosysFileInfo) Sys() any {
	// A Sys value of type *tar.Header is safe as it is system-independent.
	// The tar.FileInfoHeader function copies the fields into the returned
	// header without performing any OS lookups.
	if sys, ok := fi.FileInfo.Sys().(*tar.Header); ok {
		return sys
	}
	return nil
}

// sysStatOverride, if non-nil, populates hdr from system-dependent fields of fi.
var sysStatOverride func(fi os.FileInfo, hdr *tar.Header) error

func fileInfoHeaderNoLookups(fi os.FileInfo, link string) (*tar.Header, error) {
	if sysStatOverride == nil {
		return tar.FileInfoHeader(fi, link)
	}
	hdr, err := tar.FileInfoHeader(nosysFileInfo{fi}, link)
	if err != nil {
		return nil, err
	}
	return hdr, sysStatOverride(fi, hdr)
}

// FileInfoHeader creates a populated Header from fi.
// Compared to archive pkg this function fills in more information.
// Also, regardless of Go version, this function fills file type bits (e.g. hdr.Mode |= modeISDIR),
// which have been deleted since Go 1.9 archive/tar.
func FileInfoHeader(name string, fi os.FileInfo, link string) (*tar.Header, error) {
	hdr, err := fileInfoHeaderNoLookups(fi, link)
	if err != nil {
		return nil, err
	}
	hdr.Mode = int64(chmodTarEntry(os.FileMode(hdr.Mode)))
	name, err = canonicalTarName(name, fi.IsDir())
	if err != nil {
		return nil, fmt.Errorf("tar: cannot canonicalize path: %w", err)
	}
	hdr.Name = name
	setHeaderForSpecialDevice(hdr, fi.Sys())
	return hdr, nil
}

// See include/uapi/linux/capability.h in the kernel sources.
const (
	vfsCapRevisionMask = 0xFF000000
	vfsCapRevision2    = 0x02000000
	vfsCapRevision3    = 0x03000000
	vfsCapDataSizeV2   = 20
	vfsCapDataSizeV3   = 24
	vfsCapRootIDOffset = 20
)

// normalizeCapabilityRootID rewrites the rootid of a v3 "security.capability"
// value from the host user namespace into the container namespace described by
// idMappings, so an id-shifted layer records the container-relative owner rather
// than a host-specific UID. Ownership is remapped the same way for the file
// itself in prepareAddFile.
//
// Values that are not v3 are returned unchanged. When the rootid maps to container
// root, the value is downgraded to v2, matching what the kernel returns when the
// capability is read from within the container namespace.
func normalizeCapabilityRootID(idMappings *idtools.IDMappings, capData []byte) ([]byte, error) {
	if idMappings == nil || idMappings.Empty() || len(capData) != vfsCapDataSizeV3 {
		return capData, nil
	}
	magicEtc := binary.LittleEndian.Uint32(capData[:4])
	if magicEtc&vfsCapRevisionMask != vfsCapRevision3 {
		return capData, nil
	}
	hostRootID := binary.LittleEndian.Uint32(capData[vfsCapRootIDOffset:])
	containerID, err := idtools.RawToContainer(int(hostRootID), idMappings.UIDs())
	if err != nil {
		return nil, err
	}
	if containerID == 0 {
		v2 := make([]byte, vfsCapDataSizeV2)
		copy(v2, capData[:vfsCapDataSizeV2])
		binary.LittleEndian.PutUint32(v2[:4], (magicEtc&^vfsCapRevisionMask)|vfsCapRevision2)
		return v2, nil
	}
	out := make([]byte, vfsCapDataSizeV3)
	copy(out, capData)
	binary.LittleEndian.PutUint32(out[vfsCapRootIDOffset:], uint32(containerID))
	return out, nil
}

// readSecurityXattrToTarHeader reads security.capability, security,image
// xattrs from filesystem to a tar header
func (ta *tarWriter) readSecurityXattrToTarHeader(root *os.Root, fsPath string, hdr *tar.Header) error {
	if hdr.PAXRecords == nil {
		hdr.PAXRecords = make(map[string]string)
	}
	for _, xattr := range []string{"security.capability", "security.ima"} {
		var capability []byte
		var err error
		if !ta.runningInMinimalChroot {
			capability, err = system.RootLgetxattr(root, fsPath, xattr)
		} else {
			capability, err = system.Lgetxattr(filepath.Join(root.Name(), filepath.FromSlash(fsPath)), xattr)
		}
		if err != nil && !errors.Is(err, system.ENOTSUP) && err != system.ErrNotSupportedPlatform {
			return fmt.Errorf("failed to read %q attribute from %q in %q: %w", xattr, fsPath, root.Name(), err)
		}
		if capability == nil {
			continue
		}
		if xattr == "security.capability" {
			capability, err = normalizeCapabilityRootID(ta.IDMappings, capability)
			if err != nil {
				return fmt.Errorf("failed to normalize %q attribute from %q: %w", xattr, fsPath, err)
			}
		}
		hdr.PAXRecords[PaxSchilyXattr+xattr] = string(capability)
	}
	return nil
}

// readUserXattrToTarHeader reads user.* xattr from filesystem to a tar header
func (ta *tarWriter) readUserXattrToTarHeader(root *os.Root, fsPath string, hdr *tar.Header) error {
	var xattrs []string
	var err error
	if !ta.runningInMinimalChroot {
		xattrs, err = system.RootLlistxattr(root, fsPath)
	} else {
		xattrs, err = system.Llistxattr(filepath.Join(root.Name(), filepath.FromSlash(fsPath)))
	}
	if err != nil && !errors.Is(err, system.ENOTSUP) && err != system.ErrNotSupportedPlatform {
		return err
	}
	for _, key := range xattrs {
		if strings.HasPrefix(key, "user.") && !strings.HasPrefix(key, "user.overlay.") {
			var value []byte
			var err error
			if !ta.runningInMinimalChroot {
				value, err = system.RootLgetxattr(root, fsPath, key)
			} else {
				value, err = system.Lgetxattr(filepath.Join(root.Name(), filepath.FromSlash(fsPath)), key)
			}
			if err != nil {
				if errors.Is(err, system.E2BIG) {
					logrus.Errorf("archive: Skipping xattr for file %q in %q since value is too big: %s", fsPath, root.Name(), key)
					continue
				}
				return err
			}
			if hdr.PAXRecords == nil {
				hdr.PAXRecords = make(map[string]string)
			}
			hdr.PAXRecords[PaxSchilyXattr+key] = string(value)
		}
	}
	return nil
}

type TarWhiteoutHandler interface {
	Setxattr(path, name string, value []byte) error
	Mknod(path string, mode uint32, dev int) error
	Chown(path string, uid, gid int) error
}

type TarWhiteoutConverter interface {
	// There is no documented way to call GetWhiteoutConverter. Consider this deprecated.
	// Do not add any more public methods; if unavoidable, add them to the private tarWhiteoutConverter.

	ConvertWrite(*tar.Header, string, os.FileInfo) (*tar.Header, error)
	ConvertRead(*tar.Header, string) (bool, error)
	ConvertReadWithHandler(*tar.Header, string, TarWhiteoutHandler) (bool, error)
}

type tarWhiteoutConverter interface {
	TarWhiteoutConverter
	convertWrite(hdr *tar.Header, root *os.Root, fsPath string, fi os.FileInfo) (*tar.Header, error)
}

// GetWhiteoutConverter has no documented way to be called externally. Do not add any users outside of c/storage.
func GetWhiteoutConverter(format WhiteoutFormat, data any) TarWhiteoutConverter {
	return getWhiteoutConverter(format, data, nil)
}

type tarWriter struct {
	TarWriter *tar.Writer
	Buffer    *bufio.Writer

	// for hardlink mapping
	SeenFiles  map[uint64]string
	IDMappings *idtools.IDMappings
	ChownOpts  *idtools.IDPair

	// For packing and unpacking whiteout files in the
	// non standard format. The whiteout files defined
	// by the AUFS standard are used as the tar whiteout
	// standard.
	whiteoutConverter tarWhiteoutConverter
	// CopyPass indicates that the contents of any archive we're creating
	// will instantly be extracted and written to disk, so we can deviate
	// from the traditional behavior/format to get features like subsecond
	// precision in timestamps.
	CopyPass bool

	// Timestamp, if set, will be set in each header as create/mod/access time
	Timestamp *time.Time

	// runningInMinimalChroot indicates that we are confined to a fairly strict chroot,
	// so we don’t need to worry about Lgetxattr / Llistxattr escaping the source directory.
	//
	// We need this because:
	// - getxattrat() would be ideal, but requires Linux 6.13, and as of 2026-05 that might still be too new
	// - Our fallback is to open "/proc/self/fd/$fd" of an O_PATH file handle, but /proc is not available in these chroots.
	runningInMinimalChroot bool
}

func newTarWriter(idMapping *idtools.IDMappings, writer io.Writer, chownOpts *idtools.IDPair, timestamp *time.Time, runningInChroot bool) *tarWriter {
	return &tarWriter{
		SeenFiles:              make(map[uint64]string),
		TarWriter:              tar.NewWriter(writer),
		Buffer:                 pools.BufioWriter32KPool.Get(nil),
		IDMappings:             idMapping,
		ChownOpts:              chownOpts,
		Timestamp:              timestamp,
		runningInMinimalChroot: runningInChroot,
	}
}

// canonicalTarName provides a platform-independent and consistent posix-style
// path for files and directories to be archived regardless of the platform.
func canonicalTarName(name string, isDir bool) (string, error) {
	name, err := CanonicalTarNameForPath(name)
	if err != nil {
		return "", err
	}

	// suffix with '/' for directories
	if isDir && !strings.HasSuffix(name, "/") {
		name += "/"
	}
	return name, nil
}

type addFileData struct {
	// The path within a separately-provided root from which to read contents.
	fsPath string

	// os.Stat for the above.
	fi os.FileInfo

	// The file header of the above.
	hdr *tar.Header

	// if present, an extra whiteout entry to write after the header.
	extraWhiteout *tar.Header
}

// prepareAddFile generates the tar file header(s) for adding a file
// from fsPath within root as tarName to the tar archive, without writing to the
// tar stream. Thus, any error may be ignored without corrupting the
// tar file. A (nil, nil) return means that the file should be
// ignored for non-error reasons.
func (ta *tarWriter) prepareAddFile(root *os.Root, fsPath, tarName string) (*addFileData, error) {
	// WARNING: This function is called in contexts where the contents of root may be maliciously
	// concurrently modified.

	fi, err := root.Lstat(fsPath) // FIXME: can we eliminate this and Lstat only once, e.g. from fs.WalkDir()?
	if err != nil {
		return nil, err
	}

	var link string
	if fi.Mode()&os.ModeSymlink != 0 {
		var err error
		link, err = root.Readlink(fsPath)
		if err != nil {
			return nil, err
		}
	}
	if fi.Mode()&os.ModeSocket != 0 {
		logrus.Infof("archive: skipping %q in %q since it is a socket", fsPath, root.Name())
		return nil, nil
	}

	hdr, err := FileInfoHeader(tarName, fi, link)
	if err != nil {
		return nil, err
	}
	if err := ta.readSecurityXattrToTarHeader(root, fsPath, hdr); err != nil {
		return nil, err
	}
	if err := ta.readUserXattrToTarHeader(root, fsPath, hdr); err != nil {
		return nil, err
	}
	if err := readFileFlagsToTarHeader(fi, hdr); err != nil {
		return nil, err
	}
	if ta.CopyPass {
		copyPassHeader(hdr)
	}

	// if it's not a directory and has more than 1 link,
	// it's hard linked, so set the type flag accordingly
	if !fi.IsDir() && hasHardlinks(fi) {
		inode := getInodeFromStat(fi.Sys())
		// a link should have a name that it links too
		// and that linked name should be first in the tar archive
		if oldpath, ok := ta.SeenFiles[inode]; ok {
			hdr.Typeflag = tar.TypeLink
			hdr.Linkname = oldpath
			hdr.Size = 0 // This must be here for the writer math to add up!
		}
	}

	// handle re-mapping container ID mappings back to host ID mappings before
	// writing tar headers/files. We skip whiteout files because they were written
	// by the kernel and already have proper ownership relative to the host
	if !strings.HasPrefix(filepath.Base(hdr.Name), WhiteoutPrefix) && !ta.IDMappings.Empty() {
		fileIDPair, err := getFileUIDGID(fi.Sys())
		if err != nil {
			return nil, err
		}
		hdr.Uid, hdr.Gid, err = ta.IDMappings.ToContainer(fileIDPair)
		if err != nil {
			return nil, err
		}
	}

	// explicitly override with ChownOpts
	if ta.ChownOpts != nil {
		hdr.Uid = ta.ChownOpts.UID
		hdr.Gid = ta.ChownOpts.GID
		// Don’t expose the user names from the local system; they probably don’t match the ta.ChownOpts value anyway,
		// and they unnecessarily give recipients of the tar file potentially private data.
		hdr.Uname = ""
		hdr.Gname = ""
	}

	// if override timestamp set, replace all times with this
	if ta.Timestamp != nil {
		hdr.ModTime = *ta.Timestamp
		hdr.AccessTime = *ta.Timestamp
		hdr.ChangeTime = *ta.Timestamp
	}

	maybeTruncateHeaderModTime(hdr)

	result := &addFileData{
		fsPath: fsPath,
		hdr:    hdr,
		fi:     fi,
	}
	if ta.whiteoutConverter != nil {
		// The whiteoutConverter suggests a generic mechanism,
		// but this code is only used to convert between
		// overlayfs (on-disk) and AUFS (in the tar file)
		// whiteouts, and is initiated because the overlayfs
		// storage driver returns OverlayWhiteoutFormat from
		// Driver.getWhiteoutFormat().
		//
		// For AUFS, a directory with all its contents deleted
		// should be represented as a directory containing a
		// magic whiteout empty regular file, hence the
		// extraWhiteout header returned here.
		result.extraWhiteout, err = ta.whiteoutConverter.convertWrite(hdr, root, fsPath, fi)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// addFile performs the write. An error here corrupts the tar file.
func (ta *tarWriter) addFile(root *os.Root, headers *addFileData) error {
	// WARNING: This function is called in contexts where the contents of root may be maliciously
	// concurrently modified.

	hdr := headers.hdr
	if headers.extraWhiteout != nil {
		if hdr.Typeflag == tar.TypeReg && hdr.Size > 0 {
			// If we write hdr with hdr.Size > 0, we have
			// to write the body before we can write the
			// extraWhiteout header. This can only happen
			// if the contract for WhiteoutConverter is
			// not honored, so bail out.
			return fmt.Errorf("tar: cannot use extra whiteout with non-empty file %s", hdr.Name)
		}
		if err := ta.TarWriter.WriteHeader(hdr); err != nil {
			return err
		}
		hdr = headers.extraWhiteout
	}

	if err := ta.TarWriter.WriteHeader(hdr); err != nil {
		return err
	}

	if hdr.Typeflag == tar.TypeReg && hdr.Size > 0 {
		file, err := root.Open(headers.fsPath)
		if err != nil {
			return err
		}

		ta.Buffer.Reset(ta.TarWriter)
		defer ta.Buffer.Reset(nil)
		_, err = io.Copy(ta.Buffer, file)
		file.Close()
		if err != nil {
			return err
		}
		err = ta.Buffer.Flush()
		if err != nil {
			return err
		}
	}

	if !headers.fi.IsDir() && hasHardlinks(headers.fi) {
		ino := getInodeFromStat(headers.fi.Sys())
		if _, seen := ta.SeenFiles[ino]; !seen {
			ta.SeenFiles[ino] = headers.hdr.Name
		}
	}

	return nil
}

// path must not exist (unless it is a directory and hdr also specifies a directory)
// If hdr specifies a hard link, it is interpreted relative to extractDir (as the root to which the
// tar extraction should be confined to); but path might not be inside extractDir!
func extractTarFileEntry(path, extractDir string, hdr *tar.Header, reader io.Reader, Lchown bool, chownOpts *idtools.IDPair, inUserns, ignoreChownErrors bool, forceMask *os.FileMode, buffer []byte) error {
	// hdr.Mode is in linux format, which we can use for sycalls,
	// but for os.Foo() calls we need the mode converted to os.FileMode,
	// so use hdrInfo.Mode() (they differ for e.g. setuid bits)
	hdrInfo := hdr.FileInfo()

	typeFlag := hdr.Typeflag
	mask := hdrInfo.Mode()
	var hardlinkTargetPath string

	// update also the implementation of ForceMask in pkg/chunked
	if forceMask != nil {
		mask = *forceMask
		// If we have a forceMask, force the real type to either be a directory,
		// a link, or a regular file.
		if typeFlag != tar.TypeDir && typeFlag != tar.TypeSymlink && typeFlag != tar.TypeLink {
			typeFlag = tar.TypeReg
		}
	}

	switch typeFlag {
	case tar.TypeDir:
		// Create directory unless it exists as a directory already.
		// In that case we just want to merge the two
		if fi, err := os.Lstat(path); err != nil || !fi.IsDir() {
			if err := os.Mkdir(path, mask); err != nil {
				return err
			}
		}

	case tar.TypeReg:
		// Source is regular file. We use system.OpenFileSequential to use sequential
		// file access to avoid depleting the standby list on Windows.
		// On Linux, this equates to a regular os.OpenFile
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, mask)
		if err != nil {
			return err
		}
		if _, err := io.CopyBuffer(file, reader, buffer); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}

	case tar.TypeBlock, tar.TypeChar:
		if inUserns { // cannot create devices in a userns
			logrus.Debugf("Tar: Can't create device %v while running in user namespace", path)
			return nil
		}
		fallthrough
	case tar.TypeFifo:
		// Handle this is an OS-specific way
		if err := handleTarTypeBlockCharFifo(hdr, path); err != nil {
			return err
		}

	case tar.TypeLink:
		// Check for hardlink breakout. This is INSUFFICIENT as a check if the path includes symlinks;
		// we preserve it only to keep the existing restrictions on acceptable inputs.
		insecureTargetPath := filepath.Join(extractDir, hdr.Linkname)
		if insecureTargetPath != extractDir {
			insecureExpectedPrefix := extractDir
			if insecureExpectedPrefix != string(os.PathSeparator) {
				insecureExpectedPrefix += string(os.PathSeparator)
			}
			if !strings.HasPrefix(insecureTargetPath, insecureExpectedPrefix) {
				return breakoutError(fmt.Errorf("invalid hardlink %q", hdr.Linkname))
			}
		}

		targetHdrDir, targetHdrBase, err := createpath.SplitPath(hdr.Linkname)
		if err != nil {
			return err
		}
		targetParentPath, err := securejoin.SecureJoin(extractDir, targetHdrDir)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetParentPath, targetHdrBase) // Warning: this can refer to an existing (and escaping) symlink
		if err := handleLLink(targetPath, path); err != nil {
			return err
		}
		hardlinkTargetPath = targetPath

	case tar.TypeSymlink:
		// Yes, this allows arbitrary symlinks: absolute, relative to existing files, relative dangling,
		// relative and escaping the target directory.
		//
		// The callers who extract the archive are expected to constrain path lookup to avoid escaping symlinks,
		// if relevant for their use case.
		//
		// Previously this code attempted to restrict escaping symlinks, but that code was flawed and possible to bypass;
		// OTOH storing non-path data in symlinks is generally legitimate enough, and because the previous code was flawed,
		// callers have always had to constrain path lookup after extracting the archive.
		if err := os.Symlink(hdr.Linkname, path); err != nil {
			return err
		}

	case tar.TypeXGlobalHeader:
		logrus.Debug("PAX Global Extended Headers found and ignored")
		return nil

	default:
		return fmt.Errorf("unhandled tar header type %d", hdr.Typeflag)
	}

	// Lchown is not supported on Windows.
	if Lchown && runtime.GOOS != windows {
		if chownOpts == nil {
			chownOpts = &idtools.IDPair{UID: hdr.Uid, GID: hdr.Gid}
		}
		err := idtools.SafeLchown(path, chownOpts.UID, chownOpts.GID)
		if err != nil {
			if ignoreChownErrors {
				fmt.Fprintf(os.Stderr, "Chown error detected. Ignoring due to ignoreChownErrors flag: %v\n", err)
			} else {
				return err
			}
		}
	}

	// There is no LChmod, so ignore mode for symlink. Also, this
	// must happen after chown, as that can modify the file mode
	if err := handleLChmod(hdr, path, hardlinkTargetPath, hdrInfo, forceMask); err != nil {
		return err
	}

	aTime := hdr.AccessTime
	if aTime.Before(hdr.ModTime) {
		// Last access time should never be before last modified time.
		aTime = hdr.ModTime
	}

	// system.Chtimes doesn't support a NOFOLLOW flag atm
	if hdr.Typeflag == tar.TypeLink {
		if fi, err := os.Lstat(hardlinkTargetPath); err == nil && (fi.Mode()&os.ModeSymlink == 0) {
			if err := system.Chtimes(path, aTime, hdr.ModTime); err != nil {
				return err
			}
		}
	} else if hdr.Typeflag != tar.TypeSymlink {
		if err := system.Chtimes(path, aTime, hdr.ModTime); err != nil {
			return err
		}
	} else {
		ts := []syscall.Timespec{timeToTimespec(aTime), timeToTimespec(hdr.ModTime)}
		if err := system.LUtimesNano(path, ts); err != nil && err != system.ErrNotSupportedPlatform {
			return err
		}
	}

	var errs []string
	for key, value := range hdr.PAXRecords {
		xattrKey, ok := strings.CutPrefix(key, PaxSchilyXattr)
		if !ok {
			continue
		}
		if _, found := xattrsToIgnore[xattrKey]; found {
			continue
		}
		if err := system.Lsetxattr(path, xattrKey, []byte(value), 0); err != nil {
			if errors.Is(err, system.ENOTSUP) || (inUserns && errors.Is(err, syscall.EPERM)) {
				// Ignore specific error cases:
				// - ENOTSUP: Expected for graphdrivers lacking extended attribute support:
				//   - Legacy AUFS versions
				//   - FreeBSD with unsupported namespaces (trusted, security)
				// - EPERM: Expected when operating within a user namespace
				// All other errors will cause a failure.
				errs = append(errs, err.Error())
				continue
			}
			return err
		}
	}

	if forceMask != nil && (typeFlag == tar.TypeReg || typeFlag == tar.TypeDir || runtime.GOOS == "darwin") {
		value := idtools.Stat{
			IDs:   idtools.IDPair{UID: hdr.Uid, GID: hdr.Gid},
			Mode:  hdrInfo.Mode(),
			Major: int(hdr.Devmajor),
			Minor: int(hdr.Devminor),
		}
		if err := idtools.SetContainersOverrideXattr(path, value); err != nil {
			return err
		}
	}

	// We defer setting flags on directories until the end of
	// Unpack or UnpackLayer in case setting them makes the
	// directory immutable.
	if hdr.Typeflag != tar.TypeDir {
		if err := WriteFileFlagsFromTarHeader(path, hdr); err != nil {
			return err
		}
	}

	if len(errs) > 0 {
		logrus.WithFields(logrus.Fields{
			"errors": errs,
		}).Warn("ignored xattrs in archive: underlying filesystem doesn't support them")
	}

	return nil
}

// Tar creates an archive from the directory at `path`, and returns it as a
// stream of bytes. This is a convenience wrapper for [TarWithOptions].
func Tar(path string, compression Compression) (io.ReadCloser, error) {
	return TarWithOptions(path, &TarOptions{Compression: compression})
}

func tarWithOptionsTo(dest io.Writer, srcPath string, options *TarOptions) (result error) {
	// WARNING: This is called in contexts where the contents of srcPath may be maliciously
	// concurrently modified.

	// Fix the source path to work with long path names. This is a no-op
	// on platforms other than Windows.
	srcPath = fixVolumePathPrefix(srcPath)

	pm, err := fileutils.NewPatternMatcher(options.ExcludePatterns)
	if err != nil {
		return err
	}

	compressWriter, err := CompressStream(dest, options.Compression)
	if err != nil {
		return err
	}

	ta := newTarWriter(
		idtools.NewIDMappingsFromMaps(options.UIDMaps, options.GIDMaps),
		compressWriter,
		options.ChownOpts,
		options.Timestamp,
		options.InternalRunningInMinimalChroot,
	)
	ta.whiteoutConverter = getWhiteoutConverter(options.WhiteoutFormat, options.WhiteoutData, options)
	ta.CopyPass = options.CopyPass

	includeFiles := options.IncludeFiles
	defer func() {
		if err := compressWriter.Close(); err != nil && result == nil {
			result = err
		}
	}()

	// this buffer is needed for the duration of this piped stream
	defer pools.BufioWriter32KPool.Put(ta.Buffer)

	// In general we log errors here but ignore them because
	// during e.g. a diff operation the container can continue
	// mutating the filesystem and we can see transient errors
	// from this

	// os.OpenRoot requires the root to be a directory, and fails with an untyped error otherwise;
	// it also follows symlinks.
	// So, we can't later join a non-dir with any includes if we can't get a root for the non-dir.
	// So, we must split the source path and use the basename as the include;
	// this is not using _exactly_ the specified srcDir as root, so it provides worse protection,
	// but in the worrisome “contexts where the contents of srcPath may be maliciously
	// concurrently modified” case srcPath definitely is a directory.
	stat, err := os.Lstat(srcPath)
	if err != nil {
		return err
	}

	if !stat.IsDir() {
		if len(includeFiles) > 0 {
			logrus.Warn("Tar: Can't archive a file with includes")
		}

		dir, base := SplitPathDirEntry(srcPath)
		srcPath = dir
		includeFiles = []string{base}
	}
	root, err := os.OpenRoot(srcPath)
	if err != nil {
		return err
	}
	defer root.Close()

	if len(includeFiles) == 0 {
		includeFiles = []string{"."}
	}

	seen := make(map[string]bool)

	for _, include := range includeFiles {
		rebaseName := options.RebaseNames[include]

		// root.FS() restricts paths to fs.ValidPath(), while callers might pass /absolute/paths.
		walkRoot := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(include)), "/")
		if walkRoot == "" {
			walkRoot = "."
		}
		if err := fs.WalkDir(root.FS(), walkRoot, func(fsFilePath string, d fs.DirEntry, err error) error {
			if err != nil {
				logrus.Errorf("Tar: Can't stat file %q in %q to tar: %s", fsFilePath, root.Name(), err)
				return nil
			}

			relFilePath := filepath.FromSlash(fsFilePath)
			if !options.IncludeSourceDir && relFilePath == "." && d.IsDir() {
				return nil
			}

			if options.IncludeSourceDir && include == "." && relFilePath != "." {
				relFilePath = strings.Join([]string{".", relFilePath}, string(filepath.Separator))
			}

			skip := false

			// If "include" is an exact match for the current file
			// then even if there's an "excludePatterns" pattern that
			// matches it, don't skip it. IOW, assume an explicit 'include'
			// is asking for that file no matter what - which is true
			// for some files, like .dockerignore and Dockerfile (sometimes)
			if include != relFilePath {
				matches, err := pm.IsMatch(relFilePath)
				if err != nil {
					return fmt.Errorf("matching %s: %w", relFilePath, err)
				}
				skip = matches
			}

			if skip {
				// If we want to skip this file and its a directory
				// then we should first check to see if there's an
				// excludes pattern (e.g. !dir/file) that starts with this
				// dir. If so then we can't skip this dir.

				// Its not a dir then so we can just return/skip.
				if !d.IsDir() {
					return nil
				}

				// No exceptions (!...) in patterns so just skip dir
				if !pm.Exclusions() {
					return filepath.SkipDir
				}

				dirSlash := relFilePath + string(filepath.Separator)

				for _, pat := range pm.Patterns() {
					if !pat.Exclusion() {
						continue
					}
					if strings.HasPrefix(pat.String()+string(filepath.Separator), dirSlash) {
						// found a match - so can't skip this dir
						return nil
					}
				}

				// No matching exclusion dir so just skip dir
				return filepath.SkipDir
			}

			if seen[relFilePath] {
				return nil
			}
			seen[relFilePath] = true

			// Rename the base resource.
			if rebaseName != "" {
				var replacement string
				if rebaseName != string(filepath.Separator) {
					// Special case the root directory to replace with an
					// empty string instead so that we don't end up with
					// double slashes in the paths.
					replacement = rebaseName
				}

				relFilePath = strings.Replace(relFilePath, include, replacement, 1)
			}

			headers, err := ta.prepareAddFile(root, fsFilePath, relFilePath)
			if err != nil {
				logrus.Errorf("Can't add file %q in %q to tar: %s; skipping", fsFilePath, root.Name(), err)
			} else if headers != nil {
				if err := ta.addFile(root, headers); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return ta.TarWriter.Close()
}

// TarWithOptions creates an archive from the directory at `path`, only including files whose relative
// paths are included in `options.IncludeFiles` (if non-nil) or not in `options.ExcludePatterns`.
//
// If used on a file system being modified concurrently,
// TarWithOptions will create a valid tar archive, but may leave out
// some files.
func TarWithOptions(srcPath string, options *TarOptions) (io.ReadCloser, error) {
	// WARNING: This is called in contexts where the contents of srcPath may be maliciously
	// concurrently modified.

	pipeReader, pipeWriter := io.Pipe()
	go func() {
		err := tarWithOptionsTo(pipeWriter, srcPath, options)
		if pipeErr := pipeWriter.CloseWithError(err); pipeErr != nil {
			logrus.Errorf("Can't close pipe writer: %s", pipeErr)
		}
	}()

	return pipeReader, nil
}

// Unpack unpacks the decompressedArchive to dest with options.
func Unpack(decompressedArchive io.Reader, dest string, options *TarOptions) error {
	tr := tar.NewReader(decompressedArchive)
	trBuf := pools.BufioReader32KPool.Get(nil)
	defer pools.BufioReader32KPool.Put(trBuf)

	var dirs []*tar.Header
	idMappings := idtools.NewIDMappingsFromMaps(options.UIDMaps, options.GIDMaps)
	rootIDs := idMappings.RootPair()
	whiteoutConverter := GetWhiteoutConverter(options.WhiteoutFormat, options.WhiteoutData)
	buffer := make([]byte, 1<<20)

	doChown := !options.NoLchown
	if options.ForceMask != nil {
		// if ForceMask is in place, make sure lchown is disabled.
		doChown = false
	}
	var rootHdr *tar.Header

	// This is required because path = securejoin.SecureJoin(dest, ...) implicitly Clean()s
	// the result, and we later use a (path == dest) comparison.
	// Alternatively, we could compute filepath.Rel(dest, path) == ".", but that would
	// be more expensive (filepath.Rel starts with two Clean calls).
	dest = filepath.Clean(dest)
	// Iterate through the files in the archive.
loop:
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			// end of tar archive
			break
		}
		if err != nil {
			return err
		}

		// Normalize name. This does NOT allow us to infer useful security properties
		// (does this escape "dest"?) from the string syntax, because the path may include
		// arbitrary (possibly escaping) symlinks.
		//
		// We continue to do this primarily to preserve the semantics of ExcludePatterns.
		// DO NOT add any more code that makes it user-visible whether we Clean hdr.Name;
		// almost all filesystem operations should instead use "path" below.
		//
		// This keeps ".." and "../…" as-is, but normalizes "/../" to "/". Or Windows:
		// This keeps ".." and "..\…" as-is, but normalizes "\..\" to "\".
		hdr.Name = filepath.Clean(hdr.Name)

		for _, exclude := range options.ExcludePatterns {
			if strings.HasPrefix(hdr.Name, exclude) {
				continue loop
			}
		}

		// This check is INSUFFICIENT to detect breakouts if hdr.Name contains symlink parents;
		// we preserve it only to keep the existing restrictions on acceptable inputs.
		insecureRel, err := filepath.Rel(dest, filepath.Join(dest, hdr.Name))
		if err != nil {
			return err
		}
		if insecureRel == ".." || strings.HasPrefix(insecureRel, ".."+string(os.PathSeparator)) {
			return breakoutError(fmt.Errorf("%q is outside of %q", hdr.Name, dest))
		}

		// This does not detect attempts to break out, it just silently restricts them to dest.
		// We could use os.Root to create files instead — that fails on breakout attempts, but
		// Go does not include all operations we need as of Go 1.25, so that would be a larger
		// change — and as of Go 1.26 (which does not use openat2 and the like yet) it would
		// ultimately be more expensive, we would be repeatedly getting a handle to hdrDir in order
		// to make *at syscalls.
		hdrDir, hdrBase, err := createpath.SplitPath(hdr.Name)
		if err != nil {
			return err
		}
		parentPath, err := securejoin.SecureJoin(dest, hdrDir)
		if err != nil {
			return err
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
				return fmt.Errorf("refusing to act on a non-directory entry as the archive root")
			}
		} else {
			// Not the root directory, ensure that the parent directory exists
			if err := fileutils.Lexists(parentPath); err != nil && os.IsNotExist(err) {
				err = idtools.MkdirAllAndChownNew(parentPath, 0o777, rootIDs)
				if err != nil {
					return err
				}
			}
		}

		if path == dest {
			rootHdr = hdr
		}

		// If path exits we almost always just want to remove and replace it
		// The only exception is when it is a directory *and* the file from
		// the layer is also a directory. Then we want to merge them (i.e.
		// just apply the metadata from the layer).
		// (Above, we have already refused to replace all of dest with a non-directory.)
		if fi, err := os.Lstat(path); err == nil {
			if options.NoOverwriteDirNonDir && fi.IsDir() && hdr.Typeflag != tar.TypeDir {
				// If NoOverwriteDirNonDir is true then we cannot replace
				// an existing directory with a non-directory from the archive.
				return overwriteError(fmt.Errorf("cannot overwrite directory %q with non-directory in %q", path, dest))
			}

			if options.NoOverwriteDirNonDir && !fi.IsDir() && hdr.Typeflag == tar.TypeDir {
				// If NoOverwriteDirNonDir is true then we cannot replace
				// an existing non-directory with a directory from the archive.
				return overwriteError(fmt.Errorf("cannot overwrite non-directory %q with directory in %q", path, dest))
			}

			if !fi.IsDir() || hdr.Typeflag != tar.TypeDir {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
			}
		}
		trBuf.Reset(tr)

		chownOpts := options.ChownOpts
		if err := remapIDs(nil, idMappings, chownOpts, hdr); err != nil {
			return err
		}

		if whiteoutConverter != nil && path != dest {
			// The path != dest check guards against a corner case where dest
			// uses a whiteout-related base name.
			// ConvertRead is only allowed to create files within parent(path) and
			// ensures it does not follow symlinks when creating them.
			writeFile, err := whiteoutConverter.ConvertRead(hdr, path)
			if err != nil {
				return err
			}
			if !writeFile {
				continue
			}
		}

		if chownOpts != nil {
			chownOpts = &idtools.IDPair{UID: hdr.Uid, GID: hdr.Gid}
		}

		if err = extractTarFileEntry(path, dest, hdr, trBuf, doChown, chownOpts, options.InUserNS, options.IgnoreChownErrors, options.ForceMask, buffer); err != nil {
			return err
		}

		// Directory mtimes must be handled at the end to avoid further
		// file creation in them to modify the directory mtime
		if hdr.Typeflag == tar.TypeDir {
			dirs = append(dirs, hdr)
		}
	}

	for _, hdr := range dirs {
		// We did create a directory at hdr.Name, but later entries in the tar archive
		// could have replaced hdr.Name or any of its parents with a different file / file kind.
		// So we don’t actually know that hdr.Name refers to a directory; in particular it might
		// be a (possibly escaping) symlink.
		hdrDir, hdrBase, err := createpath.SplitPath(hdr.Name)
		if err != nil {
			return err
		}
		parentPath, err := securejoin.SecureJoin(dest, hdrDir)
		if err != nil {
			return err
		}
		path := filepath.Join(parentPath, hdrBase) // Warning: this can refer to an existing (and escaping) symlink

		fi, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			continue // The directory was replaced; whatever happened here, hdr is no longer relevant.
		}
		if err := system.Chtimes(path, hdr.AccessTime, hdr.ModTime); err != nil { // Note: follows symlinks
			return err
		}
		if err := WriteFileFlagsFromTarHeader(path, hdr); err != nil {
			return err
		}
	}

	if options.ForceMask != nil {
		value := idtools.Stat{Mode: os.ModeDir | os.FileMode(0o755)}
		if rootHdr != nil {
			value.IDs.UID = rootHdr.Uid
			value.IDs.GID = rootHdr.Gid
			value.Mode = os.ModeDir | os.FileMode(rootHdr.Mode)
		}
		if err := idtools.SetContainersOverrideXattr(dest, value); err != nil {
			return err
		}
	}

	return nil
}

// Untar reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
// The archive may be compressed with one of the following algorithms:
//
//	identity (uncompressed), gzip, bzip2, xz.
//
// FIXME: specify behavior when target path exists vs. doesn't exist.
func Untar(tarArchive io.Reader, dest string, options *TarOptions) error {
	return untarHandler(tarArchive, dest, options, true)
}

// UntarUncompressed reads a stream of bytes from `archive`, parses it as a tar archive,
// and unpacks it into the directory at `dest`.
// The archive must be an uncompressed stream.
func UntarUncompressed(tarArchive io.Reader, dest string, options *TarOptions) error {
	return untarHandler(tarArchive, dest, options, false)
}

// Handler for teasing out the automatic decompression
func untarHandler(tarArchive io.Reader, dest string, options *TarOptions, decompress bool) error {
	if tarArchive == nil {
		return fmt.Errorf("empty archive")
	}
	dest = filepath.Clean(dest)
	if options == nil {
		options = &TarOptions{}
	}

	r := tarArchive
	if decompress {
		decompressedArchive, err := DecompressStream(tarArchive)
		if err != nil {
			return err
		}
		defer decompressedArchive.Close()
		r = decompressedArchive
	}

	return Unpack(r, dest, options)
}

// TarUntar is a convenience function which calls Tar and Untar, with the output of one piped into the other.
// If either Tar or Untar fails, TarUntar aborts and returns the error.
func (archiver *Archiver) TarUntar(src, dst string) error {
	logrus.Debugf("TarUntar(%s %s)", src, dst)
	tarMappings := archiver.TarIDMappings
	if tarMappings == nil {
		tarMappings = &idtools.IDMappings{}
	}
	options := &TarOptions{
		UIDMaps:     tarMappings.UIDs(),
		GIDMaps:     tarMappings.GIDs(),
		Compression: Uncompressed,
		CopyPass:    true,
		InUserNS:    unshare.IsRootless(),
	}
	archive, err := TarWithOptions(src, options)
	if err != nil {
		return err
	}
	defer archive.Close()
	untarMappings := archiver.UntarIDMappings
	if untarMappings == nil {
		untarMappings = &idtools.IDMappings{}
	}
	options = &TarOptions{
		UIDMaps:   untarMappings.UIDs(),
		GIDMaps:   untarMappings.GIDs(),
		ChownOpts: archiver.ChownOpts,
		InUserNS:  unshare.IsRootless(),
	}
	return archiver.Untar(archive, dst, options)
}

// UntarPath untar a file from path to a destination, src is the source tar file path.
func (archiver *Archiver) UntarPath(src, dst string) error {
	archive, err := os.Open(src)
	if err != nil {
		return err
	}
	defer archive.Close()
	untarMappings := archiver.UntarIDMappings
	if untarMappings == nil {
		untarMappings = &idtools.IDMappings{}
	}
	options := &TarOptions{
		UIDMaps:   untarMappings.UIDs(),
		GIDMaps:   untarMappings.GIDs(),
		ChownOpts: archiver.ChownOpts,
		InUserNS:  unshare.IsRootless(),
	}
	return archiver.Untar(archive, dst, options)
}

// CopyWithTar creates a tar archive of filesystem path `src`, and
// unpacks it at filesystem path `dst`.
// The archive is streamed directly with fixed buffering and no
// intermediary disk IO.
func (archiver *Archiver) CopyWithTar(src, dst string) error {
	srcSt, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcSt.IsDir() {
		return archiver.CopyFileWithTar(src, dst)
	}

	// if this archiver is set up with ID mapping we need to create
	// the new destination directory with the remapped root UID/GID pair
	// as owner
	rootIDs := archiver.UntarIDMappings.RootPair()
	if archiver.ChownOpts != nil {
		rootIDs = *archiver.ChownOpts
	}
	// Create dst, copy src's content into it
	logrus.Debugf("Creating dest directory: %s", dst)
	if err := idtools.MkdirAllAndChownNew(dst, 0o755, rootIDs); err != nil {
		return err
	}
	logrus.Debugf("Calling TarUntar(%s, %s)", src, dst)
	return archiver.TarUntar(src, dst)
}

// CopyFileWithTar emulates the behavior of the 'cp' command-line
// for a single file. It copies a regular file from path `src` to
// path `dst`, and preserves all its metadata.
func (archiver *Archiver) CopyFileWithTar(src, dst string) (err error) {
	logrus.Debugf("CopyFileWithTar(%s, %s)", src, dst)
	srcSt, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcSt.IsDir() {
		return fmt.Errorf("can't copy a directory")
	}

	// Clean up the trailing slash. This must be done in an operating
	// system specific manner.
	if dst[len(dst)-1] == os.PathSeparator {
		dst = filepath.Join(dst, filepath.Base(src))
	}
	// Create the holding directory if necessary
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}

	r, w := io.Pipe()
	errC := promise.Go(func() error {
		defer w.Close()

		srcF, err := os.Open(src)
		if err != nil {
			return err
		}
		defer srcF.Close()

		hdr, err := tar.FileInfoHeader(srcSt, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.Base(dst)
		hdr.Mode = int64(chmodTarEntry(os.FileMode(hdr.Mode)))
		copyPassHeader(hdr)

		if err := remapIDs(archiver.TarIDMappings, nil, archiver.ChownOpts, hdr); err != nil {
			return err
		}

		tw := tar.NewWriter(w)
		defer tw.Close()
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := io.Copy(tw, srcF); err != nil {
			return err
		}
		return nil
	})
	defer func() {
		if er := <-errC; err == nil && er != nil {
			err = er
		}
	}()

	options := &TarOptions{
		UIDMaps:              archiver.UntarIDMappings.UIDs(),
		GIDMaps:              archiver.UntarIDMappings.GIDs(),
		ChownOpts:            archiver.ChownOpts,
		InUserNS:             unshare.IsRootless(),
		NoOverwriteDirNonDir: true,
	}
	err = archiver.Untar(r, filepath.Dir(dst), options)
	if err != nil {
		r.CloseWithError(err)
	}
	return err
}

func remapIDs(readIDMappings, writeIDMappings *idtools.IDMappings, chownOpts *idtools.IDPair, hdr *tar.Header) (err error) {
	var uid, gid int
	if chownOpts != nil {
		uid, gid = chownOpts.UID, chownOpts.GID
	} else {
		if readIDMappings != nil && !readIDMappings.Empty() {
			uid, gid, err = readIDMappings.ToContainer(idtools.IDPair{UID: hdr.Uid, GID: hdr.Gid})
			if err != nil {
				return err
			}
		} else if runtime.GOOS == darwin {
			uid, gid = hdr.Uid, hdr.Gid
			if xstat, ok := hdr.PAXRecords[PaxSchilyXattr+idtools.ContainersOverrideXattr]; ok {
				attrs := strings.Split(xstat, ":")
				if len(attrs) >= 3 {
					val, err := strconv.ParseUint(attrs[0], 10, 32)
					if err != nil {
						uid = int(val)
					}
					val, err = strconv.ParseUint(attrs[1], 10, 32)
					if err != nil {
						gid = int(val)
					}
				}
			}
		} else {
			uid, gid = hdr.Uid, hdr.Gid
		}
	}
	ids := idtools.IDPair{UID: uid, GID: gid}
	if writeIDMappings != nil && !writeIDMappings.Empty() {
		ids, err = writeIDMappings.ToHost(ids)
		if err != nil {
			return err
		}
	}
	hdr.Uid, hdr.Gid = ids.UID, ids.GID
	return nil
}

// NewTempArchive reads the content of src into a temporary file, and returns the contents
// of that file as an archive. The archive can only be read once - as soon as reading completes,
// the file will be deleted.
func NewTempArchive(src io.Reader, dir string) (*TempArchive, error) {
	f, err := os.CreateTemp(dir, "")
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(f, src); err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := st.Size()
	return &TempArchive{File: f, Size: size}, nil
}

// TempArchive is a temporary archive. The archive can only be read once - as soon as reading completes,
// the file will be deleted.
type TempArchive struct {
	*os.File
	Size   int64 // Pre-computed from Stat().Size() as a convenience
	read   int64
	closed bool
}

// Close closes the underlying file if it's still open, or does a no-op
// to allow callers to try to close the TempArchive multiple times safely.
func (archive *TempArchive) Close() error {
	if archive.closed {
		return nil
	}

	archive.closed = true

	return archive.File.Close()
}

func (archive *TempArchive) Read(data []byte) (int, error) {
	n, err := archive.File.Read(data)
	archive.read += int64(n)
	if err != nil || archive.read == archive.Size {
		archive.Close()
		os.Remove(archive.File.Name())
	}
	return n, err
}

// IsArchive checks for the magic bytes of a tar or any supported compression
// algorithm.
func IsArchive(header []byte) bool {
	compression := DetectCompression(header)
	if compression != Uncompressed {
		return true
	}
	r := tar.NewReader(bytes.NewReader(header))
	_, err := r.Next()
	return err == nil
}

// UntarPath is a convenience function which looks for an archive
// at filesystem path `src`, and unpacks it at `dst`.
func UntarPath(src, dst string) error {
	return NewDefaultArchiver().UntarPath(src, dst)
}

const (
	// HeaderSize is the size in bytes of a tar header
	HeaderSize = 512
)

// NewArchiver returns a new Archiver
func NewArchiver(idMappings *idtools.IDMappings) *Archiver {
	if idMappings == nil {
		idMappings = &idtools.IDMappings{}
	}
	return &Archiver{Untar: Untar, TarIDMappings: idMappings, UntarIDMappings: idMappings}
}

// NewArchiverWithChown returns a new Archiver which uses Untar and the provided ID mapping configuration on both ends
func NewArchiverWithChown(tarIDMappings *idtools.IDMappings, chownOpts *idtools.IDPair, untarIDMappings *idtools.IDMappings) *Archiver {
	if tarIDMappings == nil {
		tarIDMappings = &idtools.IDMappings{}
	}
	if untarIDMappings == nil {
		untarIDMappings = &idtools.IDMappings{}
	}
	return &Archiver{Untar: Untar, TarIDMappings: tarIDMappings, ChownOpts: chownOpts, UntarIDMappings: untarIDMappings}
}

// CopyFileWithTarAndChown returns a function which copies a single file from outside
// of any container into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func CopyFileWithTarAndChown(chownOpts *idtools.IDPair, hasher io.Writer, uidmap []idtools.IDMap, gidmap []idtools.IDMap) func(src, dest string) error {
	untarMappings := idtools.NewIDMappingsFromMaps(uidmap, gidmap)
	archiver := NewArchiverWithChown(nil, chownOpts, untarMappings)
	if hasher != nil {
		originalUntar := archiver.Untar
		archiver.Untar = func(tarArchive io.Reader, dest string, options *TarOptions) error {
			contentReader, contentWriter, err := os.Pipe()
			if err != nil {
				return fmt.Errorf("creating pipe extract data to %q: %w", dest, err)
			}
			defer contentReader.Close()
			defer contentWriter.Close()
			var hashError error
			var hashWorker sync.WaitGroup
			hashWorker.Go(func() {
				t := tar.NewReader(contentReader)
				_, err := t.Next()
				if err != nil {
					hashError = err
				}
				if _, err = io.Copy(hasher, t); err != nil && err != io.EOF {
					hashError = err
				}
			})
			if err = originalUntar(io.TeeReader(tarArchive, contentWriter), dest, options); err != nil {
				err = fmt.Errorf("extracting data to %q while copying: %w", dest, err)
			}
			hashWorker.Wait()
			if err == nil && hashError != nil {
				err = fmt.Errorf("calculating digest of data for %q while copying: %w", dest, hashError)
			}
			return err
		}
	}
	return archiver.CopyFileWithTar
}

// CopyWithTarAndChown returns a function which copies a directory tree from outside of
// any container into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func CopyWithTarAndChown(chownOpts *idtools.IDPair, hasher io.Writer, uidmap []idtools.IDMap, gidmap []idtools.IDMap) func(src, dest string) error {
	untarMappings := idtools.NewIDMappingsFromMaps(uidmap, gidmap)
	archiver := NewArchiverWithChown(nil, chownOpts, untarMappings)
	if hasher != nil {
		originalUntar := archiver.Untar
		archiver.Untar = func(tarArchive io.Reader, dest string, options *TarOptions) error {
			return originalUntar(io.TeeReader(tarArchive, hasher), dest, options)
		}
	}
	return archiver.CopyWithTar
}

// UntarPathAndChown returns a function which extracts an archive in a specified
// location into our working container, mapping permissions using the
// container's ID maps, possibly overridden using the passed-in chownOpts
func UntarPathAndChown(chownOpts *idtools.IDPair, hasher io.Writer, uidmap []idtools.IDMap, gidmap []idtools.IDMap) func(src, dest string) error {
	untarMappings := idtools.NewIDMappingsFromMaps(uidmap, gidmap)
	archiver := NewArchiverWithChown(nil, chownOpts, untarMappings)
	if hasher != nil {
		originalUntar := archiver.Untar
		archiver.Untar = func(tarArchive io.Reader, dest string, options *TarOptions) error {
			return originalUntar(io.TeeReader(tarArchive, hasher), dest, options)
		}
	}
	return archiver.UntarPath
}

// TarPath returns a function which creates an archive of a specified
// location in the container's filesystem, mapping permissions using the
// container's ID maps
func TarPath(uidmap []idtools.IDMap, gidmap []idtools.IDMap) func(path string) (io.ReadCloser, error) {
	tarMappings := idtools.NewIDMappingsFromMaps(uidmap, gidmap)
	return func(path string) (io.ReadCloser, error) {
		return TarWithOptions(path, &TarOptions{
			Compression: Uncompressed,
			UIDMaps:     tarMappings.UIDs(),
			GIDMaps:     tarMappings.GIDs(),
		})
	}
}

// GetOverlayXattrName returns the xattr used by the overlay driver with the
// given name.
// It uses the trusted.overlay prefix when running as root, and user.overlay
// in rootless mode.
func GetOverlayXattrName(name string) string {
	if unshare.IsRootless() {
		return fmt.Sprintf("user.overlay.%s", name)
	}
	return fmt.Sprintf("trusted.overlay.%s", name)
}
