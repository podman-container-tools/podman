package util //nolint:revive,nolintlint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"go.podman.io/buildah/pkg/parse"
)

// Mirrors path to a tmpfile if path points to a
// file descriptor instead of actual file on filesystem
// reason: operations with file descriptors are can lead
// to edge cases where content on FD is not in a consumable
// state after first consumption.
// returns path as string and bool to confirm if temp file
// was created and needs to be cleaned up.
func MirrorToTempFileIfPathIsDescriptor(file string) (string, bool) {
	// one use-case is discussed here
	// https://github.com/containers/buildah/issues/3070
	if !strings.HasPrefix(file, "/dev/fd/") {
		return file, false
	}
	b, err := os.ReadFile(file)
	if err != nil {
		// if anything goes wrong return original path
		return file, false
	}
	tmpfile, err := os.CreateTemp(parse.GetTempDir(), "buildah-temp-file")
	if err != nil {
		return file, false
	}
	defer tmpfile.Close()
	if _, err := tmpfile.Write(b); err != nil {
		// if anything goes wrong return original path
		return file, false
	}

	return tmpfile.Name(), true
}

// DiscoverContainerfile tries to find a Containerfile or a Dockerfile within the provided `path`.
// The path may be a directory (in which case Containerfile/Dockerfile is searched inside it)
// or a direct path to a container file.
//
// When path is a directory (or a symlink to one), Containerfile/Dockerfile
// candidates are resolved with RESOLVE_IN_ROOT semantics so that symlinks
// escaping the context are clamped back.  When path points directly at a
// file (or a symlink to one), it is returned as-is.
func DiscoverContainerfile(path string) (foundCtrFile string, err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("discovering Containerfile: %w", err)
	}

	target, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("discovering Containerfile: %w", err)
	}

	// If path is a symlink to a directory (e.g. the build context itself is
	// a symlink), follow it so the IsDir() branch handles it.
	if target.Mode()&os.ModeSymlink != 0 {
		if realInfo, err := os.Stat(path); err == nil && realInfo.IsDir() {
			target = realInfo
		}
	}

	switch {
	case target.IsDir():
		for _, name := range []string{"Containerfile", "Dockerfile"} {
			ctrfile := filepath.Join(path, name)
			if resolved, ok := isRegularFileInContext(path, ctrfile); ok {
				return resolved, nil
			}
		}
		return "", fmt.Errorf("cannot find Containerfile or Dockerfile in context directory: %w", fs.ErrNotExist)

	case target.Mode().IsRegular():
		return path, nil

	case target.Mode()&os.ModeSymlink != 0:
		if fi, err := os.Stat(path); err == nil && fi.Mode().IsRegular() {
			return path, nil
		}
		return "", fmt.Errorf("assumed Containerfile %q is not a file", path)

	default:
		return "", fmt.Errorf("assumed Containerfile %q is not a file", path)
	}
}

// isRegularFileInContext checks whether path resolves to a regular file
// inside contextDir using RESOLVE_IN_ROOT semantics (securejoin.SecureJoin):
// ".." components are clamped to the root and absolute symlink targets are
// re-rooted under contextDir.  This matches Docker BuildKit's behavior.
//
// On success it returns the resolved host path.
func isRegularFileInContext(contextDir, path string) (string, bool) {
	name, err := filepath.Rel(contextDir, path)
	if err != nil {
		return "", false
	}
	resolved, err := securejoin.SecureJoin(contextDir, name)
	if err != nil {
		return "", false
	}
	fi, err := os.Stat(resolved)
	if err != nil {
		return "", false
	}
	if !fi.Mode().IsRegular() {
		return "", false
	}
	return resolved, true
}
