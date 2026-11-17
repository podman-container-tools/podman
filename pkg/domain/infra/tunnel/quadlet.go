package tunnel

import (
	"context"
	"errors"

	"go.podman.io/podman/v6/pkg/domain/entities"
)

var errNotImplemented = errors.New("not implemented for the remote Podman client")

func (ic *ContainerEngine) QuadletExists(_ context.Context, _ string) (*entities.BoolReport, error) {
	return nil, errNotImplemented
}

func (ic *ContainerEngine) QuadletInstall(_ context.Context, _ []string, _ entities.QuadletInstallOptions) (*entities.QuadletInstallReport, error) {
	return nil, errNotImplemented
}

func (ic *ContainerEngine) QuadletList(_ context.Context, _ entities.QuadletListOptions) ([]*entities.ListQuadlet, error) {
	return nil, errNotImplemented
}

func (ic *ContainerEngine) QuadletPrint(_ context.Context, _ string) (string, error) {
	return "", errNotImplemented
}

func (ic *ContainerEngine) QuadletRemove(_ context.Context, _ []string, _ entities.QuadletRemoveOptions) (*entities.QuadletRemoveReport, error) {
	return nil, errNotImplemented
}
