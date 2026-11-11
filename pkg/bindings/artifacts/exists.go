package artifacts

import (
	"context"
	"net/http"

	"go.podman.io/podman/v6/pkg/bindings"
)

// Exists returns true if a given artifact exists in local storage
func Exists(ctx context.Context, nameOrID string, _ *ExistsOptions) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/artifacts/%s/exists", nil, nil, nameOrID)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	if response.IsSuccess() {
		return true, nil
	}
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	return false, response.Process(nil)
}
