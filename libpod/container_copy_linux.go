//go:build !remote

package libpod

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/libpod/define"
	"golang.org/x/sys/unix"
)

// joinMountAndExec executes the specified function `f` inside the container's
// mount and PID namespace.  That allows for having the exact view on the
// container's file system.
//
// Note, if the container is not running `f()` will be executed as is.
func (c *Container) joinMountAndExec(f func() error) error {
	if c.state.State != define.ContainerStateRunning {
		return f()
	}

	// Container's running, so we need to execute `f()` inside its mount NS.
	errChan := make(chan error)
	go func() {
		runtime.LockOSThread()

		// Join the mount and PID NS of the container.
		getFD := func(ns LinuxNS) (*os.File, error) {
			nsPath, err := c.namespacePath(ns)
			if err != nil {
				return nil, err
			}
			return os.Open(nsPath)
		}

		mountFD, err := getFD(MountNS)
		if err != nil {
			errChan <- err
			return
		}
		defer mountFD.Close()

		inHostPidNS, err := c.inHostPidNS()
		if err != nil {
			errChan <- fmt.Errorf("checking inHostPidNS: %w", err)
			return
		}
		var pidFD *os.File
		if !inHostPidNS {
			pidFD, err = getFD(PIDNS)
			if err != nil {
				errChan <- err
				return
			}
			defer pidFD.Close()
		}

		if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
			errChan <- err
			return
		}

		if pidFD != nil {
			if err := unix.Setns(int(pidFD.Fd()), unix.CLONE_NEWPID); err != nil {
				errChan <- err
				return
			}
		}
		if err := unix.Setns(int(mountFD.Fd()), unix.CLONE_NEWNS); err != nil {
			errChan <- err
			return
		}

		// Last but not least, execute the workload.
		errChan <- f()
	}()
	return <-errChan
}

func (c *Container) resolveCopyTarget(mountPoint string, containerPath string) (string, string, *Volume, error) {
	// If the container is running, we will execute the copy
	// inside the container's mount namespace so we return a path
	// relative to the container's root.
	if c.state.State == define.ContainerStateRunning {
		return "/", c.pathAbs(containerPath), nil, nil
	}
	return c.resolvePath(mountPoint, containerPath)
}

type copyMountTarget struct {
	src  string
	dest string
}

// mountContainerVolumesAndMounts bind mounts host source paths for all named volumes
// and bind mounts associated with a stopped container into the container's mountPoint.
// Returns a slice of cleanup functions to unmount the targets in reverse order.
func (c *Container) mountContainerVolumesAndMounts(mountPoint string) ([]func(), error) {
	if c.state.State == define.ContainerStateRunning {
		return nil, nil
	}

	var targets []copyMountTarget
	for _, v := range c.config.NamedVolumes {
		vol, err := c.runtime.state.Volume(v.Name)
		if err != nil {
			continue
		}
		volPoint, err := vol.MountPoint()
		if err != nil || volPoint == "" {
			continue
		}
		targets = append(targets, copyMountTarget{src: volPoint, dest: v.Dest})
	}

	// c.config.Spec.Mounts contains all OCI mounts from the container spec,
	// including user-specified bind mounts. We filter to type "bind" only,
	// skipping kernel pseudo-filesystems (proc, sysfs, tmpfs, devpts, etc.)
	// that do not have meaningful host-side source paths.
	for _, m := range c.config.Spec.Mounts {
		if m.Type != define.TypeBind {
			continue
		}
		if m.Source == "" || m.Destination == "" {
			continue
		}
		targets = append(targets, copyMountTarget{src: m.Source, dest: m.Destination})
	}

	if len(targets) == 0 {
		return nil, nil
	}

	// Sort targets by depth (shorter paths first) so that parent bind mounts
	// are always mounted before nested children.
	sort.Slice(targets, func(i, j int) bool {
		return strings.Count(filepath.Clean(targets[i].dest), "/") <
			strings.Count(filepath.Clean(targets[j].dest), "/")
	})

	var cleanupFuncs []func()
	for _, t := range targets {
		targetInMountPoint, err := securejoin.SecureJoin(mountPoint, t.dest)
		if err != nil {
			continue
		}

		st, err := os.Stat(t.src)
		if err != nil {
			continue
		}

		if st.IsDir() {
			if err := os.MkdirAll(targetInMountPoint, 0o755); err != nil {
				logrus.Debugf("Failed to create directory %s for container copy: %v", targetInMountPoint, err)
				continue
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(targetInMountPoint), 0o755); err != nil {
				logrus.Debugf("Failed to create parent dir for %s for container copy: %v", targetInMountPoint, err)
				continue
			}
			f, err := os.OpenFile(targetInMountPoint, os.O_CREATE|os.O_RDWR, 0o644)
			if err == nil {
				f.Close()
			}
		}

		if err := unix.Mount(t.src, targetInMountPoint, "", unix.MS_BIND|unix.MS_REC, ""); err != nil {
			logrus.Debugf("Failed to bind mount %s to %s for container copy: %v", t.src, targetInMountPoint, err)
			continue
		}

		mntPath := targetInMountPoint
		cleanupFuncs = append(cleanupFuncs, func() {
			if err := unix.Unmount(mntPath, unix.MNT_DETACH); err != nil {
				logrus.Debugf("Failed to unmount %s after container copy: %v", mntPath, err)
			}
		})
	}

	// Unmount in reverse order (deepest paths first) to avoid EBUSY when
	// a nested bind mount sits on top of a shallower one.
	for i, j := 0, len(cleanupFuncs)-1; i < j; i, j = i+1, j-1 {
		cleanupFuncs[i], cleanupFuncs[j] = cleanupFuncs[j], cleanupFuncs[i]
	}

	return cleanupFuncs, nil
}
