package rootlookupcache

import (
	"fmt"
	"os"
	"path"
)

// Cache attempts to speed up repeated path lookups in an os.Root.
//
// As of Go 1.26, operations on os.Root always open a handle for the parent directory of the target
// path before performing the actual operation. That is currently fairly costly, opening a handle
// for each path component individually (e.g. it does not use openat2 on Linux).
//
// So, try to amortize the cost: if the target path is only a basename, os.Root can just do the
// actual operation as a single system call.
// Even with openat2 this would avoid repeated openat2() lookups of the parent.
//
// This is valuable for code needing to do further I/O on files within a fs.WalkDir handler
// (sadly fs.WalkDir does not give us a parent handle, it needs one internally); or for similar
// code that needs to repeatedly make operations within a single directory.
type Cache struct {
	root     *os.Root
	rootFile *os.File // For root; or nil

	// This is a minimal implementation, ideal for fairly shallow directory hierarchies with several files.
	// For a deeper hierarchy, operations on (root/dir/file1, root/dir/subdir1, root/dir/file2, root/dir/subdir2, ...)
	// would, e.g. with fs.WalkDir, result in requests for (root/dir, root/dir/subdir1, root/dir, root/dir/subdir2, ...).
	// That would be better handled by caching a few of the parent directories (but not caching siblings).
	//
	// Also, the cache key is a potentially long string. In situations where we receive arbitrary paths, that’s
	// the best we can do; but callers that implement tree walks manually could also explicitly manage the lifetime
	// / equality of cache values (e.g. while recursing to subdirectories, the caller knows when it returns back to a
	// parent, and could perhaps maintain some kind of a direct reference to a cache entry).
	cachedDirRoot *os.Root // Or nil
	cachedPath    string   // Valid if cachedDirRoot != nil
	cachedDirFile *os.File // For cachedDirRoot, which must be non-nil; or nil
}

// NewCache creates a cache for operations within root.
// The root must be valid until Close() is called.
func NewCache(root *os.Root) *Cache {
	return &Cache{
		root: root,
	}
}

// Close releases any possibly cached handles.
func (c *Cache) Close() error {
	if c.rootFile != nil {
		_ = c.rootFile.Close()
	}
	if c.cachedDirRoot != nil {
		_ = c.cachedDirRoot.Close()
	}
	if c.cachedDirFile != nil {
		_ = c.cachedDirFile.Close()
	}
	return nil
}

// RootForDir returns an *os.Root for a specified directory (a fs.ValidPath) within the cache’s root.
// The returned value can be used only until the next call to RootForDir() or Close();
// the caller must not close it manually.
func (c *Cache) RootForDir(dir string) (*os.Root, error) {
	if dir == "." {
		return c.root, nil
	}

	if c.cachedDirRoot != nil && c.cachedPath == dir {
		return c.cachedDirRoot, nil
	}

	dirRoot, err := c.root.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	if c.cachedDirRoot != nil {
		_ = c.cachedDirRoot.Close()
	}
	if c.cachedDirFile != nil {
		_ = c.cachedDirFile.Close()
		c.cachedDirFile = nil
	}
	c.cachedDirRoot = dirRoot
	c.cachedPath = dir

	return dirRoot, nil
}

// PreparePath returns a parent-directory *os.Root and a basename for efficiently accessing fsPath (per fs.ValidPath),
// while restricting it to the cache’s root.
//
// The returned root can be used only until the next call to RootForDir() or Close();
// the caller must not close it manually.
func (c *Cache) PreparePath(fsPath string) (*os.Root, string, error) {
	parentPath := path.Dir(fsPath)
	parentRoot, err := c.RootForDir(parentPath)
	if err != nil {
		return nil, "", err
	}
	// A path per fs.ValidPath should not contain a ".."; reject it so that we can ensure no escape from parentPath and therefore from c.root.
	basename := path.Base(fsPath)
	if basename == ".." {
		return nil, "", fmt.Errorf("internal error: trailing .. in a path that should conform to fs.ValidPath: %q", fsPath)
	}
	return parentRoot, basename, nil
}

// FileForRoot returns, or newly opens, an *os.File descriptor for reading root (which must have been the last value obtained by RootForDir or PreparePath).
// The returned value can be used only until the next call to RootForDir(), FileForRoot() or Close();
// the caller must not close it manually.
func (c *Cache) FileForRoot(root *os.Root) (*os.File, error) {
	// Ideally, we should be able to use the file descriptor already existing in *os.Root, without the extra Open(".")/Close().

	// We should not really _need_ the special case for c.root, we probably only need one cached *os.File
	// for accessing all files in a directory, and c.root is not really special WRT expected use patterns.
	//
	// But we have the RootForDir(".") special case because it is cheap to implement there, and without a
	// corresponding File cache for "." we’d need a more complex invalidation logic in RootForDir.
	if root == c.root {
		if c.rootFile == nil {
			fd, err := c.root.Open(".")
			if err != nil {
				return nil, err
			}
			c.rootFile = fd
		}
		return c.rootFile, nil
	}

	if c.cachedDirRoot != nil && root == c.cachedDirRoot {
		if c.cachedDirFile == nil {
			fd, err := c.cachedDirRoot.Open(".")
			if err != nil {
				return nil, err
			}
			c.cachedDirFile = fd
		}
		return c.cachedDirFile, nil
	}

	// Otherwise we would need to open a new file descriptor, and then we would need to close it at some point,
	// i.e. we would need to be tracking it in the cache.
	// That could work under the assumption that the caller is following the cache rules, but under that assumption,
	// we should never get here.
	return nil, fmt.Errorf("internal error: FileForRoot called with an unexpected root")
}
