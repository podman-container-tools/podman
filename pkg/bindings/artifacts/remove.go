package artifacts

import (
	"context"
	"net/http"

	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// Remove removes an artifact from local storage.
func Remove(ctx context.Context, options *RemoveOptions) (*entities.ArtifactRemoveReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodDelete, "/artifacts/remove", params, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var artifactRemoveReport entities.ArtifactRemoveReport
	if err := response.Process(&artifactRemoveReport); err != nil {
		return nil, err
	}

	return &artifactRemoveReport, nil
}
