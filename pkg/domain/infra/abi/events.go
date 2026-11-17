//go:build !remote && (linux || freebsd)

package abi

import (
	"context"

	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ic *ContainerEngine) Events(ctx context.Context, opts entities.EventsOptions) error {
	readOpts := events.ReadOptions{FromStart: opts.FromStart, Stream: opts.Stream, Filters: opts.Filter, EventChannel: opts.EventChan, Since: opts.Since, Until: opts.Until}
	return ic.Libpod.Events(ctx, readOpts)
}
