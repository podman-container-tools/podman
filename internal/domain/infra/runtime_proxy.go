//go:build !remote && (linux || freebsd)

package infra

import (
	"context"

	flag "github.com/spf13/pflag"
	ientities "go.podman.io/podman/v6/internal/domain/entities"
	"go.podman.io/podman/v6/internal/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra"
	"go.podman.io/storage"
)

func NewLibpodTestingRuntime(flags *flag.FlagSet, opts *entities.PodmanConfig) (ientities.TestingEngine, error) {
	r, err := infra.GetRuntime(context.Background(), flags, opts)
	if err != nil {
		return nil, err
	}
	store, err := storage.GetStore(r.StorageConfig())
	if err != nil {
		return nil, err
	}
	return &abi.TestingEngine{Libpod: r, Store: store}, nil
}
