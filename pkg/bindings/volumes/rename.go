package volumes

import (
	"context"
	"net/http"

	"go.podman.io/podman/v6/pkg/bindings"
)

// Rename an existing volume.
func Rename(ctx context.Context, nameOrID string, options *RenameOptions) error {
	if options == nil {
		options = new(RenameOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}
	params, err := options.ToParams()
	if err != nil {
		return err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodPost, "/volumes/%s/rename", params, nil, nameOrID)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return response.Process(nil)
}
