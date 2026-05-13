//go:build !remote && (linux || freebsd)

package abi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/opencontainers/go-digest"
	imgspecv1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	bdefine "go.podman.io/buildah/define"
	"go.podman.io/buildah/pkg/volumes"
	"go.podman.io/common/libimage"
	"go.podman.io/common/libimage/filter"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/docker"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/image"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/pkg/compression"
	"go.podman.io/image/v5/signature"
	"go.podman.io/image/v5/transports"
	"go.podman.io/image/v5/transports/alltransports"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/domain/entities/reports"
	domainUtils "go.podman.io/podman/v6/pkg/domain/utils"
	"go.podman.io/podman/v6/pkg/errorhandling"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/storage"
	"go.podman.io/storage/pkg/unshare"
	"go.podman.io/storage/types"
)

const UnknownDigestSuffix = docker.UnknownDigestSuffix

func (ir *ImageEngine) Exists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	exists, err := ir.Libpod.LibimageRuntime().Exists(nameOrID)
	if err != nil {
		return nil, err
	}
	return &entities.BoolReport{Value: exists}, nil
}

func (ir *ImageEngine) Prune(ctx context.Context, opts entities.ImagePruneOptions) ([]*reports.PruneReport, error) {
	pruneOptions := &libimage.RemoveImagesOptions{
		RemoveContainerFunc:     ir.Libpod.RemoveContainersForImageCallback(ctx, true),
		IsExternalContainerFunc: ir.Libpod.IsExternalContainerCallback(ctx),
		ExternalContainers:      opts.External,
		Filters:                 append(opts.Filter, "readonly=false"),
		WithSize:                true,
		DryRun:                  opts.DryRun,
	}

	if !opts.All {
		// Issue #20469: Docker clients handle the --all flag on the
		// client side by setting the dangling filter directly.
		alreadySet := false
		for _, filter := range pruneOptions.Filters {
			if strings.HasPrefix(filter, "dangling=") {
				alreadySet = true
				break
			}
		}
		if !alreadySet {
			pruneOptions.Filters = append(pruneOptions.Filters, "dangling=true")
		}
	}
	if opts.External {
		pruneOptions.Filters = append(pruneOptions.Filters, "containers=external")
	} else {
		pruneOptions.Filters = append(pruneOptions.Filters, "containers=false")
	}

	if opts.DryRun {
		pruneOptions.Ignore = true	
	}

	pruneReports := make([]*reports.PruneReport, 0)

	// Now prune all images until we converge.
	numPreviouslyRemovedImages := 1
	for {
		removedImages, rmErrors := ir.Libpod.LibimageRuntime().RemoveImages(ctx, nil, pruneOptions)
		if rmErrors != nil {
			return nil, errorhandling.JoinErrors(rmErrors)
		}

		for _, rmReport := range removedImages {
			r := *rmReport
			pruneReports = append(pruneReports, &reports.PruneReport{
				Id:   r.ID,
				Size: uint64(r.Size),
			})
		}

		numRemovedImages := len(removedImages)
		if numRemovedImages+numPreviouslyRemovedImages == 0 || pruneOptions.DryRun {
			break
		}
		numPreviouslyRemovedImages = numRemovedImages
	}

	if opts.BuildCache || opts.All {
		// Clean build cache if any
		if err := volumes.CleanCacheMount(); err != nil {
			return nil, err
		}
	}

	return pruneReports, nil
}

func toDomainHistoryLayer(layer *libimage.ImageHistory) entities.ImageHistoryLayer {
	l := entities.ImageHistoryLayer{
		Comment:   layer.Comment,
		CreatedBy: layer.CreatedBy,
		ID:        layer.ID,
		Size:      layer.Size,
		Tags:      layer.Tags,
	}
	if layer.Created != nil {
		l.Created = *layer.Created
	}
	return l
}

func (ir *ImageEngine) History(ctx context.Context, nameOrID string, _ entities.ImageHistoryOptions) (*entities.ImageHistoryReport, error) {
	image, _, err := ir.Libpod.LibimageRuntime().LookupImage(nameOrID, nil)
	if err != nil {
		return nil, err
	}

	results, err := image.History(ctx)
	if err != nil {
		return nil, err
	}

	history := entities.ImageHistoryReport{
		Layers: make([]entities.ImageHistoryLayer, len(results)),
	}

	for i := range results {
		history.Layers[i] = toDomainHistoryLayer(&results[i])
	}
	return &history, nil
}

func (ir *ImageEngine) Mount(ctx context.Context, nameOrIDs []string, opts entities.ImageMountOptions) ([]*entities.ImageMountReport, error) {
	listMountsOnly := false
	var images []*libimage.Image
	var err error

	hasCapSysAdmin, err := unshare.HasCapSysAdmin()
	if err != nil {
		return nil, err
	}

	if os.Geteuid() != 0 || !hasCapSysAdmin {
		if driver := ir.Libpod.StorageConfig().GraphDriverName; driver != "vfs" {
			// Do not allow to mount a graphdriver that is not vfs if we are creating the userns as part
			// of the mount command.
			return nil, fmt.Errorf("cannot mount using driver %s in rootless mode", driver)
		}

		became, ret, err := rootless.BecomeRootInUserNS("")
		if err != nil {
			return nil, err
		}
		if became {
			os.Exit(ret)
		}
	}

	switch {
	case opts.All && len(nameOrIDs) > 0:
		return nil, errors.New("cannot mix --all with images")
	case len(nameOrIDs) > 0:
		images, err = ir.Libpod.LibimageRuntime().ListImagesByNames(nameOrIDs)
		if err != nil {
			return nil, err
		}
	default:
		listImagesOptions := &libimage.ListImagesOptions{}
		if opts.All {
			listImagesOptions.Filters = []string{"readonly=false"}
		} else {
			listMountsOnly = true
		}
		images, err = ir.Libpod.LibimageRuntime().ListImages(ctx, listImagesOptions)
		if err != nil {
			return nil, err
		}
	}

	mountReports := []*entities.ImageMountReport{}
	for _, i := range images {
		var mountPoint string
		var err error
		if listMountsOnly {
			// We're only looking for mounted images.
			mountPoint, err = i.Mountpoint()
			if err != nil {
				if errors.Is(err, types.ErrImageUnknown) {
					continue
				}
				return nil, err
			}
			// Not mounted, so skip.
			if mountPoint == "" {
				continue
			}
		} else {
			mountPoint, err = i.Mount(ctx, nil, "")
			if err != nil {
				return nil, err
			}
		}

		tags, err := i.RepoTags()
		if err != nil {
			return nil, err
		}
		mountReports = append(mountReports, &entities.ImageMountReport{
			Id:           i.ID(),
			Name:         string(i.Digest()),
			Repositories: tags,
			Path:         mountPoint,
		})
	}
	return mountReports, nil
}

func (ir *ImageEngine) Unmount(ctx context.Context, nameOrIDs []string, options entities.ImageUnmountOptions) ([]*entities.ImageUnmountReport, error) {
	var images []*libimage.Image
	var err error
	switch {
	case options.All && len(nameOrIDs) > 0:
		return nil, errors.New("cannot mix --all with images")
	case len(nameOrIDs) > 0:
		images, err = ir.Libpod.LibimageRuntime().ListImagesByNames(nameOrIDs)
		if err != nil {
			return nil, err
		}
	default:
		listImagesOptions := &libimage.ListImagesOptions{}
		if options.All {
			listImagesOptions.Filters = []string{"readonly=false"}
		}
		images, err = ir.Libpod.LibimageRuntime().ListImages(ctx, listImagesOptions)
		if err != nil {
			return nil, err
		}
	}

	unmountReports := []*entities.ImageUnmountReport{}
	for _, image := range images {
		r := &entities.ImageUnmountReport{Id: image.ID()}
		mountPoint, err := image.Mountpoint()
		if err != nil {
			r.Err = err
			unmountReports = append(unmountReports, r)
			continue
		}
		if mountPoint == "" {
			// Skip if the image wasn't mounted.
			continue
		}
		r.Err = image.Unmount(options.Force)
		unmountReports = append(unmountReports, r)
	}
	return unmountReports, nil
}

func (ir *ImageEngine) Pull(ctx context.Context, rawImage string, options entities.ImagePullOptions) (*entities.ImagePullReport, error) {
	pullOptions := &libimage.PullOptions{AllTags: options.AllTags}
	pullOptions.AuthFilePath = options.Authfile
	pullOptions.CertDirPath = options.CertDir
	pullOptions.Username = options.Username
	pullOptions.Password = options.Password
	pullOptions.Architecture = options.Arch
	pullOptions.OS = options.OS
	pullOptions.Variant = options.Variant
	pullOptions.SignaturePolicyPath = options.SignaturePolicy
	pullOptions.InsecureSkipTLSVerify = options.SkipTLSVerify
	pullOptions.Writer = options.Writer
	pullOptions.OciDecryptConfig = options.OciDecryptConfig
	pullOptions.MaxRetries = options.Retry

	if options.RetryDelay != "" {
		duration, err := time.ParseDuration(options.RetryDelay)
		if err != nil {
			return nil, err
		}
		pullOptions.RetryDelay = &duration
	}

	if !options.Quiet && pullOptions.Writer == nil {
		pullOptions.Writer = os.Stderr
	}

	pulledImages, err := ir.Libpod.LibimageRuntime().Pull(ctx, rawImage, options.PullPolicy, pullOptions)
	if err != nil {
		return nil, err
	}

	pulledIDs := make([]string, len(pulledImages))
	for i := range pulledImages {
		pulledIDs[i] = pulledImages[i].ID()
	}

	return &entities.ImagePullReport{Images: pulledIDs}, nil
}

func (ir *ImageEngine) Inspect(ctx context.Context, namesOrIDs []string, _ entities.InspectOptions) ([]*entities.ImageInspectReport, []error, error) {
	reports := []*entities.ImageInspectReport{}
	errs := []error{}

	inspectOptions := &libimage.InspectOptions{WithParent: true, WithSize: true}
	for _, i := range namesOrIDs {
		img, _, err := ir.Libpod.LibimageRuntime().LookupImage(i, nil)
		if err != nil {
			// This is probably a no such image, treat as nonfatal.
			errs = append(errs, err)
			continue
		}
		result, err := img.Inspect(ctx, inspectOptions)
		if err != nil {
			// This is more likely to be fatal.
			return nil, nil, err
		}
		report := entities.ImageInspectReport{}
		if err := domainUtils.DeepCopy(&report, result); err != nil {
			return nil, nil, err
		}
		reports = append(reports, &report)
	}
	return reports, errs, nil
}

func (ir *ImageEngine) Push(ctx context.Context, source string, destination string, options entities.ImagePushOptions) (*entities.ImagePushReport, error) {
	var manifestType string
	switch options.Format {
	case "":
		// Default
	case "oci":
		manifestType = imgspecv1.MediaTypeImageManifest
	case "v2s1":
		manifestType = manifest.DockerV2Schema1SignedMediaType
	case "v2s2", "docker":
		manifestType = manifest.DockerV2Schema2MediaType
	default:
		return nil, fmt.Errorf("unknown format %q. Choose on of the supported formats: 'oci', 'v2s1', or 'v2s2'", options.Format)
	}

	pushOptions := &libimage.PushOptions{}
	pushOptions.AuthFilePath = options.Authfile
	pushOptions.CertDirPath = options.CertDir
	pushOptions.DirForceCompress = options.Compress
	pushOptions.Username = options.Username
	pushOptions.Password = options.Password
	pushOptions.ManifestMIMEType = manifestType
	pushOptions.RemoveSignatures = options.RemoveSignatures
	pushOptions.PolicyAllowStorage = true
	pushOptions.SignaturePolicyPath = options.SignaturePolicy
	pushOptions.Signers = options.Signers
	pushOptions.SignBy = options.SignBy
	pushOptions.SignPassphrase = options.SignPassphrase
	pushOptions.SignBySigstorePrivateKeyFile = options.SignBySigstorePrivateKeyFile
	pushOptions.SignSigstorePrivateKeyPassphrase = options.SignSigstorePrivateKeyPassphrase
	pushOptions.InsecureSkipTLSVerify = options.SkipTLSVerify
	pushOptions.Writer = options.Writer
	pushOptions.OciEncryptConfig = options.OciEncryptConfig
	pushOptions.OciEncryptLayers = options.OciEncryptLayers
	pushOptions.CompressionLevel = options.CompressionLevel
	pushOptions.ForceCompressionFormat = options.ForceCompressionFormat
	pushOptions.MaxRetries = options.Retry
	if options.RetryDelay != "" {
		duration, err := time.ParseDuration(options.RetryDelay)
		if err != nil {
			return nil, err
		}
		pushOptions.RetryDelay = &duration
	}

	compressionFormat := options.CompressionFormat
	if compressionFormat == "" {
		config, err := ir.Libpod.GetConfigNoCopy()
		if err != nil {
			return nil, err
		}
		compressionFormat = config.Engine.CompressionFormat
	}
	if compressionFormat != "" {
		algo, err := compression.AlgorithmByName(compressionFormat)
		if err != nil {
			return nil, err
		}
		pushOptions.CompressionFormat = &algo
	}

	if pushOptions.CompressionLevel == nil {
		config, err := ir.Libpod.GetConfigNoCopy()
		if err != nil {
			return nil, err
		}
		pushOptions.CompressionLevel = config.Engine.CompressionLevel
	}

	if !options.Quiet && pushOptions.Writer == nil {
		pushOptions.Writer = os.Stderr
	}

	pushedManifestBytes, pushError := ir.Libpod.LibimageRuntime().Push(ctx, source, destination, pushOptions)
	if pushError == nil {
		manifestDigest, err := manifest.Digest(pushedManifestBytes)
		if err != nil {
			return nil, err
		}
		return &entities.ImagePushReport{ManifestDigest: manifestDigest.String()}, nil
	}
	// If the image could not be found, we may be referring to a manifest
	// list but could not find a matching image instance in the local
	// containers storage. In that case, fall back and attempt to push the
	// (entire) manifest.
	if _, err := ir.Libpod.LibimageRuntime().LookupManifestList(source); err == nil {
		pushedManifestString, err := ir.ManifestPush(ctx, source, destination, options)
		if err != nil {
			return nil, err
		}
		return &entities.ImagePushReport{ManifestDigest: pushedManifestString}, nil
	}
	return nil, pushError
}

func (ir *ImageEngine) Tag(_ context.Context, nameOrID string, tags []string, _ entities.ImageTagOptions) error {
	// Allow tagging manifest list instead of resolving instances from manifest
	lookupOptions := &libimage.LookupImageOptions{ManifestList: true}
	image, _, err := ir.Libpod.LibimageRuntime().LookupImage(nameOrID, lookupOptions)
	if err != nil {
		return err
	}
	for _, tag := range tags {
		if err := image.Tag(tag); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Untag(_ context.Context, nameOrID string, tags []string, _ entities.ImageUntagOptions) error {
	image, _, err := ir.Libpod.LibimageRuntime().LookupImage(nameOrID, nil)
	if err != nil {
		return err
	}
	// If only one arg is provided, all names are to be untagged
	if len(tags) == 0 {
		tags = image.Names()
	}
	for _, tag := range tags {
		if err := image.Untag(tag); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Load(ctx context.Context, options entities.ImageLoadOptions) (*entities.ImageLoadReport, error) {
	loadOptions := &libimage.LoadOptions{}
	loadOptions.SignaturePolicyPath = options.SignaturePolicy
	if !options.Quiet {
		loadOptions.Writer = os.Stderr
	}

	loadedImages, err := ir.Libpod.LibimageRuntime().Load(ctx, options.Input, loadOptions)
	if err != nil {
		return nil, err
	}
	return &entities.ImageLoadReport{Names: loadedImages}, nil
}

func (ir *ImageEngine) Save(ctx context.Context, nameOrID string, tags []string, options entities.ImageSaveOptions) error {
	saveOptions := &libimage.SaveOptions{}
	saveOptions.DirForceCompress = options.Compress
	saveOptions.OciAcceptUncompressedLayers = options.OciAcceptUncompressedLayers
	saveOptions.SignaturePolicyPath = options.SignaturePolicy

	// Force signature removal to preserve backwards compat.
	// See https://github.com/containers/podman/pull/11669#issuecomment-925250264
	saveOptions.RemoveSignatures = true

	if !options.Quiet {
		saveOptions.Writer = os.Stderr
	}

	names := []string{nameOrID}
	if options.MultiImageArchive {
		names = append(names, tags...)
	} else {
		saveOptions.AdditionalTags = tags
	}
	return ir.Libpod.LibimageRuntime().Save(ctx, names, options.Format, options.Output, saveOptions)
}

func (ir *ImageEngine) Import(ctx context.Context, options entities.ImageImportOptions) (*entities.ImageImportReport, error) {
	importOptions := &libimage.ImportOptions{}
	importOptions.Changes = options.Changes
	importOptions.CommitMessage = options.Message
	importOptions.Tag = options.Reference
	importOptions.SignaturePolicyPath = options.SignaturePolicy
	importOptions.OS = options.OS
	importOptions.Arch = options.Architecture
	importOptions.Variant = options.Variant

	if !options.Quiet {
		importOptions.Writer = os.Stderr
	}

	imageID, err := ir.Libpod.LibimageRuntime().Import(ctx, options.Source, importOptions)
	if err != nil {
		return nil, err
	}

	return &entities.ImageImportReport{Id: imageID}, nil
}

// Search for images using term and filters
func (ir *ImageEngine) Search(ctx context.Context, term string, opts entities.ImageSearchOptions) ([]entities.ImageSearchReport, error) {
	filter, err := filter.ParseSearchFilter(opts.Filters)
	if err != nil {
		return nil, err
	}

	searchOptions := &libimage.SearchOptions{
		Authfile:              opts.Authfile,
		CertDirPath:           opts.CertDir,
		Username:              opts.Username,
		Password:              opts.Password,
		IdentityToken:         opts.IdentityToken,
		Filter:                *filter,
		Limit:                 opts.Limit,
		NoTrunc:               true,
		InsecureSkipTLSVerify: opts.SkipTLSVerify,
		ListTags:              opts.ListTags,
	}

	searchResults, err := ir.Libpod.LibimageRuntime().Search(ctx, term, searchOptions)
	if err != nil {
		return nil, err
	}

	// Convert from image.SearchResults to entities.ImageSearchReport. We don't
	// want to leak any low-level packages into the remote client, which
	// requires converting.
	reports := make([]entities.ImageSearchReport, len(searchResults))
	for i := range searchResults {
		reports[i].Index = searchResults[i].Index
		reports[i].Name = searchResults[i].Name
		reports[i].Description = searchResults[i].Description
		reports[i].Stars = searchResults[i].Stars
		reports[i].Official = searchResults[i].Official
		reports[i].Automated = searchResults[i].Automated
		reports[i].Tag = searchResults[i].Tag
	}

	return reports, nil
}

// Config returns a copy of the configuration used by the runtime
func (ir *ImageEngine) Config(_ context.Context) (*config.Config, error) {
	return ir.Libpod.GetConfig()
}

func (ir *ImageEngine) Build(ctx context.Context, containerFiles []string, opts entities.BuildOptions) (*entities.BuildReport, error) {
	id, _, err := ir.Libpod.Build(ctx, opts.BuildOptions, containerFiles...)
	if err != nil {
		return nil, err
	}
	saveFormat := define.OCIArchive
	if opts.OutputFormat == bdefine.Dockerv2ImageManifest {
		saveFormat = define.V2s2Archive
	}
	return &entities.BuildReport{ID: id, SaveFormat: saveFormat}, nil
}

func (ir *ImageEngine) Tree(ctx context.Context, nameOrID string, opts entities.ImageTreeOptions) (*entities.ImageTreeReport, error) {
	image, _, err := ir.Libpod.LibimageRuntime().LookupImage(nameOrID, nil)
	if err != nil {
		return nil, err
	}
	tree, err := image.Tree(ctx, opts.WhatRequires)
	if err != nil {
		return nil, err
	}
	return &entities.ImageTreeReport{Tree: tree}, nil
}

// removeErrorsToExitCode returns an exit code for the specified slice of
// image-removal errors. The error codes are set according to the documented
// behaviour in the Podman man pages.
func removeErrorsToExitCode(rmErrors []error) int {
	var (
		// noSuchImageErrors indicates that at least one image was not found.
		noSuchImageErrors bool
		// inUseErrors indicates that at least one image is being used by a
		// container.
		inUseErrors bool
		// otherErrors indicates that at least one error other than the two
		// above occurred.
		otherErrors bool
	)

	if len(rmErrors) == 0 {
		return 0
	}

	for _, e := range rmErrors {
		//nolint:gocritic
		if errors.Is(e, storage.ErrImageUnknown) || errors.Is(e, storage.ErrLayerUnknown) {
			noSuchImageErrors = true
		} else if errors.Is(e, storage.ErrImageUsedByContainer) {
			inUseErrors = true
		} else {
			otherErrors = true
		}
	}

	switch {
	case inUseErrors:
		// One of the specified images has child images or is
		// being used by a container.
		return 2
	case noSuchImageErrors && (!otherErrors && !inUseErrors):
		// One of the specified images did not exist, and no other
		// failures.
		return 1
	default:
		return 125
	}
}

// Remove removes one or more images from local storage.
func (ir *ImageEngine) Remove(ctx context.Context, images []string, opts entities.ImageRemoveOptions) (report *entities.ImageRemoveReport, rmErrors []error) {
	report = &entities.ImageRemoveReport{}

	// Set the exit code at very end.
	defer func() {
		report.ExitCode = removeErrorsToExitCode(rmErrors)
	}()

	libimageOptions := &libimage.RemoveImagesOptions{}
	libimageOptions.Filters = []string{"readonly=false"}
	libimageOptions.Force = opts.Force
	libimageOptions.Ignore = opts.Ignore
	libimageOptions.LookupManifest = opts.LookupManifest
	libimageOptions.NoPrune = opts.NoPrune
	if !opts.All {
		libimageOptions.Filters = append(libimageOptions.Filters, "intermediate=false")
	}
	libimageOptions.RemoveContainerFunc = ir.Libpod.RemoveContainersForImageCallback(ctx, !opts.DisableForceRemoveContainers)

	libimageReport, libimageErrors := ir.Libpod.LibimageRuntime().RemoveImages(ctx, images, libimageOptions)

	for _, r := range libimageReport {
		if r.Removed {
			report.Deleted = append(report.Deleted, r.ID)
		}
		report.Untagged = append(report.Untagged, r.Untagged...)
	}

	rmErrors = libimageErrors

	return report, rmErrors
}

// Shutdown Libpod engine
func (ir *ImageEngine) Shutdown(_ context.Context) {
	shutdownSync.Do(func() {
		_ = ir.Libpod.Shutdown(false)
	})
}

func (ir *ImageEngine) Sign(ctx context.Context, names []string, options entities.SignOptions) (*entities.SignReport, error) {
	mech, err := signature.NewGPGSigningMechanism()
	if err != nil {
		return nil, fmt.Errorf("initializing GPG: %w", err)
	}
	defer mech.Close()
	if err := mech.SupportsSigning(); err != nil {
		return nil, fmt.Errorf("signing is not supported: %w", err)
	}
	sc := ir.Libpod.SystemContext()
	sc.DockerCertPath = options.CertDir
	sc.AuthFilePath = options.Authfile

	for _, signimage := range names {
		err = func() error {
			srcRef, err := alltransports.ParseImageName(signimage)
			if err != nil {
				return fmt.Errorf("parsing image name: %w", err)
			}
			rawSource, err := srcRef.NewImageSource(ctx, sc)
			if err != nil {
				return fmt.Errorf("getting image source: %w", err)
			}
			defer func() {
				if err = rawSource.Close(); err != nil {
					logrus.Errorf("Unable to close %s image source %q", srcRef.DockerReference().Name(), err)
				}
			}()
			topManifestBlob, manifestType, err := image.UnparsedInstance(rawSource, nil).Manifest(ctx)
			if err != nil {
				return fmt.Errorf("getting manifest blob: %w", err)
			}
			dockerReference := rawSource.Reference().DockerReference()
			if dockerReference == nil {
				return fmt.Errorf("cannot determine canonical Docker reference for destination %s", transports.ImageName(rawSource.Reference()))
			}
			var sigStoreDir string
			if options.Directory != "" {
				repo := reference.Path(dockerReference)
				if path.Clean(repo) != repo { // Coverage: This should not be reachable because /./ and /../ components are not valid in docker references
					return fmt.Errorf("unexpected path elements in Docker reference %s for signature storage", dockerReference.String())
				}
				sigStoreDir = filepath.Join(options.Directory, repo)
			} else {
				signatureURL, err := docker.SignatureStorageBaseURL(sc, rawSource.Reference(), true)
				if err != nil {
					return err
				}
				sigStoreDir, err = localPathFromURI(signatureURL)
				if err != nil {
					return err
				}
			}
			manifestDigest, err := manifest.Digest(topManifestBlob)
			if err != nil {
				return err
			}

			if options.All {
				if !manifest.MIMETypeIsMultiImage(manifestType) {
					return fmt.Errorf("%s is not a multi-architecture image (manifest type %s)", signimage, manifestType)
				}
				list, err := manifest.ListFromBlob(topManifestBlob, manifestType)
				if err != nil {
					return fmt.Errorf("parsing manifest list %q: %w", string(topManifestBlob), err)
				}
				instanceDigests := list.Instances()
				for _, instanceDigest := range instanceDigests {
					digest := instanceDigest
					man, _, err := image.UnparsedInstance(rawSource, &digest).Manifest(ctx)
					if err != nil {
						return err
					}
					if err = putSignature(man, mech, sigStoreDir, instanceDigest, dockerReference, options); err != nil {
						return fmt.Errorf("storing signature for %s, %v: %w", dockerReference.String(), instanceDigest, err)
					}
				}
				return nil
			}
			if err = putSignature(topManifestBlob, mech, sigStoreDir, manifestDigest, dockerReference, options); err != nil {
				return fmt.Errorf("storing signature for %s, %v: %w", dockerReference.String(), manifestDigest, err)
			}
			return nil
		}()
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (ir *ImageEngine) Scp(ctx context.Context, src, dst string, opts entities.ImageScpOptions) (*entities.ImageScpReport, error) {
	report, err := domainUtils.ExecuteTransfer(src, dst, opts.ScpExecuteTransferOptions)
	if err != nil {
		return nil, err
	}
	if report.LoadReport == nil && (report.Source != nil && report.Dest != nil) { // we need to execute the transfer
		transferOpts := entities.ScpTransferOptions{}
		transferOpts.ParentFlags = report.ParentFlags
		transferOpts.SaveFormat = report.SaveFormat
		_, err := Transfer(ctx, *report.Source, *report.Dest, transferOpts)
		if err != nil {
			return nil, err
		}
	}
	return &entities.ImageScpReport{}, nil
}

func Transfer(_ context.Context, source entities.ScpTransferImageOptions, dest entities.ScpTransferImageOptions, opts entities.ScpTransferOptions) (*entities.ScpTransferReport, error) {
	if source.User == "" {
		return nil, fmt.Errorf("you must define a user when transferring from root to rootless storage: %w", define.ErrInvalidArg)
	}
	podman, err := os.Executable()
	if err != nil {
		return nil, err
	}
	rep := entities.ScpTransferReport{}
	if rootless.IsRootless() && (len(dest.User) == 0 || dest.User == "root") { // if we are rootless and do not have a destination user we can just use sudo
		return &rep, transferRootless(source, dest, podman, opts)
	}
	return &rep, transferRootful(source, dest, podman, opts)
}

// TransferRootless creates new podman processes using exec.Command and sudo, transferring images between the given source and destination users
func transferRootless(source entities.ScpTransferImageOptions, dest entities.ScpTransferImageOptions, podman string, opts entities.ScpTransferOptions) error {
	var cmdSave *exec.Cmd
	saveCommand := slices.Clone(opts.ParentFlags)
	loadCommand := slices.Clone(opts.ParentFlags)
	saveCommand = append(saveCommand, "save")
	if opts.SaveFormat != "" {
		saveCommand = append(saveCommand, "--format", opts.SaveFormat)
	}
	loadCommand = append(loadCommand, []string{"load"}...)
	if source.Quiet {
		saveCommand = append(saveCommand, "-q")
		loadCommand = append(loadCommand, "-q")
	}

	saveCommand = append(saveCommand, []string{"--output", source.File, source.Image}...)

	loadCommand = append(loadCommand, []string{"--input", dest.File}...)

	if source.User == "root" {
		cmdSave = exec.Command("sudo", podman)
	} else {
		cmdSave = exec.Command(podman)
	}
	cmdSave = domainUtils.CreateSCPCommand(cmdSave, saveCommand)
	logrus.Debugf("Executing save command: %q", cmdSave)
	err := cmdSave.Run()
	if err != nil {
		return err
	}

	var cmdLoad *exec.Cmd
	if source.User != "root" {
		cmdLoad = exec.Command("sudo", podman)
	} else {
		cmdLoad = exec.Command(podman)
	}
	cmdLoad = domainUtils.CreateSCPCommand(cmdLoad, loadCommand)
	logrus.Debugf("Executing load command: %q", cmdLoad)
	if len(dest.Tag) > 0 {
		return domainUtils.ScpTag(cmdLoad, podman, dest)
	}
	return cmdLoad.Run()
}

// transferRootful creates new podman processes using exec.Command and a new uid/gid alongside a cleared environment
func transferRootful(source entities.ScpTransferImageOptions, dest entities.ScpTransferImageOptions, podman string, opts entities.ScpTransferOptions) error {
	basicCommand := make([]string, 0, len(opts.ParentFlags)+1)
	basicCommand = append(basicCommand, podman)
	basicCommand = append(basicCommand, opts.ParentFlags...)

	saveCommand := make([]string, 0, len(basicCommand)+6)
	saveCommand = append(saveCommand, basicCommand...)
	saveCommand = append(saveCommand, "save")
	if opts.SaveFormat != "" {
		saveCommand = append(saveCommand, "--format", opts.SaveFormat)
	}

	loadCommand := make([]string, 0, len(basicCommand)+3)
	loadCommand = append(loadCommand, basicCommand...)
	loadCommand = append(loadCommand, "load")
	if source.Quiet {
		saveCommand = append(saveCommand, "-q")
		loadCommand = append(loadCommand, "-q")
	}
	saveCommand = append(saveCommand, []string{"--output", source.File, source.Image}...)
	loadCommand = append(loadCommand, []string{"--input", dest.File}...)

	// if executing using sudo or transferring between two users, the TransferRootless approach will not work, the new process needs to be set up
	// with the proper uid and gid as well as environmental variables.
	var uSave *user.User
	var uLoad *user.User
	var err error
	source.User, _, _ = strings.Cut(source.User, ":") // In case of uid:gid.
	dest.User, _, _ = strings.Cut(dest.User, ":")
	uSave, err = lookupUser(source.User)
	if err != nil {
		return err
	}
	switch {
	case dest.User != "": // if we are given a destination user, check that first
		uLoad, err = lookupUser(dest.User)
		if err != nil {
			return err
		}
	case uSave.Name != "root": // else if we have no destination user, and source is not root that means we should be root
		uLoad, err = user.LookupId("0")
		if err != nil {
			return err
		}
	default: // else if we have no dest user, and source user IS root, we want to be the default user.
		uString := os.Getenv("SUDO_USER")
		if uString == "" {
			return errors.New("$SUDO_USER must be defined to find the default rootless user")
		}
		uLoad, err = user.Lookup(uString)
		if err != nil {
			return err
		}
	}
	_, err = execTransferPodman(uSave, saveCommand, false)
	if err != nil {
		return err
	}
	out, err := execTransferPodman(uLoad, loadCommand, len(dest.Tag) > 0)
	if err != nil {
		return err
	}
	if out != nil {
		image := domainUtils.ExtractImage(out)
		_, err := execTransferPodman(uLoad, []string{podman, "tag", image, dest.Tag}, false)
		return err
	}
	return nil
}

func lookupUser(u string) (*user.User, error) {
	if u, err := user.LookupId(u); err == nil {
		return u, nil
	}
	return user.Lookup(u)
}

func execTransferPodman(execUser *user.User, command []string, needToTag bool) ([]byte, error) {
	cmdLogin, err := domainUtils.LoginUser(execUser.Username)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = cmdLogin.Process.Kill()
		_ = cmdLogin.Wait()
	}()

	cmd := exec.Command(command[0], command[1:]...)
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "TERM=" + os.Getenv("TERM")}
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	uid, err := strconv.ParseInt(execUser.Uid, 10, 32)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.ParseInt(execUser.Gid, 10, 32)
	if err != nil {
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:         uint32(uid),
			Gid:         uint32(gid),
			Groups:      nil,
			NoSetGroups: false,
		},
	}
	if needToTag {
		cmd.Stdout = nil
		return cmd.Output()
	}
	return nil, cmd.Run()
}

func getSigFilename(sigStoreDirPath string) (string, error) {
	sigFileSuffix := 1
	sigFiles, err := os.ReadDir(sigStoreDirPath)
	if err != nil {
		return "", err
	}
	sigFilenames := make(map[string]bool)
	for _, file := range sigFiles {
		sigFilenames[file.Name()] = true
	}
	for {
		sigFilename := "signature-" + strconv.Itoa(sigFileSuffix)
		if _, exists := sigFilenames[sigFilename]; !exists {
			return sigFilename, nil
		}
		sigFileSuffix++
	}
}

func localPathFromURI(url *url.URL) (string, error) {
	if url.Scheme != "file" {
		return "", fmt.Errorf("writing to %s is not supported. Use a supported scheme", url.String())
	}
	return url.Path, nil
}

// putSignature creates signature and saves it to the signstore file
func putSignature(manifestBlob []byte, mech signature.SigningMechanism, sigStoreDir string, instanceDigest digest.Digest, dockerReference reference.Reference, options entities.SignOptions) error {
	newSig, err := signature.SignDockerManifest(manifestBlob, dockerReference.String(), mech, options.SignBy)
	if err != nil {
		return err
	}
	signatureDir := fmt.Sprintf("%s@%s=%s", sigStoreDir, instanceDigest.Algorithm(), instanceDigest.Hex())
	if err := os.MkdirAll(signatureDir, 0o751); err != nil {
		// The directory is allowed to exist
		if !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	sigFilename, err := getSigFilename(signatureDir)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(signatureDir, sigFilename), newSig, 0o644)
}
