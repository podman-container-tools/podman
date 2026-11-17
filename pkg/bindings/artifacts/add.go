package artifacts

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities"
	entitiesTypes "go.podman.io/podman/v6/pkg/domain/entities/types"
)

func Add(ctx context.Context, artifactName string, blobName string, artifactBlob io.Reader, options *AddOptions) (*entitiesTypes.ArtifactAddReport, error) {
	params, err := prepareParams(artifactName, blobName, options)
	if err != nil {
		return nil, err
	}
	return helperAdd(ctx, "/artifacts/add", params, artifactBlob)
}

func AddLocal(ctx context.Context, artifactName string, blobName string, blobPath string, options *AddOptions) (*entitiesTypes.ArtifactAddReport, error) {
	params, err := prepareParams(artifactName, blobName, options)
	if err != nil {
		return nil, err
	}
	params.Set("path", blobPath)
	return helperAdd(ctx, "/artifacts/local/add", params, nil)
}

func prepareParams(name string, fileName string, options *AddOptions) (url.Values, error) {
	if options == nil {
		options = new(AddOptions)
	}

	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	params.Set("name", name)
	params.Set("fileName", fileName)

	return params, nil
}

func helperAdd(ctx context.Context, endpoint string, params url.Values, artifactBlob io.Reader) (*entities.ArtifactAddReport, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	response, err := conn.DoRequest(ctx, artifactBlob, http.MethodPost, endpoint, params, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	var artifactAddReport entities.ArtifactAddReport
	if err := response.Process(&artifactAddReport); err != nil {
		return nil, err
	}

	return &artifactAddReport, nil
}
