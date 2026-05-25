//go:build !remote && (linux || freebsd)

package compat

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	dockerSpec "github.com/moby/docker-image-spec/specs-go/v1"
	dockerContainer "github.com/moby/moby/api/types/container"
	dockerImage "github.com/moby/moby/api/types/image"
	dockerStorage "github.com/moby/moby/api/types/storage"
	"github.com/opencontainers/go-digest"
	imageSpec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"go.podman.io/buildah"
	"go.podman.io/common/libimage"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/filters"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/api/handlers"
	"go.podman.io/podman/v6/pkg/api/handlers/utils"
	"go.podman.io/podman/v6/pkg/api/handlers/utils/apiutil"
	api "go.podman.io/podman/v6/pkg/api/types"
	"go.podman.io/podman/v6/pkg/auth"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/infra/abi"
	"go.podman.io/podman/v6/pkg/util"
	"go.podman.io/storage"
)

// mergeNameAndTagOrDigest creates an image reference as string from the
// provided image name and tagOrDigest which can be a tag, a digest or empty.
func mergeNameAndTagOrDigest(name, tagOrDigest string) string {
	if len(tagOrDigest) == 0 {
		return name
	}

	separator := ":" // default to tag
	if _, err := digest.Parse(tagOrDigest); err == nil {
		// We have a digest, so let's change the separator.
		separator = "@"
	}
	return fmt.Sprintf("%s%s%s", name, separator, tagOrDigest)
}

func ExportImage(w http.ResponseWriter, r *http.Request) {
	// 200 ok
	// 500 server
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	tmpfile, err := os.CreateTemp("", "api.tar")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to create tempfile: %w", err))
		return
	}
	defer os.Remove(tmpfile.Name())

	name := utils.GetName(r)
	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	saveOptions := entities.ImageSaveOptions{
		Format: "docker-archive",
		Output: tmpfile.Name(),
	}

	if err := imageEngine.Save(r.Context(), possiblyNormalizedName, nil, saveOptions); err != nil {
		if errors.Is(err, storage.ErrImageUnknown) {
			utils.ImageNotFound(w, name, fmt.Errorf("failed to find image %s: %w", name, err))
			return
		}
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to create tempfile: %w", err))
		return
	}

	if err := tmpfile.Close(); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to close tempfile: %w", err))
		return
	}

	rdr, err := os.Open(tmpfile.Name())
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to read the exported tarfile: %w", err))
		return
	}
	defer rdr.Close()
	utils.WriteResponse(w, http.StatusOK, rdr)
}

func CommitContainer(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Author    string   `schema:"author"`
		Changes   []string `schema:"changes"`
		Comment   string   `schema:"comment"`
		Container string   `schema:"container"`
		Pause     bool     `schema:"pause"`
		Squash    bool     `schema:"squash"`
		Repo      string   `schema:"repo"`
		Tag       string   `schema:"tag"`
		// fromSrc   string  # fromSrc is currently unused
	}{
		Tag: "latest",
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	sc := runtime.SystemContext()
	options := libpod.ContainerCommitOptions{
		Pause: true,
	}
	options.CommitOptions = buildah.CommitOptions{
		ReportWriter:          os.Stderr,
		SystemContext:         sc,
		PreferredManifestType: manifest.DockerV2Schema2MediaType,
	}

	options.Message = query.Comment
	options.Author = query.Author
	options.Pause = query.Pause
	options.Squash = query.Squash
	options.Changes = util.DecodeChanges(query.Changes)
	if r.Body != nil {
		defer r.Body.Close()
		overrideConfig, err := abi.DecodeOverrideConfig(r.Body)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		options.CommitOptions.OverrideConfig = overrideConfig
	}
	ctr, err := runtime.LookupContainer(query.Container)
	if err != nil {
		utils.Error(w, http.StatusNotFound, err)
		return
	}

	var destImage string
	if len(query.Repo) > 1 {
		destImage = fmt.Sprintf("%s:%s", query.Repo, query.Tag)
		possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, destImage)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
			return
		}
		destImage = possiblyNormalizedName
	}

	commitImage, err := ctr.Commit(r.Context(), destImage, options)
	if err != nil && !strings.Contains(err.Error(), "is not running") {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}
	utils.WriteResponse(w, http.StatusCreated, entities.IDResponse{ID: commitImage.ID()})
}

func CreateImageFromSrc(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 repo does not exist or no read access
	// 500 internal
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Changes  []string `schema:"changes"`
		FromSrc  string   `schema:"fromSrc"`
		Message  string   `schema:"message"`
		Platform string   `schema:"platform"`
		Repo     string   `schema:"repo"`
		Tag      string   `schema:"tag"`
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	// fromSrc – Source to import. The value may be a URL from which the image can be retrieved or - to read the image from the request body. This parameter may only be used when importing an image.
	source := query.FromSrc
	if source == "-" {
		f, err := os.CreateTemp("", "api_load.tar")
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to create tempfile: %w", err))
			return
		}
		defer func() {
			err := os.Remove(f.Name())
			if err != nil {
				logrus.Errorf("Failed to remove temporary file: %v.", err)
			}
		}()
		source = f.Name()
		if err := SaveFromBody(f, r); err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to write temporary file: %w", err))
			return
		}
	}

	reference := query.Repo
	if query.Repo != "" {
		possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, mergeNameAndTagOrDigest(reference, query.Tag))
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
			return
		}
		reference = possiblyNormalizedName
	}

	platformSpecs := strings.Split(query.Platform, "/")
	opts := entities.ImageImportOptions{
		Source:    source,
		Changes:   query.Changes,
		Message:   query.Message,
		Reference: reference,
		OS:        platformSpecs[0],
	}
	if len(platformSpecs) > 1 {
		opts.Architecture = platformSpecs[1]
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}
	report, err := imageEngine.Import(r.Context(), opts)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to import tarball: %w", err))
		return
	}
	// Success
	utils.WriteResponse(w, http.StatusOK, struct {
		Status         string            `json:"status"`
		Progress       string            `json:"progress"`
		ProgressDetail map[string]string `json:"progressDetail"`
		Id             string            `json:"id"`
	}{
		Status:         report.Id,
		ProgressDetail: map[string]string{},
		Id:             report.Id,
	})
}

func CreateImageFromImage(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 repo does not exist or no read access
	// 500 internal
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		FromImage  string `schema:"fromImage"`
		Tag        string `schema:"tag"`
		Platform   string `schema:"platform"`
		Retry      uint   `schema:"retry"`
		RetryDelay string `schema:"retryDelay"`
	}{
		// This is where you can override the golang default value for one of fields
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, mergeNameAndTagOrDigest(query.FromImage, query.Tag))
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	authConf, authfile, err := auth.GetCredentials(r)
	if err != nil {
		utils.Error(w, http.StatusBadRequest, err)
		return
	}
	defer auth.RemoveAuthfile(authfile)

	pullOptions := &libimage.PullOptions{}
	pullOptions.AuthFilePath = authfile
	if authConf != nil {
		pullOptions.Username = authConf.Username
		pullOptions.Password = authConf.Password
		pullOptions.IdentityToken = authConf.IdentityToken
	}
	pullOptions.Writer = os.Stderr // allows for debugging on the server

	if _, found := r.URL.Query()["retry"]; found {
		pullOptions.MaxRetries = &query.Retry
	}

	if _, found := r.URL.Query()["retryDelay"]; found {
		duration, err := time.ParseDuration(query.RetryDelay)
		if err != nil {
			utils.Error(w, http.StatusBadRequest, err)
			return
		}
		pullOptions.RetryDelay = &duration
	}

	// Handle the platform.
	platformSpecs := strings.Split(query.Platform, "/")
	pullOptions.OS = platformSpecs[0] // may be empty
	if len(platformSpecs) > 1 {
		pullOptions.Architecture = platformSpecs[1]
		if len(platformSpecs) > 2 {
			pullOptions.Variant = platformSpecs[2]
		}
	}

	utils.CompatPull(r, w, runtime, possiblyNormalizedName, config.PullPolicyAlways, pullOptions)
}

func GetImage(w http.ResponseWriter, r *http.Request) {
	// 200 no error
	// 404 no such
	// 500 internal
	name := utils.GetName(r)
	possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, name)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
		return
	}

	newImage, err := utils.GetImage(r, possiblyNormalizedName)
	if err != nil {
		// Here we need to fiddle with the error message because docker-py is looking for "No
		// such image" to determine on how to raise the correct exception.
		errMsg := strings.ReplaceAll(err.Error(), "image not known", "No such image")
		utils.Error(w, http.StatusNotFound, fmt.Errorf("failed to find image %s: %s", name, errMsg))
		return
	}
	inspect, err := imageDataToImageInspect(r.Context(), newImage, r)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to convert ImageData to ImageInspect '%s': %w", name, err))
		return
	}
	utils.WriteResponse(w, http.StatusOK, inspect)
}

func imageDataToImageInspect(ctx context.Context, l *libimage.Image, r *http.Request) (*handlers.ImageInspect, error) {
	options := &libimage.InspectOptions{WithParent: true, WithSize: true}
	info, err := l.Inspect(ctx, options)
	if err != nil {
		return nil, err
	}

	// TODO: many fields in Config still need wiring
	config := dockerSpec.DockerOCIImageConfig{
		ImageConfig: imageSpec.ImageConfig{
			User:         info.User,
			ExposedPorts: info.Config.ExposedPorts,
			Env:          info.Config.Env,
			Cmd:          info.Config.Cmd,
			Volumes:      info.Config.Volumes,
			WorkingDir:   info.Config.WorkingDir,
			Entrypoint:   info.Config.Entrypoint,
			Labels:       info.Labels,
			StopSignal:   info.Config.StopSignal,
		},
	}

	rootfs := dockerImage.RootFS{}
	if info.RootFS != nil {
		rootfs.Type = info.RootFS.Type
		rootfs.Layers = make([]string, 0, len(info.RootFS.Layers))
		for _, layer := range info.RootFS.Layers {
			rootfs.Layers = append(rootfs.Layers, string(layer))
		}
	}

	graphDriver := dockerStorage.DriverData{
		Name: info.GraphDriver.Name,
		Data: info.GraphDriver.Data,
	}
	// Add in basic ContainerConfig to satisfy docker-compose
	cc := new(dockerContainer.Config)
	cc.Hostname = info.ID[0:11] // short ID is the hostname
	cc.Volumes = info.Config.Volumes

	dockerImageInspect := dockerImage.InspectResponse{
		Architecture: info.Architecture,
		Author:       info.Author,
		Comment:      info.Comment,
		Config:       &config,
		Created:      l.Created().Format(time.RFC3339Nano),
		GraphDriver:  &graphDriver,
		ID:           "sha256:" + l.ID(),
		Metadata:     dockerImage.Metadata{},
		Os:           info.Os,
		OsVersion:    info.Version,
		RepoDigests:  info.RepoDigests,
		RepoTags:     info.RepoTags,
		RootFS:       rootfs,
		Size:         info.Size,
		Variant:      "",
	}

	imageInspect := handlers.ImageInspect{
		InspectResponse: dockerImageInspect,
		DockerVersion:   info.Version,
		Parent:          info.Parent,
	}

	if _, err := apiutil.SupportedVersion(r, "<1.44.0"); err == nil {
		imageInspect.VirtualSize = info.VirtualSize
	}

	if _, err := apiutil.SupportedVersion(r, "<1.45.0"); err == nil {
		imageInspect.ContainerConfig = cc //nolint:staticcheck // Deprecated field
	}

	return &imageInspect, nil
}

func GetImages(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)
	query := struct {
		All        bool
		Digests    bool
		Filter     string // Docker 1.24 compatibility
		SharedSize bool   `schema:"shared-size"` // Docker 1.42 compatibility
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest,
			fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	if _, found := r.URL.Query()["digests"]; found && query.Digests {
		utils.UnSupportedParameter("digests")
		return
	}

	var filterList []string
	var err error
	if utils.IsLibpodRequest(r) {
		// Podman clients split the filter map as `"{"label":["version","1.0"]}`
		filterList, err = filters.FiltersFromRequest(r)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		// Docker clients split the filter map as `"{"label":["version=1.0"]}`
		filterList, err = util.FiltersFromRequest(r)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, err)
			return
		}
		if len(query.Filter) > 0 { // Docker 1.24 compatibility
			filterList = append(filterList, "reference="+query.Filter)
		}
		filterList = append(filterList, "manifest=false")
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	listOptions := entities.ImageListOptions{All: query.All, Filter: filterList, ExtendedAttributes: utils.IsLibpodRequest(r)}
	summaries, err := imageEngine.List(r.Context(), listOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, err)
		return
	}

	if !utils.IsLibpodRequest(r) {
		// docker adds sha256: in front of the ID
		for _, s := range summaries {
			s.ID = "sha256:" + s.ID
			// Ensure RepoTags and RepoDigests are empty arrays instead of null for Docker compatibility
			// as per https://docs.docker.com/reference/api/engine/version-history/#v143-api-changes
			// Relates to https://issues.redhat.com/browse/RUN-2699
			if s.RepoTags == nil {
				s.RepoTags = []string{}
			}
			if s.RepoDigests == nil {
				s.RepoDigests = []string{}
			}
			// Docker 1.42 sets SharedSize to -1 if ont passed explicitly
			if !query.SharedSize {
				s.SharedSize = -1
			}
			// VirtualSize is deprecated in version 1.43 and removed in version 1.44
			// See https://docs.docker.com/reference/api/engine/version-history/#v143-api-changes
			if _, err := apiutil.SupportedVersion(r, "<1.44.0"); err == nil {
				s.VirtualSize = s.Size
			} else {
				s.VirtualSize = 0
			}
		}
	}
	utils.WriteResponse(w, http.StatusOK, summaries)
}

func LoadImages(w http.ResponseWriter, r *http.Request) {
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Changes map[string]string `json:"changes"` // Ignored
		Message string            `json:"message"` // Ignored
		Quiet   bool              `json:"quiet"`   // Ignored
	}{
		// This is where you can override the golang default value for one of fields
	}

	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}

	// First write the body to a temporary file that we can later attempt
	// to load.
	f, err := os.CreateTemp("", "api_load.tar")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to create tempfile: %w", err))
		return
	}
	defer func() {
		err := os.Remove(f.Name())
		if err != nil {
			logrus.Errorf("Failed to remove temporary file: %v.", err)
		}
	}()
	if err := SaveFromBody(f, r); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to write temporary file: %w", err))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	loadOptions := entities.ImageLoadOptions{Input: f.Name()}
	loadReport, err := imageEngine.Load(r.Context(), loadOptions)
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to load image: %w", err))
		return
	}

	if len(loadReport.Names) < 1 {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("one or more images are required"))
		return
	}

	utils.WriteResponse(w, http.StatusOK, struct {
		Stream string `json:"stream"`
	}{
		Stream: fmt.Sprintf("Loaded image: %s", strings.Join(loadReport.Names, ",")),
	})
}

func ExportImages(w http.ResponseWriter, r *http.Request) {
	// 200 OK
	// 500 Error
	decoder := utils.GetDecoder(r)
	runtime := r.Context().Value(api.RuntimeKey).(*libpod.Runtime)

	query := struct {
		Names []string `schema:"names"`
	}{
		// This is where you can override the golang default value for one of fields
	}
	if err := decoder.Decode(&query, r.URL.Query()); err != nil {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("failed to parse parameters for %s: %w", r.URL.String(), err))
		return
	}
	if len(query.Names) == 0 {
		utils.Error(w, http.StatusBadRequest, fmt.Errorf("no images to download"))
		return
	}

	images := make([]string, len(query.Names))
	for i, img := range query.Names {
		possiblyNormalizedName, err := utils.NormalizeToDockerHub(r, img)
		if err != nil {
			utils.Error(w, http.StatusInternalServerError, fmt.Errorf("normalizing image: %w", err))
			return
		}
		images[i] = possiblyNormalizedName
	}

	tmpfile, err := os.CreateTemp("", "api.tar")
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to create tempfile: %w", err))
		return
	}
	defer os.Remove(tmpfile.Name())
	if err := tmpfile.Close(); err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("unable to close tempfile: %w", err))
		return
	}

	imageEngine := abi.ImageEngine{Libpod: runtime}

	saveOptions := entities.ImageSaveOptions{Format: "docker-archive", Output: tmpfile.Name(), MultiImageArchive: true}
	if err := imageEngine.Save(r.Context(), images[0], images[1:], saveOptions); err != nil {
		utils.InternalServerError(w, err)
		return
	}

	rdr, err := os.Open(tmpfile.Name())
	if err != nil {
		utils.Error(w, http.StatusInternalServerError, fmt.Errorf("failed to read the exported tarfile: %w", err))
		return
	}
	defer rdr.Close()
	utils.WriteResponse(w, http.StatusOK, rdr)
}
