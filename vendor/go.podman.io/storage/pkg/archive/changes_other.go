//go:build !linux

package archive

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"go.podman.io/storage/internal/rootlookupcache"
	"go.podman.io/storage/internal/stat"
	"go.podman.io/storage/internal/xattrs"
	"go.podman.io/storage/pkg/idtools"
	"go.podman.io/storage/pkg/system"
)

func collectFileInfoForChanges(oldDir, newDir string, oldIDMap, newIDMap *idtools.IDMappings) (*FileInfo, *FileInfo, error) {
	// WARNING: This is called in contexts where the contents of newDir (but not oldDir) may be maliciously
	// concurrently modified.

	var (
		oldRoot, newRoot *FileInfo
		err1, err2       error
		errs             = make(chan error, 2)
	)
	go func() {
		oldRoot, err1 = collectFileInfo(oldDir, oldIDMap)
		errs <- err1
	}()
	go func() {
		newRoot, err2 = collectFileInfo(newDir, newIDMap)
		errs <- err2
	}()

	// block until both routines have returned
	for range 2 {
		if err := <-errs; err != nil {
			return nil, nil, err
		}
	}

	return oldRoot, newRoot, nil
}

func collectFileInfo(sourceDir string, idMappings *idtools.IDMappings) (*FileInfo, error) {
	// WARNING: This is called in contexts where the contents of sourceDir may be maliciously
	// concurrently modified.

	root, err := os.OpenRoot(sourceDir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	rootCache := rootlookupcache.NewCache(root)
	defer rootCache.Close()

	rootFileInfo := newRootFileInfo(idMappings)

	sourceStat, err := system.RootLstat(root, ".")
	if err != nil {
		return nil, err
	}

	err = fs.WalkDir(root.FS(), ".", func(fsPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if fsPath == "." {
			return nil
		}

		// As this runs on the daemon side, file paths are OS specific.
		relPath := filepath.FromSlash("/" + fsPath) // We have skipped ".", and no other fsPath values start with "." or "/", so blindly prepending "/" is safe.

		parent := rootFileInfo.LookUp(filepath.Dir(relPath))
		if parent == nil {
			return fmt.Errorf("collectFileInfo: Unexpectedly no parent for %s", relPath)
		}

		info := &FileInfo{
			name:       filepath.Base(relPath),
			children:   make(map[string]*FileInfo),
			parent:     parent,
			idMappings: idMappings,
		}

		fi, err := d.Info() // This is free and never fails, root.FS().ReadDir() always calls lstatat() to get this data.
		if err != nil {
			return err
		}
		s := stat.FromFileInfo(fi)

		// Don't cross mount points. This ignores file mounts to avoid
		// generating a diff which deletes all files following the
		// mount.
		if s.Dev() != sourceStat.Dev() && s.IsDir() {
			return filepath.SkipDir
		}

		parentRoot, fsBasename, err := rootCache.PreparePath(fsPath)
		if err != nil {
			return err
		}
		info.stat = s
		info.capability, _ = func() ([]byte, error) { // A scope for defer
			h, err := xattrs.NewLHandle(parentRoot, fsBasename, rootCache)
			if err != nil {
				return nil, err
			}
			defer h.Close()
			return h.Getxattr("security.capability")
		}()
		if s.IsSymlink() {
			info.target, err = parentRoot.Readlink(fsBasename)
			if err != nil {
				return err
			}
		}

		parent.children[info.name] = info

		return nil
	})
	if err != nil {
		return nil, err
	}
	return rootFileInfo, nil
}
