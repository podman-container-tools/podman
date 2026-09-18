//go:build !remote

package libpod

import (
	"context"

	"go.podman.io/buildah/copier"
	"go.podman.io/podman/v6/libpod/define"
)

// statInsideMount stats the specified path *inside* the container's mount and PID
// namespace.  It returns the file info along with the resolved root ("/") and
// the resolved path (relative to the root).
func (c *Container) statInsideMount(ctx context.Context, containerPath string) (*copier.StatForItem, string, string, error) {
	resolvedRoot := "/"
	resolvedPath := c.pathAbs(containerPath)
	var statInfo *copier.StatForItem

	err := c.joinMountAndExec(
		func() error {
			var statErr error
			statInfo, statErr = secureStat(ctx, resolvedRoot, resolvedPath)
			return statErr
		},
	)

	return statInfo, resolvedRoot, resolvedPath, err
}

// Calls either statOnHost or statInsideMount depending on whether the
// container is running
func (c *Container) statInContainer(ctx context.Context, mountPoint string, containerPath string) (*copier.StatForItem, string, string, error) {
	if c.state.State == define.ContainerStateRunning {
		// If the container is running, we need to join it's mount namespace
		// and stat there.
		return c.statInsideMount(ctx, containerPath)
	}
	// If the container is NOT running, we need to resolve the path
	// on the host.
	return c.statOnHost(ctx, mountPoint, containerPath)
}
