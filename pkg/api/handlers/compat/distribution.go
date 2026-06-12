//go:build !remote

package compat

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/hashicorp/go-multierror"
	dockerRegistry "github.com/moby/moby/api/types/registry"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/image"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/pkg/shortnames"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/auth"
)

func DistributionInspect(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	imageName := utils.GetName(r)

	normalizedImageName, err := utils.NormalizeToDockerHub(r, imageName)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	if _, err := reference.ParseNormalizedNamed(normalizedImageName); err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	sys := runtime.SystemContext()
	sys.AuthFilePath = authfile
	sys.DockerAuthConfig = authConf

	resolved, err := shortnames.Resolve(sys, normalizedImageName)
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	var merr *multierror.Error

	for _, candidate := range resolved.PullCandidates {
		inspect, err := inspectCandidate(r.Context(), sys, candidate.Value)
		if err != nil {
			merr = multierror.Append(merr, fmt.Errorf("%s: %w", candidate.Value.String(), err))
			continue
		}

		utils.WriteResponse(w, http.StatusOK, inspect)
		return
	}

	combinedErr := merr.ErrorOrNil()
	if combinedErr == nil {
		utils.InternalServerError(w, errors.New("no candidate succeeded but got no error"))
		return
	}

	var registryErrCode errcode.ErrorCoder
	if errors.As(combinedErr, &registryErrCode) {
		switch registryErrCode.ErrorCode().Descriptor().HTTPStatusCode {
		case http.StatusUnauthorized:
			utils.Error(w, http.StatusUnauthorized, combinedErr)
		case http.StatusNotFound:
			utils.Error(w, http.StatusNotFound, combinedErr)
		default:
			utils.InternalServerError(w, combinedErr)
		}
		return
	}
	utils.InternalServerError(w, combinedErr)
}

func inspectCandidate(ctx context.Context, sys *types.SystemContext, named reference.Named) (dockerRegistry.DistributionInspect, error) {
	ref, err := docker.NewReference(named)
	if err != nil {
		return dockerRegistry.DistributionInspect{}, err
	}

	src, err := ref.NewImageSource(ctx, sys)
	if err != nil {
		return dockerRegistry.DistributionInspect{}, err
	}
	defer src.Close()

	unparsed := image.UnparsedInstance(src, nil)
	manifestBytes, manifestType, err := unparsed.Manifest(ctx)
	if err != nil {
		return dockerRegistry.DistributionInspect{}, err
	}

	inspect := dockerRegistry.DistributionInspect{
		Descriptor: ocispec.Descriptor{
			MediaType: manifestType,
			Size:      int64(len(manifestBytes)),
		},
	}

	if canonical, ok := named.(reference.Canonical); ok {
		inspect.Descriptor.Digest = canonical.Digest()
	} else {
		digest, err := manifest.Digest(manifestBytes)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		inspect.Descriptor.Digest = digest
	}

	if manifest.MIMETypeIsMultiImage(manifestType) {
		list, err := manifest.ListFromBlob(manifestBytes, manifestType)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		for _, d := range list.Instances() {
			instance, err := list.Instance(d)
			if err != nil {
				return dockerRegistry.DistributionInspect{}, err
			}
			if instance.ReadOnly.Platform != nil {
				inspect.Platforms = append(inspect.Platforms, *instance.ReadOnly.Platform)
			}
		}
	} else {
		img, err := image.FromUnparsedImage(ctx, sys, unparsed)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		ociConfig, err := img.OCIConfig(ctx)
		if err != nil {
			return dockerRegistry.DistributionInspect{}, err
		}
		if ociConfig.OS != "" || ociConfig.Architecture != "" {
			inspect.Platforms = append(inspect.Platforms, ociConfig.Platform)
		}
	}

	return inspect, nil
}
