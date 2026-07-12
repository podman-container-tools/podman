//go:build !remote && (linux || freebsd)

package libpod

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/docker/distribution/registry/api/errcode"
	"github.com/gorilla/schema"
	libartifact_types "go.podman.io/common/pkg/libartifact/types"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/internal/localapi"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/auth"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	domain_utils "go.podman.io/podman/v6/pkg/domain/utils"
)

func InspectArtifact(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	name := utils.GetName(r)

	imageEngine := abi.ImageEngine{Libpod: runtime}

	report, err := imageEngine.ArtifactInspect(r.Context(), name, entities.ArtifactInspectOptions{})
	if err != nil {
		if errors.Is(err, libartifact_types.ErrArtifactNotExist) {
			utils.ArtifactNotFound(w, name, err)
			return
		} else {
			utils.InternalServerError(w, err)
			return
		}
	}

	utils.WriteResponse(w, http.StatusOK, report)
}

func ListArtifact(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	imageEngine := abi.ImageEngine{Libpod: runtime}

	artifacts, err := imageEngine.ArtifactList(r.Context(), entities.ArtifactListOptions{})
	if err != nil {
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, artifacts)
}

func PullArtifact(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		Name       string             `schema:"name"`
		Retry      uint               `schema:"retry"`
		RetryDelay string             `schema:"retryDelay"`
		TLSVerify  types.OptionalBool `schema:"tlsVerify"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if query.Name == "" {
		utils.Error(w, http.StatusBadRequest, errors.New("name parameter is required"))
		return
	}

	artifactsPullOptions := entities.ArtifactPullOptions{}

	// If TLS verification is explicitly specified (True or False) in the query,
	// set the InsecureSkipTLSVerify option accordingly.
	// If TLSVerify was not set in the query, OptionalBoolUndefined is used and
	// handled later based off the target registry configuration.
	switch query.TLSVerify {
	case types.OptionalBoolTrue:
		artifactsPullOptions.InsecureSkipTLSVerify = types.NewOptionalBool(false)
	case types.OptionalBoolFalse:
		artifactsPullOptions.InsecureSkipTLSVerify = types.NewOptionalBool(true)
	case types.OptionalBoolUndefined:
		// If the user doesn't define TLSVerify in the query, do nothing and pass
		// it to the backend code to handle.
	default: // Should never happen
		panic("Unexpected handling occurred for TLSVerify")
	}

	if _, found := r.URL.Query()["retry"]; found {
		artifactsPullOptions.MaxRetries = &query.Retry
	}

	if len(query.RetryDelay) != 0 {
		artifactsPullOptions.RetryDelay = query.RetryDelay
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	artifactsPullOptions.AuthFilePath = authfile
	if authConf != nil {
		artifactsPullOptions.Username = authConf.Username
		artifactsPullOptions.Password = authConf.Password
		artifactsPullOptions.IdentityToken = authConf.IdentityToken
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	artifacts, err := imageEngine.ArtifactPull(r.Context(), query.Name, artifactsPullOptions)
	if err != nil {
		var errcd errcode.ErrorCoder
		// Check to see if any of the wrapped errors is an errcode.ErrorCoder returned from the registry
		if errors.As(err, &errcd) {
			rc := errcd.ErrorCode().Descriptor().HTTPStatusCode
			// Check if the returned error is 401 StatusUnauthorized indicating the request was unauthorized
			if rc == http.StatusUnauthorized {
				utils.Error(w, http.StatusUnauthorized, err)
				return
			}
			// Check if the returned error is 404 StatusNotFound indicating the artifact was not found
			if rc == http.StatusNotFound {
				utils.Error(w, http.StatusNotFound, err)
				return
			}
		}
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, artifacts)
}

func RemoveArtifact(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	imageEngine := abi.ImageEngine{Libpod: runtime}

	name := utils.GetName(r)

	artifacts, err := imageEngine.ArtifactRm(r.Context(), entities.ArtifactRemoveOptions{Artifacts: []string{name}})
	if err != nil {
		if errors.Is(err, libartifact_types.ErrArtifactNotExist) {
			utils.ArtifactNotFound(w, name, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, artifacts)
}

func BatchRemoveArtifact(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		All       bool     `schema:"all"`
		Artifacts []string `schema:"artifacts"`
		Ignore    bool     `schema:"ignore"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if query.All && len(query.Artifacts) > 0 {
		utils.Error(w, http.StatusBadRequest, errors.New("when setting all to true, you may not pass any artifact names or digests"))
		return
	}

	if !query.All && len(query.Artifacts) < 1 {
		utils.Error(w, http.StatusBadRequest, errors.New("an artifact or all option must be specified"))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	removeOptions := entities.ArtifactRemoveOptions{
		Artifacts: query.Artifacts,
		All:       query.All,
		Ignore:    query.Ignore,
	}

	artifacts, err := imageEngine.ArtifactRm(r.Context(), removeOptions)
	if err != nil {
		if errors.Is(err, libartifact_types.ErrArtifactNotExist) {
			if removeOptions.Ignore {
				utils.WriteResponse(w, http.StatusOK, artifacts)
				return
			}
			utils.ArtifactNotFound(w, "", err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, artifacts)
}

type artifactAddRequestQuery struct {
	Name             string   `schema:"name"`
	FileName         string   `schema:"fileName"`
	FileMIMEType     string   `schema:"fileMIMEType"`
	Annotations      []string `schema:"annotations"`
	ArtifactMIMEType string   `schema:"artifactMIMEType"`
	Append           bool     `schema:"append"`
	Replace          bool     `schema:"replace"`
	Path             string   `schema:"path"`
}

func AddArtifact(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := artifactAddRequestQuery{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if query.Name == "" || query.FileName == "" {
		utils.Error(w, http.StatusBadRequest, errors.New("name and file parameters are required"))
		return
	}

	artifactBlobs := []entities.ArtifactBlob{{
		BlobReader: r.Body,
		FileName:   query.FileName,
	}}

	addArtifactHelper(query, artifactBlobs, w, r)
}

func AddLocalArtifact(w http.ResponseWriter, r *http.Request) {
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := artifactAddRequestQuery{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	if query.Name == "" || query.FileName == "" {
		utils.Error(w, http.StatusBadRequest, errors.New("name and file parameters are required"))
		return
	}

	cleanPath := filepath.Clean(query.Path)
	// Check if the path exists on server side.
	// Note: localapi.ValidatePathForLocalAPI returns nil if the file exists and path is absolute, not an error.
	switch err := localapi.ValidatePathForLocalAPI(cleanPath); {
	case err == nil:
		// no error -> continue
	case errors.Is(err, localapi.ErrPathNotAbsolute):
		utils.Error(w, http.StatusBadRequest, err)
		return
	case errors.Is(err, fs.ErrNotExist):
		utils.Error(w, http.StatusNotFound, fmt.Errorf("file does not exist: %q", cleanPath))
		return
	default:
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to access file: %w", err))
		return
	}

	artifactBlobs := []entities.ArtifactBlob{{
		BlobFilePath: cleanPath,
		FileName:     query.FileName,
	}}

	addArtifactHelper(query, artifactBlobs, w, r)
}

func addArtifactHelper(query artifactAddRequestQuery, artifactBlobs []entities.ArtifactBlob, w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	annotations, err := domain_utils.ParseAnnotations(query.Annotations)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}

	artifactAddOptions := entities.ArtifactAddOptions{
		Append:           query.Append,
		Annotations:      annotations,
		ArtifactMIMEType: query.ArtifactMIMEType,
		FileMIMEType:     query.FileMIMEType,
		Replace:          query.Replace,
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	artifacts, err := imageEngine.ArtifactAdd(r.Context(), query.Name, artifactBlobs, artifactAddOptions)
	if err != nil {
		if errors.Is(err, libartifact_types.ErrArtifactNotExist) {
			utils.ArtifactNotFound(w, query.Name, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusCreated, artifacts)
}

func PushArtifact(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		Retry      uint               `schema:"retry"`
		RetryDelay string             `schema:"retrydelay"`
		TLSVerify  types.OptionalBool `schema:"tlsVerify"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, errors.New("name parameter is required"))
		return
	}

	name := utils.GetName(r)

	artifactsPushOptions := entities.ArtifactPushOptions{}

	// If TLS verification is explicitly specified (True or False) in the query,
	// set the SkipTLSVerify option accordingly.
	// If TLSVerify was not set in the query, OptionalBoolUndefined is used and
	// handled later based off the target registry configuration.
	switch query.TLSVerify {
	case types.OptionalBoolTrue:
		artifactsPushOptions.SkipTLSVerify = types.NewOptionalBool(false)
	case types.OptionalBoolFalse:
		artifactsPushOptions.SkipTLSVerify = types.NewOptionalBool(true)
	case types.OptionalBoolUndefined:
		// If the user doesn't define TLSVerify in the query, do nothing and pass
		// it to the backend code to handle.
	default: // Should never happen
		panic("Unexpected handling occurred for TLSVerify")
	}

	if _, found := r.URL.Query()["retry"]; found {
		artifactsPushOptions.Retry = &query.Retry
	}

	if len(query.RetryDelay) != 0 {
		artifactsPushOptions.RetryDelay = query.RetryDelay
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	artifactsPushOptions.Authfile = authfile
	if authConf != nil {
		artifactsPushOptions.Username = authConf.Username
		artifactsPushOptions.Password = authConf.Password
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	artifacts, err := imageEngine.ArtifactPush(r.Context(), name, artifactsPushOptions)
	if err != nil {
		var errcd errcode.ErrorCoder
		// Check to see if any of the wrapped errors is an errcode.ErrorCoder returned from the registry
		if errors.As(err, &errcd) {
			rc := errcd.ErrorCode().Descriptor().HTTPStatusCode
			// Check if the returned error is 401 indicating the request was unauthorized
			if rc == 401 {
				utils.Error(w, 401, err)
				return
			}
		}

		var notFoundErr layout.ImageNotFoundError
		if errors.As(err, &notFoundErr) {
			utils.ArtifactNotFound(w, name, notFoundErr)
			return
		}

		utils.InternalServerError(w, err)
		return
	}

	utils.WriteResponse(w, http.StatusOK, artifacts)
}

func ExtractArtifact(w http.ResponseWriter, r *http.Request) {
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	decoder := r.Context().Value(api.DecoderKey).(*schema.Decoder)

	query := struct {
		Digest       string `schema:"digest"`
		Title        string `schema:"title"`
		ExcludeTitle bool   `schema:"excludetitle"`
	}{}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	extractOpts := entities.ArtifactExtractOptions{
		Title:        query.Title,
		Digest:       query.Digest,
		ExcludeTitle: query.ExcludeTitle,
	}

	name := utils.GetName(r)

	imageEngine := abi.ImageEngine{Libpod: runtime}

	err := imageEngine.ArtifactExtractTarStream(r.Context(), w, name, extractOpts)
	if err != nil {
		if errors.Is(err, libartifact_types.ErrArtifactNotExist) {
			utils.ArtifactNotFound(w, name, err)
			return
		}
		utils.InternalServerError(w, err)
		return
	}
}
