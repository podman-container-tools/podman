//go:build !remote && (linux || freebsd)

package infra

import (
	"context"

	flag "github.com/spf13/pflag"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
)

// ContainerEngine Proxy will be EOL'ed after podman is separated from libpod repo

func NewLibpodRuntime(ctx context.Context, flags *flag.FlagSet, opts *entities.PodmanConfig) (entities.ContainerEngine, error) {
	r, err := GetRuntime(ctx, flags, opts)
	if err != nil {
		return nil, err
	}
	return &abi.ContainerEngine{Libpod: r}, nil
}

func NewLibpodImageRuntime(ctx context.Context, flags *flag.FlagSet, opts *entities.PodmanConfig) (entities.ImageEngine, error) {
	r, err := GetRuntime(ctx, flags, opts)
	if err != nil {
		return nil, err
	}
	return &abi.ImageEngine{Libpod: r}, nil
}
