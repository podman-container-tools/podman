//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage"
	"go.podman.io/common/libimage/define"
	cp "go.podman.io/image/v5/copy"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/image"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/pkg/compression"
	"go.podman.io/image/v5/pkg/shortnames"
	"go.podman.io/image/v5/transports"
	"go.podman.io/image/v5/transports/alltransports"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/pkg/domain/entities"
	envLib "go.podman.io/podman/v6/pkg/env"
	"go.podman.io/storage"
)

// ManifestCreate implements logic for creating manifest lists via ImageEngine
func (ir *ImageEngine) ManifestCreate(ctx context.Context, name string, images []string, opts entities.ManifestCreateOptions) (string, error) {
	if len(name) == 0 {
		return "", errors.New("no name specified for creating a manifest list")
	}

	manifestList, err := ir.Libpod.LibimageRuntime().CreateManifestList(name)
	if err != nil {
		if errors.Is(err, storage.ErrDuplicateName) && opts.Amend {
			amendList, amendErr := ir.Libpod.LibimageRuntime().LookupManifestList(name)
			if amendErr != nil {
				return "", err
			}
			manifestList = amendList
		} else {
			return "", err
		}
	}

	if len(opts.Annotations) != 0 {
		annotateOptions := &libimage.ManifestListAnnotateOptions{
			IndexAnnotations: opts.Annotations,
		}
		if err := manifestList.AnnotateInstance("", annotateOptions); err != nil {
			return "", err
		}
	}

	addOptions := &libimage.ManifestListAddOptions{All: opts.All}
	for _, image := range images {
		if _, err := manifestList.Add(ctx, image, addOptions); err != nil {
			return "", err
		}
	}

	return manifestList.ID(), nil
}

// ManifestExists checks if a manifest list with the given name exists in local storage
func (ir *ImageEngine) ManifestExists(_ context.Context, name string) (*entities.BoolReport, error) {
	_, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		if errors.Is(err, storage.ErrImageUnknown) {
			return &entities.BoolReport{Value: false}, nil
		}
		return nil, err
	}

	return &entities.BoolReport{Value: true}, nil
}

// ManifestInspect returns the content of a manifest list or image
func (ir *ImageEngine) ManifestInspect(ctx context.Context, name string, opts entities.ManifestInspectOptions) (*define.ManifestListData, error) {
	// NOTE: we have to do a bit of a limbo here as `podman manifest
	// inspect foo` wants to do a remote-inspect of foo iff "foo" in the
	// containers storage is an ordinary image but not a manifest list.

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		if errors.Is(err, storage.ErrImageUnknown) || errors.Is(err, libimage.ErrNotAManifestList) {
			// Do a remote inspect if there's no local image or if the
			// local image is not a manifest list.
			return ir.remoteManifestInspect(ctx, name, opts)
		}

		return nil, err
	}

	return manifestList.Inspect()
}

// inspect a remote manifest list.
func (ir *ImageEngine) remoteManifestInspect(ctx context.Context, name string, opts entities.ManifestInspectOptions) (*define.ManifestListData, error) {
	inspectList := define.ManifestListData{}
	sys := ir.Libpod.SystemContext()

	if opts.Authfile != "" {
		sys.AuthFilePath = opts.Authfile
	}

	sys.DockerInsecureSkipTLSVerify = opts.SkipTLSVerify
	if opts.SkipTLSVerify == types.OptionalBoolTrue {
		sys.OCIInsecureSkipTLSVerify = true
	}

	resolved, err := shortnames.Resolve(sys, name)
	if err != nil {
		return nil, err
	}

	var (
		latestErr error
		result    []byte
		manType   string
	)
	appendErr := func(e error) {
		if latestErr == nil {
			latestErr = e
		} else {
			// FIXME should we use multierror package instead?

			// we want the new line here so ignore the linter
			latestErr = fmt.Errorf("tried %v\n: %w", e, latestErr)
		}
	}

	for _, candidate := range resolved.PullCandidates {
		ref, err := alltransports.ParseImageName("docker://" + candidate.Value.String())
		if err != nil {
			return nil, err
		}
		src, err := ref.NewImageSource(ctx, sys)
		if err != nil {
			appendErr(fmt.Errorf("reading image %q: %w", transports.ImageName(ref), err))
			continue
		}
		defer src.Close()

		manifestBytes, manifestType, err := image.UnparsedInstance(src, nil).Manifest(ctx)
		if err != nil {
			appendErr(fmt.Errorf("loading manifest %q: %w", transports.ImageName(ref), err))
			continue
		}

		result = manifestBytes
		manType = manifestType
		break
	}

	if len(result) == 0 && latestErr != nil {
		return nil, latestErr
	}

	switch manType {
	case manifest.DockerV2Schema2MediaType:
		logrus.Warnf("The manifest type %s is not a manifest list but a single image.", manType)
		schema2Manifest, err := manifest.Schema2FromManifest(result)
		if err != nil {
			return nil, fmt.Errorf("parsing manifest blob %q as a %q: %w", string(result), manType, err)
		}

		if result, err = schema2Manifest.Serialize(); err != nil {
			return nil, err
		}
	default:
		list, err := manifest.ListFromBlob(result, manType)
		if err != nil {
			return nil, fmt.Errorf("parsing manifest blob %q as a %q: %w", string(result), manType, err)
		}
		if result, err = list.Serialize(); err != nil {
			return nil, err
		}
	}

	if err := json.Unmarshal(result, &inspectList); err != nil {
		return nil, err
	}
	return &inspectList, nil
}

// ManifestAdd adds images to the manifest list
func (ir *ImageEngine) ManifestAdd(ctx context.Context, name string, images []string, opts entities.ManifestAddOptions) (string, error) {
	if len(images) < 1 {
		return "", errors.New("manifest add requires at least one image")
	}

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		return "", err
	}

	addOptions := &libimage.ManifestListAddOptions{
		All:                   opts.All,
		AuthFilePath:          opts.Authfile,
		CertDirPath:           opts.CertDir,
		InsecureSkipTLSVerify: opts.SkipTLSVerify,
		Username:              opts.Username,
		Password:              opts.Password,
	}

	images = slices.Clone(images)
	for _, image := range opts.Images {
		if !slices.Contains(images, image) {
			images = append(images, image)
		}
	}

	for _, image := range images {
		instanceDigest, err := manifestList.Add(ctx, image, addOptions)
		if err != nil {
			return "", err
		}

		annotateOptions := &libimage.ManifestListAnnotateOptions{
			Architecture: opts.Arch,
			Features:     opts.Features,
			OS:           opts.OS,
			OSVersion:    opts.OSVersion,
			Variant:      opts.Variant,
			Subject:      opts.IndexSubject,
		}

		if annotateOptions.Annotations, err = mergeAnnotations(opts.Annotations, opts.Annotation); err != nil {
			return "", err
		}

		if annotateOptions.IndexAnnotations, err = mergeAnnotations(opts.IndexAnnotations, opts.IndexAnnotation); err != nil {
			return "", err
		}

		if err := manifestList.AnnotateInstance(instanceDigest, annotateOptions); err != nil {
			return "", err
		}
	}
	return manifestList.ID(), nil
}

func mergeAnnotations(preferred map[string]string, aux []string) (map[string]string, error) {
	if len(aux) != 0 {
		auxAnnotations := make(map[string]string)
		for _, annotationSpec := range aux {
			key, val, hasVal := strings.Cut(annotationSpec, "=")
			if !hasVal {
				return nil, fmt.Errorf("no value given for annotation %q", key)
			}
			auxAnnotations[key] = val
		}
		if preferred == nil {
			preferred = make(map[string]string)
		}
		preferred = envLib.Join(auxAnnotations, preferred)
	}
	return preferred, nil
}

// ManifestAnnotate updates an entry of the manifest list
func (ir *ImageEngine) ManifestAnnotate(ctx context.Context, name, image string, opts entities.ManifestAnnotateOptions) (string, error) {
	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		return "", err
	}

	annotateOptions := &libimage.ManifestListAnnotateOptions{
		Architecture: opts.Arch,
		Features:     opts.Features,
		OS:           opts.OS,
		OSVersion:    opts.OSVersion,
		Variant:      opts.Variant,
		Subject:      opts.IndexSubject,
	}
	if annotateOptions.Annotations, err = mergeAnnotations(opts.Annotations, opts.Annotation); err != nil {
		return "", err
	}
	if annotateOptions.IndexAnnotations, err = mergeAnnotations(opts.IndexAnnotations, opts.IndexAnnotation); err != nil {
		return "", err
	}

	var instanceDigest digest.Digest
	if image == "" {
		if len(opts.Annotations) != 0 {
			return "", errors.New("setting annotation on an item in a manifest list requires an instance digest")
		}
		if len(opts.Annotation) != 0 {
			return "", errors.New("setting annotation on an item in a manifest list requires an instance digest")
		}
		if opts.Arch != "" {
			return "", errors.New("setting architecture on an item in a manifest list requires an instance digest")
		}
		if len(opts.Features) != 0 {
			return "", errors.New("setting features on an item in a manifest list requires an instance digest")
		}
		if opts.OS != "" {
			return "", errors.New("setting OS on an item in a manifest list requires an instance digest")
		}
		if len(opts.OSFeatures) != 0 {
			return "", errors.New("setting OS features on an item in a manifest list requires an instance digest")
		}
		if opts.OSVersion != "" {
			return "", errors.New("setting OS version on an item in a manifest list requires an instance digest")
		}
		if opts.Variant != "" {
			return "", errors.New("setting variant on an item in a manifest list requires an instance digest")
		}
	} else {
		if len(opts.IndexAnnotations) != 0 {
			return "", errors.New("setting index-wide annotation in a manifest list requires no instance digest")
		}
		if len(opts.IndexAnnotation) != 0 {
			return "", errors.New("setting index-wide annotation in a manifest list requires no instance digest")
		}
		if len(opts.IndexSubject) != 0 {
			return "", errors.New("setting subject for a manifest list requires no instance digest")
		}
		instanceDigest, err = ir.digestFromDigestOrManifestListMember(ctx, manifestList, image)
		if err != nil {
			return "", fmt.Errorf("finding instance for %q: %w", image, err)
		}
	}

	if err := manifestList.AnnotateInstance(instanceDigest, annotateOptions); err != nil {
		return "", err
	}

	return manifestList.ID(), nil
}

// ManifestAddArtifact creates artifact manifest for files and adds them to the manifest list
func (ir *ImageEngine) ManifestAddArtifact(ctx context.Context, name string, files []string, opts entities.ManifestAddArtifactOptions) (string, error) {
	if len(files) < 1 {
		return "", errors.New("manifest add artifact requires at least one file")
	}

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		return "", err
	}

	files = slices.Clone(files)
	for _, file := range opts.Files {
		if !slices.Contains(files, file) {
			files = append(files, file)
		}
	}

	addArtifactOptions := &libimage.ManifestListAddArtifactOptions{
		Type:          opts.Type,
		ConfigType:    opts.ConfigType,
		Config:        opts.Config,
		LayerType:     opts.LayerType,
		ExcludeTitles: opts.ExcludeTitles,
		Annotations:   opts.Annotations,
		Subject:       opts.Subject,
	}

	instanceDigest, err := manifestList.AddArtifact(ctx, addArtifactOptions, files...)
	if err != nil {
		return "", err
	}

	annotateOptions := &libimage.ManifestListAnnotateOptions{
		Architecture: opts.Arch,
		Features:     opts.Features,
		OS:           opts.OS,
		OSVersion:    opts.OSVersion,
		Variant:      opts.Variant,
		Subject:      opts.IndexSubject,
	}

	if annotateOptions.Annotations, err = mergeAnnotations(opts.ManifestAnnotateOptions.Annotations, opts.ManifestAnnotateOptions.Annotation); err != nil {
		return "", err
	}

	if annotateOptions.IndexAnnotations, err = mergeAnnotations(opts.ManifestAnnotateOptions.IndexAnnotations, opts.ManifestAnnotateOptions.IndexAnnotation); err != nil {
		return "", err
	}

	if err := manifestList.AnnotateInstance(instanceDigest, annotateOptions); err != nil {
		return "", err
	}

	return manifestList.ID(), nil
}

func (ir *ImageEngine) digestFromDigestOrManifestListMember(ctx context.Context, list *libimage.ManifestList, name string) (digest.Digest, error) {
	instanceDigest, err := digest.Parse(name)
	if err == nil {
		return instanceDigest, nil
	}
	listData, inspectErr := list.Inspect()
	if inspectErr != nil {
		return "", fmt.Errorf(`inspecting list "%s" for instance list: %v`, list.ID(), err)
	}
	// maybe the name is a file name we previously attached as part of an artifact manifest
	for _, descriptor := range listData.Manifests {
		if slices.Contains(descriptor.Files, path.Base(name)) || slices.Contains(descriptor.Files, name) {
			return descriptor.Digest, nil
		}
	}
	// maybe it's the name of an image we added to the list?
	ref, err := alltransports.ParseImageName(name)
	if err != nil {
		withDocker := fmt.Sprintf("%s://%s", docker.Transport.Name(), name)
		ref, err = alltransports.ParseImageName(withDocker)
		if err != nil {
			image, _, err := ir.Libpod.LibimageRuntime().LookupImage(name, &libimage.LookupImageOptions{ManifestList: true})
			if err != nil {
				return "", fmt.Errorf("locating image named %q to check if it's in the manifest list: %w", name, err)
			}
			if ref, err = image.StorageReference(); err != nil {
				return "", fmt.Errorf("reading image reference %q to check if it's in the manifest list: %w", name, err)
			}
		}
	}
	// read the manifest of this image
	src, err := ref.NewImageSource(ctx, ir.Libpod.SystemContext())
	if err != nil {
		return "", fmt.Errorf("reading local image %q to check if it's in the manifest list: %w", name, err)
	}
	defer src.Close()
	manifestBytes, _, err := image.UnparsedInstance(src, nil).Manifest(ctx)
	if err != nil {
		return "", fmt.Errorf("locating image named %q to check if it's in the manifest list: %w", name, err)
	}
	refDigest, err := manifest.Digest(manifestBytes)
	if err != nil {
		return "", fmt.Errorf("digesting manifest of local image %q: %w", name, err)
	}
	return refDigest, nil
}

// ManifestRemoveDigest removes specified digest from the specified manifest list
func (ir *ImageEngine) ManifestRemoveDigest(_ context.Context, name, image string) (string, error) {
	instanceDigest, err := digest.Parse(image)
	if err != nil {
		return "", fmt.Errorf(`invalid image digest "%s": %v`, image, err)
	}

	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		return "", err
	}

	if err := manifestList.RemoveInstance(instanceDigest); err != nil {
		return "", err
	}

	return manifestList.ID(), nil
}

// ManifestRm removes the specified manifest list from storage
func (ir *ImageEngine) ManifestRm(ctx context.Context, names []string, opts entities.ImageRemoveOptions) (report *entities.ImageRemoveReport, rmErrors []error) {
	return ir.Remove(ctx, names, entities.ImageRemoveOptions{LookupManifest: true, Ignore: opts.Ignore})
}

// ManifestPush pushes a manifest list or image index to the destination
func (ir *ImageEngine) ManifestPush(ctx context.Context, name, destination string, opts entities.ImagePushOptions) (string, error) {
	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		return "", fmt.Errorf("retrieving local image from image name %s: %w", name, err)
	}

	var manifestType string
	if opts.Format != "" {
		switch opts.Format {
		case "oci":
			manifestType = imgspecv1.MediaTypeImageManifest
		case "v2s2", "docker":
			manifestType = manifest.DockerV2Schema2MediaType
		default:
			return "", fmt.Errorf("unknown format %q. Choose one of the supported formats: 'oci' or 'v2s2'", opts.Format)
		}
	}

	pushOptions := &libimage.ManifestListPushOptions{}
	pushOptions.AuthFilePath = opts.Authfile
	pushOptions.CertDirPath = opts.CertDir
	pushOptions.Username = opts.Username
	pushOptions.Password = opts.Password
	pushOptions.ImageListSelection = cp.CopySpecificImages
	pushOptions.ManifestMIMEType = manifestType
	pushOptions.RemoveSignatures = opts.RemoveSignatures
	pushOptions.Signers = opts.Signers
	pushOptions.SignBy = opts.SignBy
	pushOptions.SignPassphrase = opts.SignPassphrase
	pushOptions.SignBySigstorePrivateKeyFile = opts.SignBySigstorePrivateKeyFile
	pushOptions.SignSigstorePrivateKeyPassphrase = opts.SignSigstorePrivateKeyPassphrase
	pushOptions.InsecureSkipTLSVerify = opts.SkipTLSVerify
	pushOptions.Writer = opts.Writer
	pushOptions.CompressionLevel = opts.CompressionLevel
	pushOptions.AddCompression = opts.AddCompression
	pushOptions.ForceCompressionFormat = opts.ForceCompressionFormat
	pushOptions.MaxRetries = opts.Retry

	config, err := ir.Libpod.GetConfigNoCopy()
	if err != nil {
		return "", err
	}

	if pushOptions.MaxRetries == nil {
		retry := config.Engine.Retry
		pushOptions.MaxRetries = &retry
	}
	retryDelay := opts.RetryDelay
	if retryDelay == "" {
		retryDelay = config.Engine.RetryDelay
	}
	if retryDelay != "" {
		duration, err := time.ParseDuration(retryDelay)
		if err != nil {
			return "", err
		}
		pushOptions.RetryDelay = &duration
	}

	compressionFormat := opts.CompressionFormat
	if compressionFormat == "" {
		compressionFormat = config.Engine.CompressionFormat
	}
	if compressionFormat != "" {
		algo, err := compression.AlgorithmByName(compressionFormat)
		if err != nil {
			return "", err
		}
		pushOptions.CompressionFormat = &algo
	}
	if pushOptions.CompressionLevel == nil {
		pushOptions.CompressionLevel = config.Engine.CompressionLevel
	}

	if opts.All {
		pushOptions.ImageListSelection = cp.CopyAllImages
	}
	if !opts.Quiet && pushOptions.Writer == nil {
		pushOptions.Writer = os.Stderr
	}

	manDigest, err := manifestList.Push(ctx, destination, pushOptions)
	if err != nil {
		return "", err
	}

	if opts.Rm {
		rmOpts := &libimage.RemoveImagesOptions{LookupManifest: true}
		if _, rmErrors := ir.Libpod.LibimageRuntime().RemoveImages(ctx, []string{manifestList.ID()}, rmOpts); len(rmErrors) > 0 {
			return "", fmt.Errorf("removing manifest after push: %w", rmErrors[0])
		}
	}

	return manDigest.String(), err
}

// ManifestListClear clears out all instances from the manifest list
func (ir *ImageEngine) ManifestListClear(_ context.Context, name string) (string, error) {
	manifestList, err := ir.Libpod.LibimageRuntime().LookupManifestList(name)
	if err != nil {
		return "", err
	}

	listContents, err := manifestList.Inspect()
	if err != nil {
		return "", err
	}

	for _, instance := range listContents.Manifests {
		if err := manifestList.RemoveInstance(instance.Digest); err != nil {
			return "", err
		}
	}

	return manifestList.ID(), nil
}
