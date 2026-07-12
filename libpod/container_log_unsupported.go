//go:build !remote && ((linux && !systemd) || freebsd)

package libpod

import (
	"context"
	"fmt"

	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/logs"
)

func (c *Container) readFromJournal(_ context.Context, _ *logs.LogOptions, _ chan *logs.LogLine, _ int64, _ string) error {
	return fmt.Errorf("journald logging only enabled with systemd on linux: %w", define.ErrOSNotSupported)
}
