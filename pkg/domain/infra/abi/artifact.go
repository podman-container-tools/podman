//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/libartifact"
	"go.podman.io/common/pkg/libartifact/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func (ir *ImageEngine) ArtifactInspect(ctx context.Context, name string, _ entities.ArtifactInspectOptions) (*entities.ArtifactInspectReport, error) {
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}
	asr, err := libartifact.NewArtifactStorageReference(name)
	if err != nil {
		return nil, err
	}
	art, err := artStore.Inspect(ctx, asr)
	if err != nil {
		return nil, err
	}
	artInspectReport := entities.ArtifactInspectReport{
		Artifact: art,
		Digest:   art.Digest.String(),
	}
	return &artInspectReport, nil
}

func (ir *ImageEngine) ArtifactList(ctx context.Context, _ entities.ArtifactListOptions) ([]*entities.ArtifactListReport, error) {
	reports := make([]*entities.ArtifactListReport, 0)
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}
	lrs, err := artStore.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, lr := range lrs {
		artListReport := entities.ArtifactListReport{
			Artifact: lr,
		}
		reports = append(reports, &artListReport)
	}
	return reports, nil
}

func (ir *ImageEngine) ArtifactPull(ctx context.Context, name string, opts entities.ArtifactPullOptions) (*entities.ArtifactPullReport, error) {
	artRefToPull, err := libartifact.NewArtifactReference(name)
	if err != nil {
		return nil, err
	}

	pullOptions := &libimage.CopyOptions{}
	pullOptions.AuthFilePath = opts.AuthFilePath
	pullOptions.CertDirPath = opts.CertDirPath
	pullOptions.Username = opts.Username
	pullOptions.Password = opts.Password
	pullOptions.SignaturePolicyPath = opts.SignaturePolicyPath
	pullOptions.InsecureSkipTLSVerify = opts.InsecureSkipTLSVerify
	pullOptions.Writer = opts.Writer
	pullOptions.OciDecryptConfig = opts.OciDecryptConfig
	pullOptions.MaxRetries = opts.MaxRetries
	if opts.RetryDelay != "" {
		duration, err := time.ParseDuration(opts.RetryDelay)
		if err != nil {
			return nil, fmt.Errorf("unable to parse value provided %q: %w", opts.RetryDelay, err)
		}
		pullOptions.RetryDelay = &duration
	}

	if !opts.Quiet && pullOptions.Writer == nil {
		pullOptions.Writer = os.Stderr
	}
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}
	artifactDigest, err := artStore.Pull(ctx, artRefToPull, *pullOptions)
	if err != nil {
		return nil, err
	}

	return &entities.ArtifactPullReport{
		ArtifactDigest: &artifactDigest,
	}, nil
}

func (ir *ImageEngine) ArtifactRm(ctx context.Context, opts entities.ArtifactRemoveOptions) (*entities.ArtifactRemoveReport, error) {
	var namesOrDigests []string
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}

	if opts.All {
		allArtifacts, err := artStore.List(ctx)
		if err != nil {
			return nil, err
		}
		for _, art := range allArtifacts {
			// Using the digest here instead of name to protect against
			// an artifact that lacks a name
			namesOrDigests = append(namesOrDigests, art.Digest.Encoded())
		}
	}

	// NOTE: If opts.All is true, len(opts.Artifacts) will == 0
	if len(opts.Artifacts) != 0 {
		namesOrDigests = append(namesOrDigests, opts.Artifacts...)
	}

	artifactDigests := make([]*digest.Digest, 0, len(namesOrDigests))
	for _, nameOrDigest := range namesOrDigests {
		asr, err := libartifact.NewArtifactStorageReference(nameOrDigest)
		if err != nil {
			return nil, err
		}

		artifactDigest, err := artStore.Remove(ctx, asr)
		if err != nil {
			if opts.Ignore && errors.Is(err, types.ErrArtifactNotExist) {
				logrus.Debugf("Artifact with name or digest %q does not exist, ignoring error as request", nameOrDigest)
				continue
			}
			return nil, err
		}
		artifactDigests = append(artifactDigests, artifactDigest)
	}
	artifactRemoveReport := entities.ArtifactRemoveReport{
		ArtifactDigests: artifactDigests,
	}
	return &artifactRemoveReport, err
}

func (ir *ImageEngine) ArtifactPush(ctx context.Context, name string, opts entities.ArtifactPushOptions) (*entities.ArtifactPushReport, error) {
	var retryDelay *time.Duration

	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}

	if opts.RetryDelay != "" {
		rd, err := time.ParseDuration(opts.RetryDelay)
		if err != nil {
			return nil, err
		}
		retryDelay = &rd
	}

	copyOpts := libimage.CopyOptions{
		SourceLookupReferenceFunc:        nil,
		DestinationLookupReferenceFunc:   nil,
		CompressionFormat:                nil,
		CompressionLevel:                 nil,
		ForceCompressionFormat:           false,
		AuthFilePath:                     opts.Authfile,
		BlobInfoCacheDirPath:             "",
		CertDirPath:                      opts.CertDir,
		DirForceCompress:                 false,
		ImageListSelection:               0,
		InsecureSkipTLSVerify:            opts.SkipTLSVerify,
		MaxRetries:                       opts.Retry,
		RetryDelay:                       retryDelay,
		ManifestMIMEType:                 "",
		OciAcceptUncompressedLayers:      false,
		OciEncryptConfig:                 nil,
		OciEncryptLayers:                 opts.OciEncryptLayers,
		OciDecryptConfig:                 nil,
		Progress:                         nil,
		PolicyAllowStorage:               false,
		SignaturePolicyPath:              opts.SignaturePolicy,
		Signers:                          opts.Signers,
		SignBy:                           opts.SignBy,
		SignPassphrase:                   opts.SignPassphrase,
		SignBySigstorePrivateKeyFile:     opts.SignBySigstorePrivateKeyFile,
		SignSigstorePrivateKeyPassphrase: opts.SignSigstorePrivateKeyPassphrase,
		RemoveSignatures:                 opts.RemoveSignatures,
		Architecture:                     "",
		OS:                               "",
		Variant:                          "",
		Username:                         opts.Username,
		Password:                         opts.Password,
		IdentityToken:                    "",
		Writer:                           opts.Writer,
	}
	artRef, err := libartifact.NewArtifactReference(name)
	if err != nil {
		return nil, err
	}
	artifactDigest, err := artStore.Push(ctx, artRef, artRef, copyOpts)
	if err != nil {
		return nil, err
	}

	return &entities.ArtifactPushReport{
		ArtifactDigest: &artifactDigest,
	}, nil
}

func (ir *ImageEngine) ArtifactAdd(ctx context.Context, name string, artifactBlobs []entities.ArtifactBlob, opts entities.ArtifactAddOptions) (*entities.ArtifactAddReport, error) {
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return nil, err
	}

	artToAdd, err := libartifact.NewArtifactReference(name)
	if err != nil {
		return nil, err
	}
	// If replace is true, try to remove existing artifact (ignore errors if it doesn't exist)
	if opts.Replace {
		if _, err = artStore.Remove(ctx, artToAdd.ToArtifactStoreReference()); err != nil && !errors.Is(err, types.ErrArtifactNotExist) {
			logrus.Debugf("Artifact %q removal failed: %s", name, err)
		}
	}

	addOptions := types.AddOptions{
		Annotations:      opts.Annotations,
		ArtifactMIMEType: opts.ArtifactMIMEType,
		Append:           opts.Append,
		FileMIMEType:     opts.FileMIMEType,
		Replace:          opts.Replace,
	}

	artifactDigest, err := artStore.Add(ctx, artToAdd, artifactBlobs, &addOptions)
	if err != nil {
		return nil, err
	}
	return &entities.ArtifactAddReport{
		ArtifactDigest: artifactDigest,
	}, nil
}

func (ir *ImageEngine) ArtifactExtract(ctx context.Context, name string, target string, opts entities.ArtifactExtractOptions) error {
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return err
	}
	extractOpt := types.ExtractOptions{
		FilterBlobOptions: types.FilterBlobOptions{
			Digest: opts.Digest,
			Title:  opts.Title,
		},
	}

	asr, err := libartifact.NewArtifactStorageReference(name)
	if err != nil {
		return err
	}
	return artStore.Extract(ctx, asr, target, &extractOpt)
}

func (ir *ImageEngine) ArtifactExtractTarStream(ctx context.Context, w io.Writer, name string, opts entities.ArtifactExtractOptions) error {
	artStore, err := ir.Libpod.ArtifactStore()
	if err != nil {
		return err
	}
	extractOpt := types.ExtractOptions{
		FilterBlobOptions: types.FilterBlobOptions{
			Digest: opts.Digest,
			Title:  opts.Title,
		},
		ExcludeTitle: opts.ExcludeTitle,
	}

	asr, err := libartifact.NewArtifactStorageReference(name)
	if err != nil {
		return err
	}
	return artStore.ExtractTarStream(ctx, w, asr, &extractOpt)
}
