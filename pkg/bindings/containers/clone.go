package containers

import (
	"bytes"
	"context"
	"net/http"

	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities/types"
)

// Clone creates a copy of the given container. Options that are not set keep
// the value of the original container.
func Clone(ctx context.Context, nameOrID string, options *CloneOptions) (*types.ContainerCreateResponse, error) {
	if options == nil {
		options = new(CloneOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(handlers.ContainerCloneEntities(*options))
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, bytes.NewReader(body), http.MethodPost, "/containers/%s/clone", nil, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var report types.ContainerCreateResponse
	if err := response.Process(&report); err != nil {
		return nil, err
	}
	return &report, nil
}
