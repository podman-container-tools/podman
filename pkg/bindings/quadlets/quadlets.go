package quadlets

import (
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// List returns a list of quadlets on the server.
func List(ctx context.Context, options *ListOptions) ([]*entities.ListQuadlet, error) {
	if options == nil {
		options = new(ListOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	var quadlets []*entities.ListQuadlet
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/quadlets/json", params, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return quadlets, response.Process(&quadlets)
}

// Exists checks whether a quadlet with the given name exists on the server.
func Exists(ctx context.Context, name string, _ *ExistsOptions) (bool, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return false, err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/quadlets/%s/exists", nil, nil, name)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()

	return response.IsSuccess(), nil
}

// Print returns the contents of a quadlet file from the server.
func Print(ctx context.Context, name string, _ *PrintOptions) (string, error) {
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return "", err
	}
	response, err := conn.DoRequest(ctx, nil, http.MethodGet, "/quadlets/%s/file", nil, nil, name)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	if !response.IsSuccess() {
		return "", response.Process(nil)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// Install sends local quadlet files to the server for installation via multipart upload.
func Install(ctx context.Context, filePaths []string, options *InstallOptions) (*entities.QuadletInstallReport, error) {
	if options == nil {
		options = new(InstallOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer writer.Close()

		for _, filePath := range filePaths {
			filename := filepath.Base(filePath)
			part, err := writer.CreateFormFile(filename, filename)
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			file, err := os.Open(filePath)
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			_, err = io.Copy(part, file)
			file.Close()
			if err != nil {
				pw.CloseWithError(err)
				return
			}
		}
	}()

	header := make(http.Header)
	header.Set("Content-Type", writer.FormDataContentType())

	var report entities.QuadletInstallReport
	response, err := conn.DoRequest(ctx, pr, http.MethodPost, "/quadlets", params, header)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &report, response.Process(&report)
}

// Remove removes one or more quadlets from the server (batch operation).
func Remove(ctx context.Context, quadletNames []string, options *RemoveOptions) (*entities.QuadletRemoveReport, error) {
	if options == nil {
		options = new(RemoveOptions)
	}
	conn, err := bindings.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	params, err := options.ToParams()
	if err != nil {
		return nil, err
	}
	for _, name := range quadletNames {
		params.Add("quadlets", name)
	}

	var report entities.QuadletRemoveReport
	response, err := conn.DoRequest(ctx, nil, http.MethodDelete, "/quadlets", params, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	return &report, response.Process(&report)
}
