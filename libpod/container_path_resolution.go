//go:build !remote && (linux || freebsd)

package libpod

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod/define"
)

// pathAbs returns an absolute path.  If the specified path is
// relative, it will be resolved relative to the container's working dir.
func (c *Container) pathAbs(path string) string {
	if !filepath.IsAbs(path) {
		// If the containerPath is not absolute, it's relative to the
		// container's working dir.  To be extra careful, let's first
		// join the working dir with "/", and the add the containerPath
		// to it.
		path = filepath.Join(filepath.Join("/", c.WorkingDir()), path)
	}
	return path
}

// pathResolution holds a resolved path and, for a volume subpath, the mount
// that keeps its root stable until the operation is done.
type pathResolution struct {
	root   string
	path   string
	volume *Volume
	mount  *safeMountInfo
}

func (r pathResolution) close() {
	if r.mount != nil {
		r.mount.Close()
	}
}

// resolvePath resolves the container's mount point and the container path as
// specified by the user. Both may resolve outside the container's mount point
// when the path hits a volume or bind mount. The caller must close the result.
func (c *Container) resolvePath(ctx context.Context, mountPoint string, containerPath string) (pathResolution, error) {
	// Let's first make sure we have a path relative to the mount point.
	pathRelativeToContainerMountPoint := c.pathAbs(containerPath)
	resolvedPathOnTheContainerMountPoint := filepath.Join(mountPoint, pathRelativeToContainerMountPoint)
	pathRelativeToContainerMountPoint = strings.TrimPrefix(pathRelativeToContainerMountPoint, mountPoint)
	pathRelativeToContainerMountPoint = filepath.Join("/", pathRelativeToContainerMountPoint)

	// Now we have an "absolute container Path" but not yet resolved on the
	// host (e.g., "/foo/bar/file.txt").  As mentioned above, we need to
	// check if "/foo/bar/file.txt" is on a volume or bind mount.  To do
	// that, we need to walk *down* the paths to the root.  Assuming
	// volume-1 is mounted to "/foo" and volume-2 is mounted to "/foo/bar",
	// we must select "/foo/bar".  Once selected, we need to rebase the
	// remainder (i.e, "/file.txt") on the volume's mount point on the
	// host.  Same applies to bind mounts.

	searchPath := pathRelativeToContainerMountPoint
	for {
		volume, subPath, err := findVolume(ctx, c, searchPath)
		if err != nil {
			return pathResolution{}, err
		}
		if volume != nil {
			logrus.Debugf("Container path %q resolved to volume %q on path %q", containerPath, volume.Name(), searchPath)

			mountPoint, err := volume.MountPoint()
			if err != nil {
				return pathResolution{}, err
			}
			if mountPoint == "" {
				return pathResolution{}, fmt.Errorf("volume %s is not mounted, cannot copy into it", volume.Name())
			}

			var safeMount *safeMountInfo
			if subPath != "" {
				safeMount, err = c.safeMountSubPath(mountPoint, subPath)
				if err != nil {
					return pathResolution{}, err
				}
				mountPoint = safeMount.mountPoint
			}

			// We found a matching volume for searchPath.  We now
			// need to first find the relative path of our input
			// path to the searchPath, and then join it with the
			// volume's mount point.
			pathRelativeToVolume := strings.TrimPrefix(pathRelativeToContainerMountPoint, searchPath)
			absolutePathOnTheVolumeMount, err := securejoin.SecureJoin(mountPoint, pathRelativeToVolume)
			if err != nil {
				if safeMount != nil {
					safeMount.Close()
				}
				return pathResolution{}, err
			}
			return pathResolution{root: mountPoint, path: absolutePathOnTheVolumeMount, volume: volume, mount: safeMount}, nil
		}

		if mount := findBindMount(c, searchPath); mount != nil {
			logrus.Debugf("Container path %q resolved to bind mount %q:%q on path %q", containerPath, mount.Source, mount.Destination, searchPath)
			// We found a matching bind mount for searchPath.  We
			// now need to first find the relative path of our
			// input path to the searchPath, and then join it with
			// the source of the bind mount.
			pathRelativeToBindMount := strings.TrimPrefix(pathRelativeToContainerMountPoint, searchPath)
			absolutePathOnTheBindMount, err := securejoin.SecureJoin(mount.Source, pathRelativeToBindMount)
			if err != nil {
				return pathResolution{}, err
			}
			return pathResolution{root: mount.Source, path: absolutePathOnTheBindMount}, nil
		}

		if searchPath == "/" {
			// Cannot go beyond "/", so we're done.
			break
		}
		// Walk *down* the path (e.g., "/foo/bar/x" -> "/foo/bar").
		searchPath = filepath.Dir(searchPath)
	}

	// No volume, no bind mount but just a normal path on the container.
	return pathResolution{root: mountPoint, path: resolvedPathOnTheContainerMountPoint}, nil
}

// findVolume checks if the specified containerPath matches the destination
// path of a Volume. It returns the matching Volume, its configured subpath, or nil.
func findVolume(ctx context.Context, c *Container, containerPath string) (*Volume, string, error) {
	runtime := c.Runtime()
	cleanedContainerPath := filepath.Clean(containerPath)
	for _, vol := range c.config.NamedVolumes {
		if cleanedContainerPath == filepath.Clean(vol.Dest) {
			volume, err := runtime.GetVolume(ctx, vol.Name)
			return volume, vol.SubPath, err
		}
	}
	return nil, "", nil
}

// isSubDir checks whether path is a subdirectory of root.
func isSubDir(path, root string) bool {
	// check if the specified container path is below a bind mount.
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, "../")
}

// isPathOnVolume returns true if the specified containerPath is a subdir of any
// Volume's destination.
func isPathOnVolume(c *Container, containerPath string) bool {
	cleanedContainerPath := filepath.Clean(containerPath)
	for _, vol := range c.config.NamedVolumes {
		cleanedDestination := filepath.Clean(vol.Dest)
		if cleanedContainerPath == cleanedDestination {
			return true
		}
		if isSubDir(cleanedContainerPath, cleanedDestination) {
			return true
		}
		for dest := cleanedDestination; dest != "/" && dest != "."; dest = filepath.Dir(dest) {
			if cleanedContainerPath == dest {
				return true
			}
		}
	}
	return false
}

// findBindMount checks if the specified containerPath matches the destination
// path of a Mount.  Returns a matching Mount or nil.
func findBindMount(c *Container, containerPath string) *specs.Mount {
	cleanedPath := filepath.Clean(containerPath)
	for _, m := range c.config.Spec.Mounts {
		if m.Type != define.TypeBind {
			continue
		}
		if cleanedPath == filepath.Clean(m.Destination) {
			mount := m
			return &mount
		}
	}
	return nil
}

// isPathOnMount returns true if the specified containerPath is a subdir of any
// Mount's destination.
func isPathOnMount(c *Container, containerPath string) bool {
	cleanedContainerPath := filepath.Clean(containerPath)
	for _, m := range c.config.Spec.Mounts {
		cleanedDestination := filepath.Clean(m.Destination)
		if cleanedContainerPath == cleanedDestination {
			return true
		}
		if isSubDir(cleanedContainerPath, cleanedDestination) {
			return true
		}
		for dest := cleanedDestination; dest != "/" && dest != "."; dest = filepath.Dir(dest) {
			if cleanedContainerPath == dest {
				return true
			}
		}
	}
	return false
}
