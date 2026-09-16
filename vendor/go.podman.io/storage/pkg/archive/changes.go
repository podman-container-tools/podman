package archive

import (
	"archive/tar"
	"bytes"
	"cmp"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/sirupsen/logrus"
	"go.podman.io/storage/pkg/fileutils"
	"go.podman.io/storage/pkg/idtools"
	"go.podman.io/storage/pkg/pools"
	"go.podman.io/storage/pkg/system"
)

// ChangeType represents the change type.
type ChangeType int

const (
	// ChangeModify represents the modify operation.
	ChangeModify = iota
	// ChangeAdd represents the add operation.
	ChangeAdd
	// ChangeDelete represents the delete operation.
	ChangeDelete
)

func (c ChangeType) String() string {
	switch c {
	case ChangeModify:
		return "C"
	case ChangeAdd:
		return "A"
	case ChangeDelete:
		return "D"
	}
	return ""
}

// Change represents a change, it wraps the change type and path.
// It describes changes of the files in the path respect to the
// parent layers. The change could be modify, add, delete.
// This is used for layer diff.
type Change struct {
	Path string
	Kind ChangeType
}

func (change *Change) String() string {
	return fmt.Sprintf("%s %s", change.Kind, change.Path)
}

func compareChangesByPath(a, b Change) int {
	return cmp.Compare(a.Path, b.Path)
}

// Gnu tar and the go tar writer don't have sub-second mtime
// precision, which is problematic when we apply changes via tar
// files, we handle this by comparing for exact times, *or* same
// second count and either a or b having exactly 0 nanoseconds
func sameFsTime(a, b time.Time) bool {
	return a.Equal(b) ||
		(a.Unix() == b.Unix() &&
			(a.Nanosecond() == 0 || b.Nanosecond() == 0))
}

func sameFsTimeSpec(a, b syscall.Timespec) bool {
	return a.Sec == b.Sec &&
		(a.Nsec == b.Nsec || a.Nsec == 0 || b.Nsec == 0)
}

// Changes walks the path rw and determines changes for the files in the path,
// with respect to the parent layers
func Changes(layers []string, rw string) ([]Change, error) {
	return changes(layers, rw, aufsDeletedFile, aufsMetadataSkip, aufsWhiteoutPresent)
}

func aufsMetadataSkip(fsPath string) (bool, error) {
	skip, err := path.Match(WhiteoutMetaPrefix+"*", fsPath)
	if err != nil {
		skip = true
	}
	return skip, err
}

func aufsDeletedFile(root *os.Root, fsPath string, fi os.FileInfo) (string, error) {
	f := path.Base(fsPath)

	// If there is a whiteout, then the file was removed
	if originalFile, ok := strings.CutPrefix(f, WhiteoutPrefix); ok {
		if isInvalidWhiteoutTargetBaseName(originalFile) {
			return "", fmt.Errorf("invalid whiteout path %q", fsPath)
		}
		return path.Join(path.Dir(fsPath), originalFile), nil
	}

	return "", nil
}

func aufsWhiteoutPresent(root, fsPath string) (bool, error) {
	path := filepath.FromSlash(fsPath)
	f := filepath.Join(filepath.Dir(path), WhiteoutPrefix+filepath.Base(path))
	pathInLayer, err := securejoin.SecureJoin(root, f)
	if err != nil {
		return false, err
	}
	err = fileutils.Lexists(pathInLayer)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) || isENOTDIR(err) {
		return false, nil
	}
	return false, err
}

func isENOTDIR(err error) bool {
	if err == nil {
		return false
	}
	if err == syscall.ENOTDIR {
		return true
	}
	if perror, ok := err.(*os.PathError); ok {
		if errno, ok := perror.Err.(syscall.Errno); ok {
			return errno == syscall.ENOTDIR
		}
	}
	return false
}

type (
	skipChange     func(string) (bool, error)
	deleteChange   func(*os.Root, string, os.FileInfo) (string, error)
	whiteoutChange func(string, string) (bool, error)
)

// changes walks the path rw and determines changes for the files in the path,
// with respect to the parent layers.
//
// deleteConverter returns "" for most files; if path (relative to rw, per fs.ValidPath) indicates a deletion,
// it returns the path (relative to rw, per fs.ValidPath) of the file that was deleted.
//
// skipCondition, if not nil, should return true if a file path (relative to rw, per fs.ValidPath) should be skipped.
// It MUST NOT do I/O on the path.
//
// whiteoutChecker, if not nil, returns true if path (relative to rw, per fs.ValidPath) is a whiteout within a layer.
func changes(layers []string, rw string, deleteConverter deleteChange, skipCondition skipChange, whiteoutChecker whiteoutChange) ([]Change, error) {
	// WARNING: This is called in contexts where the contents of rw (but not layers) may be maliciously
	// concurrently modified.
	//
	// In such a situation it’s fine to fail, but we should not expose the rest of the system.
	// Generally, try to read every attribute of a file only once.

	var (
		changes     []Change
		changedDirs = make(map[string]struct{})
	)

	root, err := os.OpenRoot(rw)
	if err != nil {
		return nil, err
	}
	defer root.Close()

	err = fs.WalkDir(root.FS(), ".", func(fsPath string, dirEntry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		f, err := dirEntry.Info()
		if err != nil {
			return err
		}

		// Skip root
		if fsPath == "." {
			return nil
		}

		if skipCondition != nil {
			if skip, err := skipCondition(fsPath); skip {
				return err // FIXME: test coverage
			}
		}

		change := Change{
			Path: filepath.FromSlash("/" + fsPath), // We have skipped ".", and no other fsPath values start with "." or "/", so blindly prepending "/" is safe.
		}

		deletedFile, err := deleteConverter(root, fsPath, f)
		if err != nil {
			return err
		}

		// Find out what kind of modification happened
		if deletedFile != "" {
			change.Path = filepath.FromSlash(path.Join("/", deletedFile))
			change.Kind = ChangeDelete
		} else {
			// Otherwise, the file was added
			change.Kind = ChangeAdd

			// ...Unless it already existed in a top layer, in which case, it's a modification
		layerScan:
			for _, layer := range layers {
				// FIXME: This resolves symlinks in fsPath relative to layer; presumably those symlinks
				// don’t participate in whiteout lookups?!
				//
				// For now, just use securejoin/pathrs to restrict the paths to be within the layer.
				// (We can’t use os.Root because that one fails with an untyped error if it encounters a symlink to a parent to the root,
				// and those symlinks are valid inside containers.)
				// This should, almost certainly, instead do the checks for parent directories
				// in the parent->child order, and stop if it encounters a symlink.
				if whiteoutChecker != nil {
					// ...Unless a lower layer also had whiteout for this directory or one of its parents,
					// in which case, it's new
					ignore, err := whiteoutChecker(layer, fsPath)
					if err != nil {
						return err
					}
					if ignore {
						break layerScan
					}
					for fsDir := path.Dir(fsPath); fsDir != "."; fsDir = path.Dir(fsDir) {
						ignore, err = whiteoutChecker(layer, fsDir)
						if err != nil {
							return err
						}
						if ignore {
							break layerScan
						}
					}
				}
				layerFilepath, err := securejoin.SecureJoin(layer, filepath.FromSlash(fsPath)) // This is expensive but securejoin.pathrs is Linux-specific, and os.Root has non-container semantics on symlinks through /.. .
				if err != nil {
					return err
				}
				stat, err := os.Lstat(layerFilepath)
				if err != nil && !os.IsNotExist(err) {
					return err
				}
				if err == nil {
					// The file existed in the top layer, so that's a modification

					// However, if it's a directory, maybe it wasn't actually modified.
					// If you modify /foo/bar/baz, then /foo will be part of the changed files only because it's the parent of bar
					if stat.IsDir() && f.IsDir() {
						if f.Size() == stat.Size() && f.Mode() == stat.Mode() && sameFsTime(f.ModTime(), stat.ModTime()) {
							// Both directories are the same, don't record the change
							return nil
						}
					}
					change.Kind = ChangeModify
					break
				}
			}
		}

		// If /foo/bar/file.txt is modified, then /foo/bar must be part of the changed files.
		// This block is here to ensure the change is recorded even if the
		// modify time, mode and size of the parent directory in the rw and ro layers are all equal.
		// Check https://github.com/docker/docker/pull/13590 for details.
		if f.IsDir() {
			changedDirs[fsPath] = struct{}{}
		}
		if change.Kind == ChangeAdd || change.Kind == ChangeDelete {
			fsParent := path.Dir(fsPath)
			tail := []Change{}
			for fsParent != "." {
				if _, ok := changedDirs[fsParent]; !ok {
					tail = append([]Change{{Path: filepath.FromSlash("/" + fsParent), Kind: ChangeModify}}, tail...)
					changedDirs[fsParent] = struct{}{}
				}
				fsParent = path.Dir(fsParent)
			}
			changes = append(changes, tail...)
		}

		// Record change
		changes = append(changes, change)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return changes, nil
}

// FileInfo describes the information of a file.
type FileInfo struct {
	parent     *FileInfo
	idMappings *idtools.IDMappings
	name       string
	stat       *system.StatT
	children   map[string]*FileInfo
	capability []byte
	added      bool
	xattrs     map[string]string
	target     string
}

// LookUp looks up the file information of a file.
func (info *FileInfo) LookUp(path string) *FileInfo {
	// As this runs on the daemon side, file paths are OS specific.
	parent := info
	if path == string(os.PathSeparator) {
		return info
	}

	for elem := range strings.SplitSeq(path, string(os.PathSeparator)) {
		if elem != "" {
			child := parent.children[elem]
			if child == nil {
				return nil
			}
			parent = child
		}
	}
	return parent
}

func (info *FileInfo) path() string {
	if info.parent == nil {
		// As this runs on the daemon side, file paths are OS specific.
		return string(os.PathSeparator)
	}
	return filepath.Join(info.parent.path(), info.name)
}

func (info *FileInfo) addChanges(oldInfo *FileInfo, changes *[]Change) {
	sizeAtEntry := len(*changes)

	if oldInfo == nil {
		// add
		change := Change{
			Path: info.path(),
			Kind: ChangeAdd,
		}
		*changes = append(*changes, change)
		info.added = true
	}

	// We make a copy so we can modify it to detect additions
	// also, we only recurse on the old dir if the new info is a directory
	// otherwise any previous delete/change is considered recursive
	oldChildren := make(map[string]*FileInfo)
	if oldInfo != nil && info.isDir() {
		maps.Copy(oldChildren, oldInfo.children)
	}

	for name, newChild := range info.children {
		oldChild := oldChildren[name]
		if oldChild != nil {
			// change?
			oldStat := oldChild.stat
			newStat := newChild.stat
			// Note: We can't compare inode or ctime or blocksize here, because these change
			// when copying a file into a container. However, that is not generally a problem
			// because any content change will change mtime, and any status change should
			// be visible when actually comparing the stat fields. The only time this
			// breaks down is if some code intentionally hides a change by setting
			// back mtime
			if statDifferent(oldStat, oldInfo, newStat, info) ||
				!bytes.Equal(oldChild.capability, newChild.capability) ||
				oldChild.target != newChild.target ||
				!maps.Equal(oldChild.xattrs, newChild.xattrs) {
				change := Change{
					Path: newChild.path(),
					Kind: ChangeModify,
				}
				*changes = append(*changes, change)
				newChild.added = true
			}

			// Remove from copy so we can detect deletions
			delete(oldChildren, name)
		}

		newChild.addChanges(oldChild, changes)
	}
	for _, oldChild := range oldChildren {
		// delete
		change := Change{
			Path: oldChild.path(),
			Kind: ChangeDelete,
		}
		*changes = append(*changes, change)
	}

	// If there were changes inside this directory, we need to add it, even if the directory
	// itself wasn't changed. This is needed to properly save and restore filesystem permissions.
	// As this runs on the daemon side, file paths are OS specific.
	if len(*changes) > sizeAtEntry && info.isDir() && !info.added && info.path() != string(os.PathSeparator) {
		change := Change{
			Path: info.path(),
			Kind: ChangeModify,
		}
		// Let's insert the directory entry before the recently added entries located inside this dir
		*changes = append(*changes, change) // just to resize the slice, will be overwritten
		copy((*changes)[sizeAtEntry+1:], (*changes)[sizeAtEntry:])
		(*changes)[sizeAtEntry] = change
	}
}

// Changes add changes to file information.
func (info *FileInfo) Changes(oldInfo *FileInfo) []Change {
	var changes []Change

	info.addChanges(oldInfo, &changes)

	return changes
}

func newRootFileInfo(idMappings *idtools.IDMappings) *FileInfo {
	// As this runs on the daemon side, file paths are OS specific.
	root := &FileInfo{
		name:       string(os.PathSeparator),
		idMappings: idMappings,
		children:   make(map[string]*FileInfo),
		target:     "",
	}
	return root
}

// ChangesDirs compares two directories and generates an array of Change objects describing the changes.
// If oldDir is "", then all files in newDir will be Add-Changes.
func ChangesDirs(newDir string, newMappings *idtools.IDMappings, oldDir string, oldMappings *idtools.IDMappings) ([]Change, error) {
	// WARNING: This is called in contexts where the contents of newDir (but not oldDir) may be maliciously
	// concurrently modified.

	var oldRoot, newRoot *FileInfo
	if oldDir == "" {
		emptyDir, err := os.MkdirTemp("", "empty")
		if err != nil {
			return nil, err
		}
		defer os.Remove(emptyDir)
		oldDir = emptyDir
	}
	oldRoot, newRoot, err := collectFileInfoForChanges(oldDir, newDir, oldMappings, newMappings)
	if err != nil {
		return nil, err
	}

	return newRoot.Changes(oldRoot), nil
}

// ChangesSizeWithError calculates the size in bytes of the provided changes, based on newDir.
func ChangesSizeWithError(newDir string, changes []Change) (int64, error) {
	// WARNING: This is called in contexts where the contents of newDir may be maliciously
	// concurrently modified.

	root, err := os.OpenRoot(newDir)
	if err != nil {
		return -1, err
	}
	defer root.Close()

	var (
		size int64
		sf   = make(map[uint64]struct{})
	)
	for _, change := range changes {
		if change.Kind == ChangeModify || change.Kind == ChangeAdd {
			fileInfo, err := root.Lstat(strings.TrimPrefix(filepath.ToSlash(change.Path), "/"))
			if err != nil {
				// We don’t _fail_ on these errors, this is intended to be at least minimally-useful
				// with concurrent modifications happening.
				logrus.Errorf("Can not stat %q in %q: %s", change.Path, newDir, err)
				continue
			}

			if fileInfo != nil && !fileInfo.IsDir() {
				if hasHardlinks(fileInfo) {
					inode := getIno(fileInfo)
					if _, ok := sf[inode]; !ok {
						size += fileInfo.Size()
						sf[inode] = struct{}{}
					}
				} else {
					size += fileInfo.Size()
				}
			}
		}
	}
	return size, nil
}

// ChangesSize calculates the size in bytes of the provided changes, based on newDir.
//
// Deprecated: Use ChangesSizeWithError.
func ChangesSize(newDir string, changes []Change) int64 {
	size, err := ChangesSizeWithError(newDir, changes)
	if err != nil {
		return 0
	}
	return size
}

// ExportChanges produces an Archive from the provided changes, relative to dir.
func ExportChanges(dir string, changes []Change, uidMaps, gidMaps []idtools.IDMap) (io.ReadCloser, error) {
	// WARNING: This is called in contexts where the contents of dir may be maliciously
	// concurrently modified.

	reader, writer := io.Pipe()
	go func() {
		ta := newTarWriter(idtools.NewIDMappingsFromMaps(uidMaps, gidMaps), writer, nil, nil, false)

		// this buffer is needed for the duration of this piped stream
		defer pools.BufioWriter32KPool.Put(ta.Buffer)

		slices.SortFunc(changes, compareChangesByPath)

		root, err := os.OpenRoot(dir)
		if err != nil {
			writer.CloseWithError(err)
			return
		}
		defer root.Close()

		// In general we log errors here but ignore them because
		// during e.g. a diff operation the container can continue
		// mutating the filesystem and we can see transient errors
		// from this
		for _, change := range changes {
			if change.Kind == ChangeDelete {
				whiteOutDir := filepath.Dir(change.Path)
				whiteOutBase := filepath.Base(change.Path)
				whiteOut := filepath.Join(whiteOutDir, WhiteoutPrefix+whiteOutBase)
				timestamp := time.Now()
				hdr := &tar.Header{
					Name:       whiteOut[1:],
					Size:       0,
					ModTime:    timestamp,
					AccessTime: timestamp,
					ChangeTime: timestamp,
				}
				if err := ta.TarWriter.WriteHeader(hdr); err != nil {
					logrus.Debugf("Can't write whiteout header: %s", err)
				}
			} else {
				relPath := change.Path[1:]
				headers, err := ta.prepareAddFile(root, filepath.ToSlash(relPath), relPath)
				if err != nil {
					logrus.Debugf("Can't add file %q in %q to tar: %s", change.Path, root.Name(), err)
				} else if headers != nil {
					if err := ta.addFile(root, headers); err != nil {
						writer.CloseWithError(err)
						return
					}
				}
			}
		}

		// Make sure to check the error on Close.
		if err := ta.TarWriter.Close(); err != nil {
			logrus.Debugf("Can't close layer: %s", err)
		}
		if err := writer.Close(); err != nil {
			logrus.Debugf("failed close Changes writer: %s", err)
		}
	}()
	return reader, nil
}
