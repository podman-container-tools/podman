package tunnel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	bdefine "github.com/containers/buildah/define"
	"github.com/containers/podman/v5/internal/localapi"
	"github.com/containers/podman/v5/libpod/define"
	"github.com/containers/podman/v5/pkg/bindings/images"
	"github.com/containers/podman/v5/pkg/domain/entities"
	"github.com/containers/podman/v5/pkg/domain/entities/reports"
	"github.com/containers/podman/v5/pkg/domain/utils"
	"github.com/containers/podman/v5/pkg/errorhandling"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/libimage/filter"
	"go.podman.io/common/pkg/config"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/types"
	"go.podman.io/storage/pkg/archive"
)

func (ir *ImageEngine) Exists(_ context.Context, nameOrID string) (*entities.BoolReport, error) {
	found, err := images.Exists(ir.ClientCtx, nameOrID, nil)
	return &entities.BoolReport{Value: found}, err
}

func (ir *ImageEngine) Remove(_ context.Context, imagesArg []string, opts entities.ImageRemoveOptions) (*entities.ImageRemoveReport, []error) {
	options := new(images.RemoveOptions).WithForce(opts.Force).WithIgnore(opts.Ignore).WithAll(opts.All).WithLookupManifest(opts.LookupManifest).WithNoPrune(opts.NoPrune)
	return images.Remove(ir.ClientCtx, imagesArg, options)
}

func (ir *ImageEngine) List(_ context.Context, opts entities.ImageListOptions) ([]*entities.ImageSummary, error) {
	filters := make(map[string][]string, len(opts.Filter))
	for _, filter := range opts.Filter {
		f := strings.SplitN(filter, "=", 2)
		if len(f) > 1 {
			filters[f[0]] = append(filters[f[0]], f[1])
		} else {
			filters[f[0]] = append(filters[f[0]], "")
		}
	}
	options := new(images.ListOptions).WithAll(opts.All).WithFilters(filters)
	psImages, err := images.List(ir.ClientCtx, options)
	if err != nil {
		return nil, err
	}

	is := make([]*entities.ImageSummary, len(psImages))
	for i, img := range psImages {
		hold := entities.ImageSummary{}
		if err := utils.DeepCopy(&hold, img); err != nil {
			return nil, err
		}
		is[i] = &hold
	}
	return is, nil
}

func (ir *ImageEngine) Mount(_ context.Context, _ []string, _ entities.ImageMountOptions) ([]*entities.ImageMountReport, error) {
	return nil, errors.New("mounting images is not supported for remote clients")
}

func (ir *ImageEngine) Unmount(_ context.Context, _ []string, _ entities.ImageUnmountOptions) ([]*entities.ImageUnmountReport, error) {
	return nil, errors.New("unmounting images is not supported for remote clients")
}

func (ir *ImageEngine) History(_ context.Context, nameOrID string, _ entities.ImageHistoryOptions) (*entities.ImageHistoryReport, error) {
	options := new(images.HistoryOptions)
	results, err := images.History(ir.ClientCtx, nameOrID, options)
	if err != nil {
		return nil, err
	}

	history := entities.ImageHistoryReport{
		Layers: make([]entities.ImageHistoryLayer, len(results)),
	}

	for i, layer := range results {
		// Created time comes over as an int64 so needs conversion to time.time
		t := time.Unix(layer.Created, 0)
		hold := entities.ImageHistoryLayer{
			ID:        layer.ID,
			Created:   t.UTC(),
			CreatedBy: layer.CreatedBy,
			Tags:      layer.Tags,
			Size:      layer.Size,
			Comment:   layer.Comment,
		}
		history.Layers[i] = hold
	}
	return &history, nil
}

func (ir *ImageEngine) Prune(_ context.Context, opts entities.ImagePruneOptions) ([]*reports.PruneReport, error) {
	filters := make(map[string][]string, len(opts.Filter))
	for _, filter := range opts.Filter {
		f := strings.Split(filter, "=")
		filters[f[0]] = f[1:]
	}
	options := new(images.PruneOptions).WithAll(opts.All).WithFilters(filters).WithExternal(opts.External).WithBuildCache(opts.BuildCache)
	reports, err := images.Prune(ir.ClientCtx, options)
	if err != nil {
		return nil, err
	}
	return reports, nil
}

func (ir *ImageEngine) Pull(_ context.Context, rawImage string, opts entities.ImagePullOptions) (*entities.ImagePullReport, error) {
	if opts.OciDecryptConfig != nil {
		return nil, fmt.Errorf("decryption is not supported for remote clients")
	}

	options := new(images.PullOptions)
	options.WithAllTags(opts.AllTags).WithAuthfile(opts.Authfile).WithArch(opts.Arch).WithOS(opts.OS)
	options.WithVariant(opts.Variant).WithPassword(opts.Password)
	options.WithQuiet(opts.Quiet).WithUsername(opts.Username).WithPolicy(opts.PullPolicy.String())
	options.WithProgressWriter(opts.Writer)
	if s := opts.SkipTLSVerify; s != types.OptionalBoolUndefined {
		if s == types.OptionalBoolTrue {
			options.WithSkipTLSVerify(true)
		} else {
			options.WithSkipTLSVerify(false)
		}
	}
	if opts.Retry != nil {
		options.WithRetry(*opts.Retry)
	}
	if opts.RetryDelay != "" {
		options.WithRetryDelay(opts.RetryDelay)
	}
	pulledImages, err := images.Pull(ir.ClientCtx, rawImage, options)
	if err != nil {
		return nil, err
	}
	return &entities.ImagePullReport{Images: pulledImages}, nil
}

func (ir *ImageEngine) Tag(_ context.Context, nameOrID string, tags []string, _ entities.ImageTagOptions) error {
	options := new(images.TagOptions)
	for _, newTag := range tags {
		var (
			tag, repo string
		)
		ref, err := reference.Parse(newTag)
		if err != nil {
			return fmt.Errorf("parsing reference %q: %w", newTag, err)
		}
		if t, ok := ref.(reference.Tagged); ok {
			tag = t.Tag()
		}
		if r, ok := ref.(reference.Named); ok {
			repo = r.Name()
		}
		if len(repo) < 1 {
			return fmt.Errorf("invalid image name %q", nameOrID)
		}
		if err := images.Tag(ir.ClientCtx, nameOrID, tag, repo, options); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Untag(_ context.Context, nameOrID string, tags []string, _ entities.ImageUntagOptions) error {
	options := new(images.UntagOptions)
	if len(tags) == 0 {
		return images.Untag(ir.ClientCtx, nameOrID, "", "", options)
	}

	for _, newTag := range tags {
		var (
			tag, repo string
		)
		ref, err := reference.Parse(newTag)
		if err != nil {
			return fmt.Errorf("parsing reference %q: %w", newTag, err)
		}
		if t, ok := ref.(reference.Tagged); ok {
			tag = t.Tag()
		}
		if t, ok := ref.(reference.Digested); ok {
			tag += "@" + t.Digest().String()
		}
		if r, ok := ref.(reference.Named); ok {
			repo = r.Name()
		}
		if len(repo) < 1 {
			return fmt.Errorf("invalid image name %q", nameOrID)
		}
		if err := images.Untag(ir.ClientCtx, nameOrID, tag, repo, options); err != nil {
			return err
		}
	}
	return nil
}

func (ir *ImageEngine) Inspect(_ context.Context, namesOrIDs []string, opts entities.InspectOptions) ([]*entities.ImageInspectReport, []error, error) {
	options := new(images.GetOptions).WithSize(opts.Size)
	reports := []*entities.ImageInspectReport{}
	errs := []error{}
	for _, i := range namesOrIDs {
		r, err := images.GetImage(ir.ClientCtx, i, options)
		if err != nil {
			errModel, ok := err.(*errorhandling.ErrorModel)
			if !ok {
				return nil, nil, err
			}
			if errModel.ResponseCode == 404 {
				errs = append(errs, fmt.Errorf("unable to inspect %q: %w", i, err))
				continue
			}
			return nil, nil, err
		}
		reports = append(reports, r)
	}
	return reports, errs, nil
}

func (ir *ImageEngine) Load(_ context.Context, opts entities.ImageLoadOptions) (*entities.ImageLoadReport, error) {
	if localMap, ok := localapi.CheckPathOnRunningMachine(ir.ClientCtx, opts.Input); ok {
		report, err := images.LoadLocal(ir.ClientCtx, localMap.RemotePath)
		if err == nil {
			return report, nil
		}
		var errModel *errorhandling.ErrorModel
		if errors.As(err, &errModel) {
			switch errModel.ResponseCode {
			case http.StatusNotFound, http.StatusMethodNotAllowed:
			default:
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	f, err := os.Open(opts.Input)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if fInfo.IsDir() {
		return nil, fmt.Errorf("remote client supports archives only but %q is a directory", opts.Input)
	}
	return images.Load(ir.ClientCtx, f)
}

func (ir *ImageEngine) Import(_ context.Context, opts entities.ImageImportOptions) (*entities.ImageImportReport, error) {
	var (
		err error
		f   *os.File
	)
	options := new(images.ImportOptions).WithChanges(opts.Changes).WithMessage(opts.Message).WithReference(opts.Reference)
	options.WithOS(opts.OS).WithArchitecture(opts.Architecture).WithVariant(opts.Variant)
	if opts.SourceIsURL {
		options.WithURL(opts.Source)
	} else {
		f, err = os.Open(opts.Source)
		if err != nil {
			return nil, err
		}
	}
	return images.Import(ir.ClientCtx, f, options)
}

func (ir *ImageEngine) Push(_ context.Context, source string, destination string, opts entities.ImagePushOptions) (*entities.ImagePushReport, error) {
	if opts.Signers != nil {
		return nil, fmt.Errorf("forwarding Signers is not supported for remote clients")
	}
	if opts.OciEncryptConfig != nil {
		return nil, fmt.Errorf("encryption is not supported for remote clients")
	}

	options := new(images.PushOptions)
	options.WithAll(opts.All).WithCompress(opts.Compress).WithUsername(opts.Username).WithPassword(opts.Password).WithAuthfile(opts.Authfile).WithFormat(opts.Format).WithRemoveSignatures(opts.RemoveSignatures).WithQuiet(opts.Quiet).WithCompressionFormat(opts.CompressionFormat).WithProgressWriter(opts.Writer).WithForceCompressionFormat(opts.ForceCompressionFormat)

	if opts.CompressionLevel != nil {
		options.WithCompressionLevel(*opts.CompressionLevel)
	}

	if s := opts.SkipTLSVerify; s != types.OptionalBoolUndefined {
		if s == types.OptionalBoolTrue {
			options.WithSkipTLSVerify(true)
		} else {
			options.WithSkipTLSVerify(false)
		}
	}
	if opts.Retry != nil {
		options.WithRetry(*opts.Retry)
	}
	if opts.RetryDelay != "" {
		options.WithRetryDelay(opts.RetryDelay)
	}
	if err := images.Push(ir.ClientCtx, source, destination, options); err != nil {
		return nil, err
	}
	return &entities.ImagePushReport{ManifestDigest: options.GetManifestDigest()}, nil
}

func (ir *ImageEngine) Save(_ context.Context, nameOrID string, tags []string, opts entities.ImageSaveOptions) error {
	var (
		f   *os.File
		err error
	)
	options := new(images.ExportOptions).WithFormat(opts.Format).WithCompress(opts.Compress)
	options = options.WithOciAcceptUncompressedLayers(opts.OciAcceptUncompressedLayers)

	switch opts.Format {
	case "oci-dir", "docker-dir":
		f, err = os.CreateTemp("", "podman_save")
		if err == nil {
			defer func() { _ = os.Remove(f.Name()) }()
		}
	default:
		// This is ugly but I think the best we can do for now,
		// on windows there is no /dev/stdout but the save command defaults to /dev/stdout.
		// The proper thing to do would be to pass an io.Writer down from the cli frontend
		// but since the local save API does not support an io.Writer this is impossible.
		// I reported it a while ago in https://github.com/containers/common/issues/1275
		if opts.Output == "/dev/stdout" {
			f = os.Stdout
		} else {
			// This code was added to allow for opening stdout replacing
			// os.Create(opts.Output) which was attempting to open the file
			// for read/write which fails on Darwin platforms
			f, err = os.OpenFile(opts.Output, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
		}
	}
	if err != nil {
		return err
	}

	exErr := images.Export(ir.ClientCtx, append([]string{nameOrID}, tags...), f, options)
	if err := f.Close(); err != nil {
		return err
	}
	if exErr != nil {
		return exErr
	}

	if opts.Format != "oci-dir" && opts.Format != "docker-dir" {
		return nil
	}

	f, err = os.Open(f.Name())
	if err != nil {
		return err
	}
	info, err := os.Stat(opts.Output)
	switch {
	case err == nil:
		if info.Mode().IsRegular() {
			return fmt.Errorf("%q already exists as a regular file", opts.Output)
		}
	case os.IsNotExist(err):
		if err := os.Mkdir(opts.Output, 0o755); err != nil {
			return err
		}
	default:
		return err
	}

	return archive.Untar(f, opts.Output, &archive.TarOptions{NoLchown: true})
}

func (ir *ImageEngine) Search(_ context.Context, term string, opts entities.ImageSearchOptions) ([]entities.ImageSearchReport, error) {
	mappedFilters := make(map[string][]string)
	filters, err := filter.ParseSearchFilter(opts.Filters)
	if err != nil {
		return nil, err
	}
	if stars := filters.Stars; stars > 0 {
		mappedFilters["stars"] = []string{strconv.Itoa(stars)}
	}

	if official := filters.IsOfficial; official != types.OptionalBoolUndefined {
		mappedFilters["is-official"] = []string{strconv.FormatBool(official == types.OptionalBoolTrue)}
	}

	if automated := filters.IsAutomated; automated != types.OptionalBoolUndefined {
		mappedFilters["is-automated"] = []string{strconv.FormatBool(automated == types.OptionalBoolTrue)}
	}

	options := new(images.SearchOptions)
	options.WithAuthfile(opts.Authfile).WithFilters(mappedFilters).WithLimit(opts.Limit)
	options.WithListTags(opts.ListTags).WithPassword(opts.Password).WithUsername(opts.Username)
	if s := opts.SkipTLSVerify; s != types.OptionalBoolUndefined {
		if s == types.OptionalBoolTrue {
			options.WithSkipTLSVerify(true)
		} else {
			options.WithSkipTLSVerify(false)
		}
	}
	return images.Search(ir.ClientCtx, term, options)
}

func (ir *ImageEngine) Config(_ context.Context) (*config.Config, error) {
	return config.Default()
}

func (ir *ImageEngine) Build(_ context.Context, containerFiles []string, opts entities.BuildOptions) (*entities.BuildReport, error) {
	isHyperV, err := localapi.IsHyperVProvider(ir.ClientCtx)
	if err != nil {
		logrus.Debugf("IsHyperVProvider check failed: %v", err)
	}
	// Local api is not supported on Windows Hyper-V, because 9p mounts don't translate all file attributes correctly.
	// So we skip trying to use localapi in that case.
	if !isHyperV {
		if translatedContainerFiles, translatedOptions, ok := localapi.CheckIfImageBuildPathsOnRunningMachine(ir.ClientCtx, containerFiles, opts); ok {
			report, err := images.BuildFromServerContext(ir.ClientCtx, translatedContainerFiles, translatedOptions)
			if err == nil {
				return report, nil
			}
			if err != nil {
				logrus.Debugf("BuildLocal failed: %v", err)
			}

			var errModel *errorhandling.ErrorModel
			if errors.As(err, &errModel) {
				switch errModel.ResponseCode {
				case http.StatusNotFound, http.StatusMethodNotAllowed:
				default:
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	report, err := images.Build(ir.ClientCtx, containerFiles, opts)
	if err != nil {
		return nil, err
	}
	report.SaveFormat = define.OCIArchive
	if opts.OutputFormat == bdefine.Dockerv2ImageManifest {
		report.SaveFormat = define.V2s2Archive
	}
	return report, nil
}

func (ir *ImageEngine) Tree(_ context.Context, nameOrID string, opts entities.ImageTreeOptions) (*entities.ImageTreeReport, error) {
	options := new(images.TreeOptions).WithWhatRequires(opts.WhatRequires)
	return images.Tree(ir.ClientCtx, nameOrID, options)
}

// Shutdown Libpod engine
func (ir *ImageEngine) Shutdown(_ context.Context) {
}

func (ir *ImageEngine) Sign(_ context.Context, _ []string, _ entities.SignOptions) (*entities.SignReport, error) {
	return nil, errors.New("not implemented yet")
}

func (ir *ImageEngine) Scp(_ context.Context, src, dst string, opts entities.ImageScpOptions) (*entities.ImageScpReport, error) {
	options := new(images.ScpOptions)

	var destination *string
	if len(dst) > 1 {
		destination = &dst
	}
	options.Quiet = &opts.Quiet
	options.Destination = destination

	rep, err := images.Scp(ir.ClientCtx, &src, destination, *options)
	if err != nil {
		return nil, err
	}

	fmt.Println("Loaded Image(s):", rep.Id)

	return &entities.ImageScpReport{}, nil
}
