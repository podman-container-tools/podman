//go:build !remote

package libpod

import "context"

// On FreeBSD, the container's mounts are in the global mount
// namespace so we can just execute the function directly.
func (c *Container) joinMountAndExec(f func() error) error {
	return f()
}

// Similarly, we can just use resolvePath for both running and stopped
// containers.
func (c *Container) resolveCopyTarget(ctx context.Context, mountPoint string, containerPath string) (pathResolution, error) {
	return c.resolvePath(ctx, mountPoint, containerPath)
}
