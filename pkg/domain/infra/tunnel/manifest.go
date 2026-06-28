package tunnel

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.podman.io/common/libimage/define"
	"go.podman.io/image/v5/types"
	"go.podman.io/podman/v6/pkg/bindings/images"
	"go.podman.io/podman/v6/pkg/bindings/manifests"
	"go.podman.io/podman/v6/pkg/domain/entities"
	envLib "go.podman.io/podman/v6/pkg/env"
)

// ManifestCreate implements manifest create via ImageEngine
func (ir *ImageEngine) ManifestCreate(_ context.Context, name string, images []string, opts entities.ManifestCreateOptions) (string, error) {
	options := new(manifests.CreateOptions).WithAll(opts.All).WithAmend(opts.Amend).WithAnnotation(opts.Annotations)
	imageID, err := manifests.Create(ir.ClientCtx, name, images, options)
	if err != nil {
		return imageID, fmt.Errorf("creating manifest: %w", err)
	}
	return imageID, err
}

// ManifestExists checks if a manifest list with the given name exists
func (ir *ImageEngine) ManifestExists(_ context.Context, name string) (*entities.BoolReport, error) {
	exists, err := manifests.Exists(ir.ClientCtx, name, nil)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{Value: exists}, nil
}

// ManifestInspect returns contents of manifest list with given name
func (ir *ImageEngine) ManifestInspect(_ context.Context, name string, opts entities.ManifestInspectOptions) (*define.ManifestListData, error) {
	options := new(manifests.InspectOptions).WithAuthfile(opts.Authfile)
	if s := opts.SkipTLSVerify; s != types.OptionalBoolUndefined {
		if s == types.OptionalBoolTrue {
			options.WithSkipTLSVerify(true)
		} else {
			options.WithSkipTLSVerify(false)
		}
	}

	list, err := manifests.InspectListData(ir.ClientCtx, name, options)
	if err != nil {
		return nil, fmt.Errorf("getting content of manifest list or image %s: %w", name, err)
	}

	return list, err
}

// ManifestAdd adds images to the manifest list
func (ir *ImageEngine) ManifestAdd(_ context.Context, name string, imageNames []string, opts entities.ManifestAddOptions) (string, error) {
	imageNames = slices.Clone(imageNames)
	for _, image := range opts.Images {
		if !slices.Contains(imageNames, image) {
			imageNames = append(imageNames, image)
		}
	}

	options := new(manifests.AddOptions).WithAll(opts.All).WithArch(opts.Arch).WithVariant(opts.Variant)
	options.WithFeatures(opts.Features).WithImages(imageNames).WithOS(opts.OS).WithOSVersion(opts.OSVersion)
	options.WithUsername(opts.Username).WithPassword(opts.Password).WithAuthfile(opts.Authfile)

	if len(opts.Annotation) != 0 {
		annotations := make(map[string]string)
		for _, annotationSpec := range opts.Annotation {
			key, val, hasVal := strings.Cut(annotationSpec, "=")
			if !hasVal {
				return "", fmt.Errorf("no value given for annotation %q", key)
			}
			annotations[key] = val
		}
		opts.Annotations = envLib.Join(opts.Annotations, annotations)
	}
	options.WithAnnotation(opts.Annotations)

	if s := opts.SkipTLSVerify; s != types.OptionalBoolUndefined {
		if s == types.OptionalBoolTrue {
			options.WithSkipTLSVerify(true)
		} else {
			options.WithSkipTLSVerify(false)
		}
	}

	id, err := manifests.Add(ir.ClientCtx, name, options)
	if err != nil {
		return id, fmt.Errorf("adding to manifest list %s: %w", name, err)
	}
	return id, nil
}

// ManifestAddArtifact creates artifact manifests and adds them to the manifest list
func (ir *ImageEngine) ManifestAddArtifact(_ context.Context, name string, files []string, opts entities.ManifestAddArtifactOptions) (string, error) {
	files = slices.Clone(files)
	for _, file := range opts.Files {
		if !slices.Contains(files, file) {
			files = append(files, file)
		}
	}
	options := new(manifests.AddArtifactOptions).WithArch(opts.Arch).WithVariant(opts.Variant)
	options.WithFeatures(opts.Features).WithOS(opts.OS).WithOSVersion(opts.OSVersion).WithOSFeatures(opts.OSFeatures)
	if len(opts.Annotation) != 0 {
		annotations := make(map[string]string)
		for _, annotationSpec := range opts.Annotation {
			key, val, hasVal := strings.Cut(annotationSpec, "=")
			if !hasVal {
				return "", fmt.Errorf("no value given for annotation %q", key)
			}
			annotations[key] = val
		}
		options.WithAnnotation(annotations)
	}
	options.WithType(opts.Type).WithConfigType(opts.ConfigType).WithLayerType(opts.LayerType)
	options.WithConfig(opts.Config)
	options.WithExcludeTitles(opts.ExcludeTitles).WithSubject(opts.Subject)
	options.WithAnnotations(opts.Annotations)
	options.WithFiles(files)
	id, err := manifests.AddArtifact(ir.ClientCtx, name, options)
	if err != nil {
		return id, fmt.Errorf("adding to manifest list %s: %w", name, err)
	}
	return id, nil
}

// ManifestAnnotate updates an entry of the manifest list
func (ir *ImageEngine) ManifestAnnotate(_ context.Context, name, images string, opts entities.ManifestAnnotateOptions) (string, error) {
	options := new(manifests.ModifyOptions).WithArch(opts.Arch).WithVariant(opts.Variant)
	options.WithFeatures(opts.Features).WithOS(opts.OS).WithOSVersion(opts.OSVersion)

	annotations, err := mergeAnnotations(opts.Annotations, opts.Annotation)
	if err != nil {
		return "", err
	}
	options.WithAnnotations(annotations)

	indexAnnotations, err := mergeAnnotations(opts.IndexAnnotations, opts.IndexAnnotation)
	if err != nil {
		return "", err
	}
	options.WithIndexAnnotations(indexAnnotations)

	id, err := manifests.Annotate(ir.ClientCtx, name, []string{images}, options)
	if err != nil {
		return id, fmt.Errorf("annotating to manifest list %s: %w", name, err)
	}
	return id, nil
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

// ManifestRemoveDigest removes the digest from manifest list
func (ir *ImageEngine) ManifestRemoveDigest(_ context.Context, name string, image string) (string, error) {
	updatedListID, err := manifests.Remove(ir.ClientCtx, name, image, nil)
	if err != nil {
		return updatedListID, fmt.Errorf("removing from manifest %s: %w", name, err)
	}
	return fmt.Sprintf("%s :%s\n", updatedListID, image), nil
}

// ManifestRm removes the specified manifest list from storage
func (ir *ImageEngine) ManifestRm(ctx context.Context, names []string, opts entities.ImageRemoveOptions) (report *entities.ImageRemoveReport, rmErrors []error) {
	return ir.Remove(ctx, names, entities.ImageRemoveOptions{LookupManifest: true, Ignore: opts.Ignore})
}

// ManifestPush pushes a manifest list or image index to the destination
func (ir *ImageEngine) ManifestPush(ctx context.Context, name, destination string, opts entities.ImagePushOptions) (string, error) {
	if opts.Signers != nil {
		return "", fmt.Errorf("forwarding Signers is not supported for remote clients")
	}

	options := new(images.PushOptions)
	options.WithUsername(opts.Username).WithPassword(opts.Password).WithAuthfile(opts.Authfile).WithRemoveSignatures(opts.RemoveSignatures).WithAll(opts.All).WithFormat(opts.Format).WithCompressionFormat(opts.CompressionFormat).WithQuiet(opts.Quiet).WithProgressWriter(opts.Writer).WithAddCompression(opts.AddCompression).WithForceCompressionFormat(opts.ForceCompressionFormat)

	if opts.Retry != nil {
		options.WithRetry(*opts.Retry)
	}
	if opts.RetryDelay != "" {
		options.WithRetryDelay(opts.RetryDelay)
	}
	if s := opts.SkipTLSVerify; s != types.OptionalBoolUndefined {
		if s == types.OptionalBoolTrue {
			options.WithSkipTLSVerify(true)
		} else {
			options.WithSkipTLSVerify(false)
		}
	}
	digest, err := manifests.Push(ir.ClientCtx, name, destination, options)
	if err != nil {
		return "", fmt.Errorf("pushing manifest list %s: %w", name, err)
	}

	if opts.Rm {
		if _, rmErrors := ir.Remove(ctx, []string{name}, entities.ImageRemoveOptions{LookupManifest: true}); len(rmErrors) > 0 {
			return "", fmt.Errorf("removing manifest after push: %w", rmErrors[0])
		}
	}

	return digest, err
}

// ManifestListClear clears out all instances from a manifest list
func (ir *ImageEngine) ManifestListClear(_ context.Context, name string) (string, error) {
	listContents, err := manifests.InspectListData(ir.ClientCtx, name, &manifests.InspectOptions{})
	if err != nil {
		return "", err
	}

	for _, instance := range listContents.Manifests {
		if _, err := manifests.Remove(ir.ClientCtx, name, instance.Digest.String(), &manifests.RemoveOptions{}); err != nil {
			return "", err
		}
	}

	return name, nil
}
