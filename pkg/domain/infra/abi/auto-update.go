//go:build !remote && (linux || freebsd)

package abi

import (
	"context"

	"go.podman.io/podman/v6/pkg/autoupdate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ic *ContainerEngine) AutoUpdate(ctx context.Context, options entities.AutoUpdateOptions) ([]*entities.AutoUpdateReport, []error) {
	return autoupdate.AutoUpdate(ctx, ic.Libpod, options)
}
