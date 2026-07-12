package tunnel

import (
	"context"
	"errors"

	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ic *ContainerEngine) AutoUpdate(_ context.Context, _ entities.AutoUpdateOptions) ([]*entities.AutoUpdateReport, []error) {
	return nil, []error{errors.New("not implemented")}
}
