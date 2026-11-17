package tunnel

import (
	"context"

	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/bindings/containers"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ic *ContainerEngine) HealthCheckRun(_ context.Context, nameOrID string, _ entities.HealthCheckOptions) (*define.HealthCheckResults, error) {
	return containers.RunHealthCheck(ic.ClientCtx, nameOrID, nil)
}
