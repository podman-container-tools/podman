//go:build !remote && (linux || freebsd)

package libpod

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.podman.io/buildah/copier"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/copy"
)

// statOnHost stats the specified path *on the host*.  It returns the file info
// along with the resolved root and the resolved path.  Both paths are absolute
// to the host's root.  Note that the paths may resolved outside the
// container's mount point (e.g., to a volume or bind mount).
func (c *Container) statOnHost(ctx context.Context, mountPoint string, containerPath string) (*copier.StatForItem, pathResolution, error) {
	return c.statOnHostWithDepth(ctx, mountPoint, containerPath, 0)
}

func (c *Container) statOnHostWithDepth(ctx context.Context, mountPoint string, containerPath string, depth int) (*copier.StatForItem, pathResolution, error) {
	if depth > 255 {
		return nil, pathResolution{}, fmt.Errorf("too many symlinks resolving %q", containerPath)
	}

	// Now resolve the container's path.  It may hit a volume, it may hit a
	// bind mount, it may be relative.
	resolved, err := c.resolvePath(ctx, mountPoint, containerPath)
	if err != nil {
		return nil, pathResolution{}, err
	}
	if resolved.imageVolume != nil {
		target, err := imageMountLinkTarget(resolved.root, resolved.imageVolume.Dest, c.pathAbs(containerPath))
		if err != nil {
			return nil, resolved, err
		}
		if target != "" {
			statInfo, targetResolution, err := c.statOnHostWithDepth(ctx, mountPoint, target, depth+1)
			targetResolution.close()
			if err != nil {
				return nil, resolved, err
			}
			statInfo.IsSymlink = true
			statInfo.ImmediateTarget = target
			return statInfo, resolved, nil
		}
	}

	statInfo, err := secureStat(ctx, resolved.root, resolved.path)
	return statInfo, resolved, err
}

// imageMountLinkTarget resolves a link in an image mount using container paths.
// Absolute links in the mount refer to the container root, while relative links
// are based at the link's location in the container.
func imageMountLinkTarget(root, destination, containerPath string) (string, error) {
	relative, err := filepath.Rel(destination, containerPath)
	if err != nil || relative == "." {
		return "", err
	}
	parts := strings.Split(relative, string(os.PathSeparator))
	current := root
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		link, err := os.Readlink(current)
		if err != nil {
			return "", err
		}
		linkPath := filepath.Join(destination, filepath.Join(parts[:index+1]...))
		if !filepath.IsAbs(link) {
			link = filepath.Join(filepath.Dir(linkPath), link)
		}
		return filepath.Join(append([]string{link}, parts[index+1:]...)...), nil
	}
	return "", nil
}

func (c *Container) stat(ctx context.Context, containerMountPoint string, containerPath string) (*define.FileInfo, pathResolution, error) {
	var (
		resolved         pathResolution
		absContainerPath string
		statInfo         *copier.StatForItem
		statErr          error
	)

	// Make sure that "/" copies the *contents* of the mount point and not
	// the directory.
	if containerPath == "/" {
		containerPath = "/."
	}

	// Wildcards are not allowed.
	// TODO: it's now technically possible wildcards.
	// We may consider enabling support in the future.
	if strings.Contains(containerPath, "*") {
		return nil, pathResolution{}, copy.ErrENOENT
	}

	statInfo, resolved, statErr = c.statInContainer(ctx, containerMountPoint, containerPath)
	if statErr != nil {
		if statInfo == nil {
			return nil, resolved, statErr
		}
		// Not all errors from secureStat map to ErrNotExist, so we
		// have to look into the error string.  Turning it into an
		// ENOENT lets the API handlers return the correct status code
		// which is crucial for the remote client.
		if errors.Is(statErr, os.ErrNotExist) || strings.Contains(statErr.Error(), "o such file or directory") {
			statErr = copy.ErrENOENT
		}
	}

	switch {
	case statInfo.IsSymlink:
		// Symlinks are already evaluated and always relative to the
		// container's mount point.
		absContainerPath = statInfo.ImmediateTarget
	case strings.HasPrefix(resolved.path, containerMountPoint):
		// If the path is on the container's mount point, strip it off.
		absContainerPath = strings.TrimPrefix(resolved.path, containerMountPoint)
		absContainerPath = filepath.Join("/", absContainerPath)
	default:
		// No symlink and not on the container's mount point, so let's
		// move it back to the original input.  It must have evaluated
		// to a volume or bind mount but we cannot return host paths.
		absContainerPath = containerPath
	}

	// Preserve the base path as specified by the user.  The `filepath`
	// packages likes to remove trailing slashes and dots that are crucial
	// to the copy logic.
	absContainerPath = copy.PreserveBasePath(containerPath, absContainerPath)
	resolved.path = copy.PreserveBasePath(containerPath, resolved.path)

	info := &define.FileInfo{
		IsDir:      statInfo.IsDir,
		Name:       filepath.Base(absContainerPath),
		Size:       statInfo.Size,
		Mode:       statInfo.Mode,
		ModTime:    statInfo.ModTime,
		LinkTarget: absContainerPath,
	}

	return info, resolved, statErr
}

// secureStat extracts file info for path in a chroot'ed environment in root.
func secureStat(ctx context.Context, root string, path string) (*copier.StatForItem, error) {
	var glob string
	var err error

	// If root and path are equal, then dir must be empty and the glob must
	// be ".".
	if filepath.Clean(root) == filepath.Clean(path) {
		glob = "."
	} else {
		glob, err = filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
	}

	globStats, err := copier.StatContext(ctx, root, "", copier.StatOptions{}, []string{glob})
	if err != nil {
		return nil, err
	}

	if len(globStats) != 1 {
		return nil, fmt.Errorf("internal error: secureStat: expected 1 item but got %d", len(globStats))
	}
	if len(globStats) != 1 {
		return nil, fmt.Errorf("internal error: secureStat: expected 1 result but got %d", len(globStats[0].Results))
	}

	// NOTE: the key in the map differ from `glob` when hitting symlink.
	// Hence, we just take the first (and only) key/value pair.
	for _, stat := range globStats[0].Results {
		var statErr error
		if stat.Error != "" {
			statErr = errors.New(stat.Error)
		}
		// If necessary evaluate the symlink
		if stat.IsSymlink {
			target, err := copier.EvalContext(ctx, root, path, copier.EvalOptions{})
			if err != nil {
				return nil, fmt.Errorf("evaluating symlink in container: %w", err)
			}
			// Need to make sure the symlink is relative to the root!
			target = strings.TrimPrefix(target, root)
			target = filepath.Join("/", target)
			stat.ImmediateTarget = target
		}
		return stat, statErr
	}

	// Nothing found!
	return nil, copy.ErrENOENT
}
