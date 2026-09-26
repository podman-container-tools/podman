//go:build !linux && !darwin && !freebsd

package buildah

import (
	"context"
	"errors"

	nettypes "go.podman.io/common/libnetwork/types"
	"go.podman.io/storage"
)

func setChildProcess() error {
	return errors.New("function not supported on non-linux systems")
}

func runUsingRuntimeMain() {}

func (b *Builder) RunContext(ctx context.Context, command []string, options RunOptions) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return errors.New("function not supported on non-linux systems")
}

func DefaultNamespaceOptions() (NamespaceOptions, error) {
	return NamespaceOptions{}, errors.New("function not supported on non-linux systems")
}

// getNetworkInterface creates the network interface
func getNetworkInterface(store storage.Store) (nettypes.ContainerNetwork, error) {
	return nil, errors.New("function not supported on non-linux systems")
}
