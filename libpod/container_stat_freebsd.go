//go:build !remote

package libpod

import (
	"context"

	"go.podman.io/buildah/copier"
)

// On FreeBSD, jails use the global mount namespace, filtered to only
// the mounts the jail should see. This means that we can use
// statOnHost whether the container is running or not.
// container is running
func (c *Container) statInContainer(ctx context.Context, mountPoint string, containerPath string) (*copier.StatForItem, string, string, error) {
	return c.statOnHost(ctx, mountPoint, containerPath)
}
