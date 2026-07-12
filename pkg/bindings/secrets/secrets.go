package secrets

import (
	"context"
	"io"
	"net/http"

	"go.podman.io/podman/v6/pkg/bindings"
	entitiesTypes "go.podman.io/podman/v6/pkg/domain/entities/types"
)

// List returns information about existing secrets in the form of a slice.
func List(ctx context.Context, options *ListOptions) ([]*entitiesTypes.SecretInfoReport, error) {
	var secrs []*entitiesTypes.SecretInfoReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/secrets/json", params, nil)
	if err != nil {
		return secrs, err
	}
	defer response.Body.Close()

	return secrs, response.Process(&secrs)
}

// Inspect returns low-level information about a secret.
func Inspect(ctx context.Context, nameOrID string, options *InspectOptions) (*entitiesTypes.SecretInfoReport, error) {
	if options == nil {
		options = new(InspectOptions)
	}
	var inspect *entitiesTypes.SecretInfoReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/secrets/%s/json", params, nil, nameOrID)
	if err != nil {
		return inspect, err
	}
	defer response.Body.Close()
	return inspect, response.Process(&inspect)
}

// Remove removes a secret from storage
func Remove(ctx context.Context, nameOrID string) error {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodDelete, "/secrets/%s", nil, nil, nameOrID)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	return response.Process(nil)
}

// Create creates a secret given some data
func Create(ctx context.Context, reader io.Reader, options *CreateOptions) (*entitiesTypes.SecretCreateReport, error) {
	var create *entitiesTypes.SecretCreateReport
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, reader, http.MethodPost, "/secrets/create", params, nil)
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
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/secrets/%s/exists", nil, nil, nameOrID)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	return response.IsSuccess(), nil
}
