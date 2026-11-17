package generate

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities/types"
)

func Systemd(ctx context.Context, nameOrID string, options *SystemdOptions) (*types.GenerateSystemdReport, error) {
	if options == nil {
		options = new(SystemdOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/generate/%s/systemd", params, nil, nameOrID)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	report := &types.GenerateSystemdReport{}
	return report, response.Process(&report.Units)
}

// Kube generate Kubernetes YAML (v1 specification)
//
// Note: Caller is responsible for closing returned reader
func Kube(ctx context.Context, nameOrIDs []string, options *KubeOptions) (*types.GenerateKubeReport, error) {
	if options == nil {
		options = new(KubeOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	if len(nameOrIDs) < 1 {
		return nil, errors.New("must provide the name or ID of one container or pod")
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	for _, name := range nameOrIDs {
		params.Add("names", name)
	}
	if options.Replicas != nil {
		params.Set("replicas", strconv.Itoa(int(*options.Replicas)))
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/generate/kube", params, nil)
	if err != nil {
		return nil, err
	}

	if response.StatusCode == http.StatusOK {
		return &types.GenerateKubeReport{Reader: response.Body}, nil
	}

	// Unpack the error.
	return nil, response.Process(nil)
}
