package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/internal/localapi"
	"go.podman.io/podman/v6/pkg/bindings/artifacts"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

func (ir *ImageEngine) ArtifactExtract(_ context.Context, name string, target string, opts entities.ArtifactExtractOptions) error {
	options := artifacts.ExtractOptions{
		Digest:       &opts.Digest,
		Title:        &opts.Title,
		ExcludeTitle: &opts.ExcludeTitle,
	}

	return artifacts.Extract(ir.ClientCtx, name, target, &options)
}

func (ir *ImageEngine) ArtifactExtractTarStream(_ context.Context, _ io.Writer, _ string, _ entities.ArtifactExtractOptions) error {
	return fmt.Errorf("not implemented")
}

func (ir *ImageEngine) ArtifactInspect(_ context.Context, name string, _ entities.ArtifactInspectOptions) (*entities.ArtifactInspectReport, error) {
	return artifacts.Inspect(ir.ClientCtx, name, &artifacts.InspectOptions{})
}

func (ir *ImageEngine) ArtifactList(_ context.Context, _ entities.ArtifactListOptions) ([]*entities.ArtifactListReport, error) {
	return artifacts.List(ir.ClientCtx, &artifacts.ListOptions{})
}

func (ir *ImageEngine) ArtifactPull(_ context.Context, name string, opts entities.ArtifactPullOptions) (*entities.ArtifactPullReport, error) {
	options := artifacts.PullOptions{
		Username:   &opts.Username,
		Password:   &opts.Password,
		Authfile:   &opts.AuthFilePath,
		Quiet:      &opts.Quiet,
		RetryDelay: &opts.RetryDelay,
		Retry:      opts.MaxRetries,
	}

	switch opts.InsecureSkipTLSVerify {
	case types.OptionalBoolTrue:
		options.WithTlsVerify(false)
	case types.OptionalBoolFalse:
		options.WithTlsVerify(true)
	}

	return artifacts.Pull(ir.ClientCtx, name, &options)
}

func (ir *ImageEngine) ArtifactRm(_ context.Context, opts entities.ArtifactRemoveOptions) (*entities.ArtifactRemoveReport, error) {
	removeOptions := artifacts.RemoveOptions{
		All:       &opts.All,
		Artifacts: opts.Artifacts,
		Ignore:    &opts.Ignore,
	}

	return artifacts.Remove(ir.ClientCtx, &removeOptions)
}

func (ir *ImageEngine) ArtifactPush(_ context.Context, name string, opts entities.ArtifactPushOptions) (*entities.ArtifactPushReport, error) {
	options := artifacts.PushOptions{
		Username:   &opts.Username,
		Password:   &opts.Password,
		Authfile:   &opts.Authfile,
		Quiet:      &opts.Quiet,
		RetryDelay: &opts.RetryDelay,
		Retry:      opts.Retry,
	}

	switch opts.SkipTLSVerify {
	case types.OptionalBoolTrue:
		options.WithTlsVerify(false)
	case types.OptionalBoolFalse:
		options.WithTlsVerify(true)
	}

	return artifacts.Push(ir.ClientCtx, name, &options)
}

func (ir *ImageEngine) ArtifactAdd(_ context.Context, name string, artifactBlob []entities.ArtifactBlob, opts entities.ArtifactAddOptions) (*entities.ArtifactAddReport, error) {
	var artifactAddReport *entities.ArtifactAddReport

	options := artifacts.AddOptions{
		Append:           &opts.Append,
		ArtifactMIMEType: &opts.ArtifactMIMEType,
		FileMIMEType:     &opts.FileMIMEType,
		Replace:          &opts.Replace,
	}

	for k, v := range opts.Annotations {
		options.Annotations = append(options.Annotations, k+"="+v)
	}

	for i, blob := range artifactBlob {
		if i > 0 {
			// When adding more than 1 blob, set append true after the first
			options.WithAppend(true)
		}

		isWSL, err := localapi.IsWSLProvider(ir.ClientCtx)
		if err != nil {
			logrus.Debugf("IsWSLProvider check failed: %v", err)
		}
		if !isWSL {
			if localMap, ok := localapi.CheckPathOnRunningMachine(ir.ClientCtx, blob.BlobFilePath); ok {
				artifactAddReport, err = artifacts.AddLocal(ir.ClientCtx, name, blob.FileName, localMap.RemotePath, &options)
				if err == nil {
					continue
				}
				var errModel *errorhandling.ErrorModel
				if errors.As(err, &errModel) {
					switch errModel.ResponseCode {
					case http.StatusNotFound, http.StatusMethodNotAllowed:
					default:
						return nil, artifactAddErrorCleanup(ir.ClientCtx, i, name, err)
					}
				} else {
					return nil, artifactAddErrorCleanup(ir.ClientCtx, i, name, err)
				}
			}
		}

		artifactAddReport, err = addArtifact(ir.ClientCtx, name, i, blob, &options)
		if err != nil {
			return nil, err
		}
	}
	return artifactAddReport, nil
}

func artifactAddErrorCleanup(ctx context.Context, index int, name string, err error) error {
	if index == 0 {
		return err
	}
	removeOptions := artifacts.RemoveOptions{
		Artifacts: []string{name},
	}
	_, recoverErr := artifacts.Remove(ctx, &removeOptions)
	if recoverErr != nil {
		return fmt.Errorf("failed to cleanup unfinished artifact add: %w", errors.Join(err, recoverErr))
	}
	return err
}

func addArtifact(ctx context.Context, name string, index int, blob entities.ArtifactBlob, options *artifacts.AddOptions) (*entities.ArtifactAddReport, error) {
	f, err := os.Open(blob.BlobFilePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	artifactAddReport, err := artifacts.Add(ctx, name, blob.FileName, f, options)
	if err != nil {
		return nil, artifactAddErrorCleanup(ctx, index, name, err)
	}
	return artifactAddReport, nil
}
