package configs

import (
	"context"
	"io"
	"net/http"

	"go.podman.io/podman/v6/pkg/bindings"
	entitiesTypes "go.podman.io/podman/v6/pkg/domain/entities/types"
)

// List returns information about existing configs in the form of a slice.
func List(ctx context.Context, options *ListOptions) ([]*entitiesTypes.ConfigInfoReport, error) {
	var secrs []*entitiesTypes.ConfigInfoReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/configs/json", params, nil)
	if err != nil {
		return secrs, err
	}
	defer response.Body.Close()

	return secrs, response.Process(&secrs)
}

// Inspect returns low-level information about a config.
func Inspect(ctx context.Context, nameOrID string, options *InspectOptions) (*entitiesTypes.ConfigInfoReport, error) {
	if options == nil {
		options = new(InspectOptions)
	}
	var inspect *entitiesTypes.ConfigInfoReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/configs/%s/json", params, nil, nameOrID)
	if err != nil {
		return inspect, err
	}
	defer response.Body.Close()
	return inspect, response.Process(&inspect)
}

// Remove removes a config from storage
func Remove(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodDelete, "/configs/%s", nil, nil, nameOrID)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return response.Process(nil)
}

// Create creates a config given some data
func Create(ctx context.Context, reader io.Reader, options *CreateOptions) (*entitiesTypes.ConfigCreateReport, error) {
	var create *entitiesTypes.ConfigCreateReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, reader, http.MethodPost, "/configs/create", params, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return create, response.Process(&create)
}

func Exists(ctx context.Context, nameOrID string) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/configs/%s/exists", nil, nil, nameOrID)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	return response.IsSuccess(), nil
}
