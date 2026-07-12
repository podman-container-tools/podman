package copy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	encconfig "github.com/containers/ocicrypt/config"
	digest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/docker/reference"
	internalblobinfocache "go.podman.io/image/v5/internal/blobinfocache"
	"go.podman.io/image/v5/internal/digests"
	"go.podman.io/image/v5/internal/image"
	"go.podman.io/image/v5/internal/imagedestination"
	"go.podman.io/image/v5/internal/imagesource"
	internalManifest "go.podman.io/image/v5/internal/manifest"
	"go.podman.io/image/v5/internal/private"
	"go.podman.io/image/v5/manifest"
	"go.podman.io/image/v5/pkg/blobinfocache"
	compression "go.podman.io/image/v5/pkg/compression/types"
	"go.podman.io/image/v5/signature"
	"go.podman.io/image/v5/signature/signer"
	"go.podman.io/image/v5/transports"
	"go.podman.io/image/v5/types"
	"golang.org/x/sync/semaphore"
	"golang.org/x/term"
)

var (
	// ErrDecryptParamsMissing is returned if there is missing decryption parameters
	ErrDecryptParamsMissing = errors.New("Necessary DecryptParameters not present")

	// maxParallelDownloads is used to limit the maximum number of parallel
	// downloads.  Let's follow Firefox by limiting it to 6.
	maxParallelDownloads = uint(6)
)

// InstancePlatformFilter specifies a platform by OS and Architecture for filtering instances
type InstancePlatformFilter struct {
	OS           string // OS, e.g., "linux"
	Architecture string // Architecture, e.g., "amd64"
}

const (
	// CopySystemImage is the default value which, when set in
	// Options.ImageListSelection, indicates that the caller expects only one
	// image to be copied, so if the source reference refers to a list of
	// images, one that matches the current system will be selected.
	CopySystemImage ImageListSelection = iota
	// CopyAllImages is a value which, when set in Options.ImageListSelection,
	// indicates that the caller expects to copy multiple images, and if
	// the source reference refers to a list, that the list and every image
	// to which it refers will be copied.  If the source reference refers
	// to a list, the target reference can not accept lists, an error
	// should be returned.
	CopyAllImages
	// CopySpecificImages is a value which, when set in
	// Options.ImageListSelection, indicates that the caller expects the
	// source reference to be either a single image or a list of images,
	// and if the source reference is a list, wants only specific instances
	// from it copied (or none of them, if the list of instances to copy is
	// empty), along with the list itself.  If the target reference can
	// only accept one image (i.e., it cannot accept lists), an error
	// should be returned.
	CopySpecificImages
)

// ImageListSelection is one of CopySystemImage, CopyAllImages, or
// CopySpecificImages, to control whether, when the source reference is a list,
// copy.Image() copies only an image which matches the current runtime
// environment, or all images which match the supplied reference, or only
// specific images from the source reference.
type ImageListSelection int

const (
	// KeepSparseManifestList is the default value which, when set in
	// Options.SparseManifestListAction, indicates that the manifest is kept
	// as is even though some images from the list may be missing. Some
	// registries may not support this.
	KeepSparseManifestList SparseManifestListAction = iota

	// StripSparseManifestList will strip missing images from the manifest
	// list. When images are stripped the digest will differ from the original.
	StripSparseManifestList
)

// SparseManifestListAction is one of KeepSparseManifestList or StripSparseManifestList
// to control the behavior when only a subset of images from a manifest list is copied
type SparseManifestListAction int

// Options allows supplying non-default configuration modifying the behavior of CopyImage.
type Options struct {
	RemoveSignatures bool // Remove any pre-existing signatures. Signers and SignBy… will still add a new signature.
	// RemoveListSignatures removes the manifest list signature while preserving per-instance signatures.
	// If RemoveSignatures is also true, RemoveSignatures takes precedence.
	RemoveListSignatures bool
	// Signers to use to add signatures during the copy.
	// Callers are still responsible for closing these Signer objects; they can be reused for multiple copy.Image operations in a row.
	Signers                          []*signer.Signer
	SignBy                           string          // If non-empty, asks for a signature to be added during the copy, and specifies a key ID, as accepted by signature.NewGPGSigningMechanism().SignDockerManifest(),
	SignPassphrase                   string          // Passphrase to use when signing with the key ID from `SignBy`.
	SignBySigstorePrivateKeyFile     string          // If non-empty, asks for a signature to be added during the copy, using a sigstore private key file at the provided path.
	SignSigstorePrivateKeyPassphrase []byte          // Passphrase to use when signing with `SignBySigstorePrivateKeyFile`.
	SignIdentity                     reference.Named // Identify to use when signing, defaults to the docker reference of the destination

	ReportWriter     io.Writer
	SourceCtx        *types.SystemContext
	DestinationCtx   *types.SystemContext
	ProgressInterval time.Duration                 // time to wait between reports to signal the progress channel
	Progress         chan types.ProgressProperties // Reported to when ProgressInterval has arrived for a single artifact+offset.

	// Preserve digests, and fail if we cannot.
	PreserveDigests bool
	// manifest MIME type of image set by user. "" is default and means use the autodetection to the manifest MIME type
	ForceManifestMIMEType string
	ImageListSelection    ImageListSelection       // set to either CopySystemImage (the default), CopyAllImages, or CopySpecificImages to control which instances we copy when the source reference is a list; ignored if the source reference is not a list
	Instances             []digest.Digest          // if ImageListSelection is CopySpecificImages, copy only these instances, instances matching the InstancePlatforms list, and the list itself
	InstancePlatforms     []InstancePlatformFilter // if ImageListSelection is CopySpecificImages, copy instances with matching OS/Architecture (all variants and compressions), it also copies the index/manifest_list instance.
	// When only a subset of images of a list is copied, this action indicates if the manifest should be kept or stripped.
	// See CopySpecificImages.
	SparseManifestListAction SparseManifestListAction
	// Give priority to pulling gzip images if multiple images are present when configured to OptionalBoolTrue,
	// prefers the best compression if this is configured as OptionalBoolFalse. Choose automatically (and the choice may change over time)
	// if this is set to OptionalBoolUndefined (which is the default behavior, and recommended for most callers).
	// This only affects CopySystemImage.
	PreferGzipInstances types.OptionalBool

	// If OciEncryptConfig is non-nil, it indicates that an image should be encrypted.
	// The encryption options is derived from the construction of EncryptConfig object.
	OciEncryptConfig *encconfig.EncryptConfig
	// OciEncryptLayers represents the list of layers to encrypt.
	// If nil, don't encrypt any layers.
	// If non-nil and len==0, denotes encrypt all layers.
	// integers in the slice represent 0-indexed layer indices, with support for negative
	// indexing. i.e. 0 is the first layer, -1 is the last (top-most) layer.
	OciEncryptLayers *[]int
	// OciDecryptConfig contains the config that can be used to decrypt an image if it is
	// encrypted if non-nil. If nil, it does not attempt to decrypt an image.
	OciDecryptConfig *encconfig.DecryptConfig

	// A weighted semaphore to limit the amount of concurrently copied layers and configs. Applies to all copy operations using the semaphore. If set, MaxParallelDownloads is ignored.
	ConcurrentBlobCopiesSemaphore *semaphore.Weighted

	// MaxParallelDownloads indicates the maximum layers to pull at the same time. Applies to a single copy operation. A reasonable default is used if this is left as 0. Ignored if ConcurrentBlobCopiesSemaphore is set.
	MaxParallelDownloads uint

	// When OptimizeDestinationImageAlreadyExists is set, optimize the copy assuming that the destination image already
	// exists (and is equivalent). Making the eventual (no-op) copy more performant for this case. Enabling the option
	// is slightly pessimistic if the destination image doesn't exist, or is not equivalent.
	OptimizeDestinationImageAlreadyExists bool

	// Download layer contents with "nondistributable" media types ("foreign" layers) and translate the layer media type
	// to not indicate "nondistributable".
	DownloadForeignLayers bool

	// Contains slice of OptionCompressionVariant, where copy will ensure that for each platform
	// in the manifest list, a variant with the requested compression will exist.
	// Invalid when copying a non-multi-architecture image. That will probably
	// change in the future.
	EnsureCompressionVariantsExist []OptionCompressionVariant
	// ForceCompressionFormat ensures that the compression algorithm set in
	// DestinationCtx.CompressionFormat is used exclusively, and blobs of other
	// compression algorithms are not reused.
	ForceCompressionFormat bool

	// ReportResolvedReference, if set, asks the destination transport to store
	// a “resolved” (more detailed) reference to the created image
	// into the value this option points to.
	// What “resolved” means is transport-specific.
	// Most transports don’t support this, and cause the value to be set to nil.
	//
	// For the containers-storage: transport, the reference contains an image ID,
	// so that storage.ResolveReference returns exactly the created image.
	// WARNING: It is unspecified whether the reference also contains a reference.Named element.
	ReportResolvedReference *types.ImageReference

	// DestinationTimestamp, if set, will force timestamps of content created in the destination to this value.
	// Most transports don't support this.
	//
	// In oci-archive: destinations, this will set the create/mod/access timestamps in each tar entry
	// (but not a timestamp of the created archive file).
	DestinationTimestamp *time.Time

	// FIXME:
	// - this reference to an internal type is unusable from the outside even if we made the field public
	// - what is the actual semantics? Right now it is probably “choices to use when writing to the destination”, TBD
	// - anyway do we want to expose _all_ of the digests.Options tunables, or fewer?
	// - … do we want to expose _more_ granularity than that?
	//   - (“must have at least sha512 integrity when reading”, what does “at least” mean for random pairs of algorithms?)
	//   - should some of this be in config files, maybe ever per-registry?
	digestOptions digests.Options
}

// OptionCompressionVariant allows to supply information about
// selected compression algorithm and compression level by the
// end-user. Refer to EnsureCompressionVariantsExist to know
// more about its usage.
type OptionCompressionVariant struct {
	Algorithm compression.Algorithm
	Level     *int // Only used when we are creating a new image instance using the specified algorithm, not when the image already contains such an instance
}

// copier allows us to keep track of diffID values for blobs, and other
// data shared across one or more images in a possible manifest list.
// The owner must call close() when done.
type copier struct {
	policyContext *signature.PolicyContext
	dest          private.ImageDestination
	rawSource     private.ImageSource
	options       *Options // never nil

	reportWriter   io.Writer
	progressOutput io.Writer

	unparsedToplevel              *image.UnparsedImage // for rawSource
	blobInfoCache                 internalblobinfocache.BlobInfoCache2
	concurrentBlobCopiesSemaphore *semaphore.Weighted // Limits the amount of concurrently copied blobs
	signers                       []*signer.Signer    // Signers to use to create new signatures for the image
	signersToClose                []*signer.Signer    // Signers that should be closed when this copier is destroyed.
}

// Internal function to validate `requireCompressionFormatMatch` for copySingleImageOptions
func shouldRequireCompressionFormatMatch(options *Options) (bool, error) {
	if options.ForceCompressionFormat && (options.DestinationCtx == nil || options.DestinationCtx.CompressionFormat == nil) {
		return false, fmt.Errorf("cannot use ForceCompressionFormat with undefined default compression format")
	}
	return options.ForceCompressionFormat, nil
}

// Image copies image from srcRef to destRef, using policyContext to validate
// source image admissibility.  It returns the manifest which was written to
// the new copy of the image.
func Image(ctx context.Context, policyContext *signature.PolicyContext, destRef, srcRef types.ImageReference, options *Options) (copiedManifest []byte, retErr error) {
	if options == nil {
		options = &Options{}
	}
	// FIXME: Currently, digestsOptions is not implemented at all, and exists in the codebase
	// only to allow gradually building the feature set.
	// After c/image/copy consistently implements it, provide a public digest options API of some kind.
	optionsCopy := *options
	optionsCopy.digestOptions = digests.CanonicalDefault()
	options = &optionsCopy

	if err := validateImageListSelection(options.ImageListSelection); err != nil {
		return nil, err
	}

	reportWriter := io.Discard

	if options.ReportWriter != nil {
		reportWriter = options.ReportWriter
	}

	// safeClose amends retErr with an error from c.Close(), if any.
	safeClose := func(name string, c io.Closer) {
		err := c.Close()
		if err == nil {
			return
		}
		// Do not use %w for err as we don't want it to be unwrapped by callers.
		if retErr != nil {
			retErr = fmt.Errorf(" (%s: %s): %w", name, err.Error(), retErr)
		} else {
			retErr = fmt.Errorf(" (%s: %s)", name, err.Error())
		}
	}

	publicDest, err := destRef.NewImageDestination(ctx, options.DestinationCtx)
	if err != nil {
		return nil, fmt.Errorf("initializing destination %s: %w", transports.ImageName(destRef), err)
	}
	dest := imagedestination.FromPublic(publicDest)
	defer safeClose("dest", dest)

	publicRawSource, err := srcRef.NewImageSource(ctx, options.SourceCtx)
	if err != nil {
		return nil, fmt.Errorf("initializing source %s: %w", transports.ImageName(srcRef), err)
	}
	rawSource := imagesource.FromPublic(publicRawSource)
	defer safeClose("src", rawSource)

	// If reportWriter is not a TTY (e.g., when piping to a file), do not
	// print the progress bars to avoid long and hard to parse output.
	// Instead use printCopyInfo() to print single line "Copying ..." messages.
	progressOutput := reportWriter
	if !isTTY(reportWriter) {
		progressOutput = io.Discard
	}

	c := &copier{
		policyContext: policyContext,
		dest:          dest,
		rawSource:     rawSource,
		options:       options,

		reportWriter:   reportWriter,
		progressOutput: progressOutput,

		unparsedToplevel: image.UnparsedInstance(rawSource, nil),
		// FIXME? The cache is used for sources and destinations equally, but we only have a SourceCtx and DestinationCtx.
		// For now, use DestinationCtx (because blob reuse changes the behavior of the destination side more).
		// Conceptually the cache settings should be in copy.Options instead.
		blobInfoCache: internalblobinfocache.FromBlobInfoCache(blobinfocache.DefaultCache(options.DestinationCtx)),
	}
	defer c.close()
	c.blobInfoCache.Open()
	defer c.blobInfoCache.Close()

	// Set the concurrentBlobCopiesSemaphore if we can copy layers in parallel.
	if dest.HasThreadSafePutBlob() && rawSource.HasThreadSafeGetBlob() {
		c.concurrentBlobCopiesSemaphore = c.options.ConcurrentBlobCopiesSemaphore
		if c.concurrentBlobCopiesSemaphore == nil {
			max := c.options.MaxParallelDownloads
			if max == 0 {
				max = maxParallelDownloads
			}
			c.concurrentBlobCopiesSemaphore = semaphore.NewWeighted(int64(max))
		}
	} else {
		c.concurrentBlobCopiesSemaphore = semaphore.NewWeighted(int64(1))
		if c.options.ConcurrentBlobCopiesSemaphore != nil {
			if err := c.options.ConcurrentBlobCopiesSemaphore.Acquire(ctx, 1); err != nil {
				return nil, fmt.Errorf("acquiring semaphore for concurrent blob copies: %w", err)
			}
			defer c.options.ConcurrentBlobCopiesSemaphore.Release(1)
		}
	}

	if err := c.setupSigners(); err != nil {
		return nil, err
	}

	multiImage, err := isMultiImage(ctx, c.unparsedToplevel)
	if err != nil {
		return nil, fmt.Errorf("determining manifest MIME type for %s: %w", transports.ImageName(srcRef), err)
	}

	var singleInstance *image.UnparsedImage
	var singleInstanceErrorWrapping string
	if !multiImage {
		// The simple case: just copy a single image.
		singleInstance = c.unparsedToplevel
		singleInstanceErrorWrapping = ""
	} else if c.options.ImageListSelection == CopySystemImage {
		// This is a manifest list, and we weren't asked to copy multiple images.
		// Choose a single image that matches the current system to copy.
		mfest, manifestType, err := c.unparsedToplevel.Manifest(ctx)
		if err != nil {
			return nil, fmt.Errorf("reading manifest for %s: %w", transports.ImageName(srcRef), err)
		}
		manifestList, err := internalManifest.ListFromBlob(mfest, manifestType)
		if err != nil {
			return nil, fmt.Errorf("parsing primary manifest as list for %s: %w", transports.ImageName(srcRef), err)
		}
		instanceDigest, err := manifestList.ChooseInstanceByCompression(c.options.SourceCtx, c.options.PreferGzipInstances) // try to pick one that matches c.options.SourceCtx
		if err != nil {
			return nil, fmt.Errorf("choosing an image from manifest list %s: %w", transports.ImageName(srcRef), err)
		}
		logrus.Debugf("Source is a manifest list; copying (only) instance %s for current system", instanceDigest)
		singleInstance = image.UnparsedInstance(rawSource, &instanceDigest)
		singleInstanceErrorWrapping = "copying system image from manifest list"
	} // else: a multi-instance copy

	if singleInstance != nil {
		if len(options.EnsureCompressionVariantsExist) > 0 {
			return nil, fmt.Errorf("EnsureCompressionVariantsExist is not implemented when not creating a multi-architecture image")
		}
		if c.options.RemoveListSignatures {
			return nil, fmt.Errorf("RemoveListSignatures is not applicable when copying a single instance")
		}
		requireCompressionFormatMatch, err := shouldRequireCompressionFormatMatch(options)
		if err != nil {
			return nil, err
		}
		single, err := c.copySingleImage(ctx, singleInstance, nil, copySingleImageOptions{requireCompressionFormatMatch: requireCompressionFormatMatch})
		if err != nil {
			if singleInstanceErrorWrapping != "" {
				return nil, fmt.Errorf("%s: %w", singleInstanceErrorWrapping, err)
			}
			return nil, err
		}
		copiedManifest = single.manifest
	} else {
		// If we were asked to copy multiple images and can't, that's an error.
		if !supportsMultipleImages(c.dest) {
			return nil, fmt.Errorf("copying multiple images: destination transport %q does not support copying multiple images as a group", destRef.Transport().Name())
		}
		// Copy some or all of the images.
		switch c.options.ImageListSelection {
		case CopyAllImages:
			logrus.Debugf("Source is a manifest list; copying all instances")
		case CopySpecificImages:
			logrus.Debugf("Source is a manifest list; copying some instances")
		}
		if copiedManifest, err = c.copyMultipleImages(ctx); err != nil {
			return nil, err
		}
	}

	if options.ReportResolvedReference != nil {
		*options.ReportResolvedReference = nil // The default outcome, if not specifically supported by the transport.
	}
	if err := c.dest.CommitWithOptions(ctx, private.CommitOptions{
		UnparsedToplevel:        c.unparsedToplevel,
		ReportResolvedReference: options.ReportResolvedReference,
		Timestamp:               options.DestinationTimestamp,
	}); err != nil {
		return nil, fmt.Errorf("committing the finished image: %w", err)
	}

	return copiedManifest, nil
}

// Printf writes a formatted string to c.reportWriter.
// Note that the method name Printf is not entirely arbitrary: (go tool vet)
// has a built-in list of functions/methods (whatever object they are for)
// which have their format strings checked; for other names we would have
// to pass a parameter to every (go tool vet) invocation.
func (c *copier) Printf(format string, a ...any) {
	fmt.Fprintf(c.reportWriter, format, a...)
}

// close tears down state owned by copier.
func (c *copier) close() {
	for i, s := range c.signersToClose {
		if err := s.Close(); err != nil {
			logrus.Warnf("Error closing per-copy signer %d: %v", i+1, err)
		}
	}
}

// validateImageListSelection returns an error if the passed-in value is not one that we recognize as a valid ImageListSelection value
func validateImageListSelection(selection ImageListSelection) error {
	switch selection {
	case CopySystemImage, CopyAllImages, CopySpecificImages:
		return nil
	default:
		return fmt.Errorf("Invalid value for options.ImageListSelection: %d", selection)
	}
}

// Checks if the destination supports accepting multiple images by checking if it can support
// manifest types that are lists of other manifests.
func supportsMultipleImages(dest types.ImageDestination) bool {
	mtypes := dest.SupportedManifestMIMETypes()
	if len(mtypes) == 0 {
		// Anything goes!
		return true
	}
	return slices.ContainsFunc(mtypes, manifest.MIMETypeIsMultiImage)
}

// isTTY returns true if the io.Writer is a file and a tty.
func isTTY(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return false
}
