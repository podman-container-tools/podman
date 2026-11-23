package imagesource

import (
	"context"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/internal/imagesource/stubs"
	"go.podman.io/image/v5/internal/private"
	"go.podman.io/image/v5/internal/signature"
	"go.podman.io/image/v5/transports"
	"go.podman.io/image/v5/types"
)

// NewImageSource is like ref.NewImageSourceWithOptions, but it works for
// references that only implement types.ImageReference, and returns an object that
// provides the private.ImageSource API.
//
// The caller must call .Close() on the returned ImageSource.
func NewImageSource(ctx context.Context, ref types.ImageReference, options private.NewImageSourceOptions) (private.ImageSource, error) {
	// We don’t provide this as an imagereference.FromPublic wrapper for all of ImageReference,
	// because that would depend on both imagesource.FromPublic and imagedestination.FromPublic;
	// this way, some callers might only depend on one of them.

	if ref2, ok := ref.(private.ImageReference); ok {
		return ref2.NewImageSourceWithOptions(ctx, options)
	}
	// We have no idea whether the implementation uses unwanted digests
	// different from options.Digests.mustUse; if set, we can hope for the best
	// or entirely refuse to use non-private implementations.
	//
	// If we refused entirely, that would break c/common/pkg/supplemented… while that one,
	// hopefully, doesn’t actually need the digest options.
	//
	// Ultimately we might have to make NewImageSourceWithOptions a public API??
	// But that doesn't help all that much — if we exposed the whole options struct,
	// how can we tell whether the external implementation actually handles any
	// option we might add in the future?!
	//
	// Or (??!) would we want to parse data from GetManifest (and config from GetBlob??)
	// to fail if it does not conform to options.Digests.MustUse?
	//
	// In practice, external wrappers tend to wrap docker:// and oci:…, where options.Digests
	// don’t matter.

	if mustUse := options.Digests.MustUseSet(); mustUse != "" && mustUse != digest.Canonical {
		logrus.Warnf("%q does not implement digest choices for image sources; request to use %s is ignored",
			transports.ImageName(ref), mustUse)
	}
	src, err := ref.NewImageSource(ctx, options.Sys)
	if err != nil {
		return nil, err
	}
	return FromPublic(src), nil
}

// wrapped provides the private.ImageSource operations
// for a source that only implements types.ImageSource
type wrapped struct {
	stubs.NoGetBlobAtInitialize

	types.ImageSource
}

// FromPublic(src) returns an object that provides the private.ImageSource API
//
// Internal callers should use NewImageSource instead, where possible.
//
// Eventually, we might want to expose this function, and methods of the returned object,
// as a public API (or rather, a variant that does not include the already-superseded
// methods of types.ImageSource, and has added more future-proofing), and more strongly
// deprecate direct use of types.ImageSource.
//
// NOTE: The returned API MUST NOT be a public interface (it can be either just a struct
// with public methods, or perhaps a private interface), so that we can add methods
// without breaking any external implementers of a public interface.
func FromPublic(src types.ImageSource) private.ImageSource {
	if src2, ok := src.(private.ImageSource); ok {
		return src2
	}
	return &wrapped{
		NoGetBlobAtInitialize: stubs.NoGetBlobAt(src.Reference()),

		ImageSource: src,
	}
}

// GetSignaturesWithFormat returns the image's signatures.  It may use a remote (= slow) service.
// If instanceDigest is not nil, it contains a digest of the specific manifest instance to retrieve signatures for
// (when the primary manifest is a manifest list); this never happens if the primary manifest is not a manifest list
// (e.g. if the source never returns manifest lists).
func (w *wrapped) GetSignaturesWithFormat(ctx context.Context, instanceDigest *digest.Digest) ([]signature.Signature, error) {
	sigs, err := w.GetSignatures(ctx, instanceDigest)
	if err != nil {
		return nil, err
	}
	res := []signature.Signature{}
	for _, sig := range sigs {
		res = append(res, signature.SimpleSigningFromBlob(sig))
	}
	return res, nil
}
