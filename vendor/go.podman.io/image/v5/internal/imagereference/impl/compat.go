package impl

import (
	"context"

	"go.podman.io/image/v5/internal/digests"
	"go.podman.io/image/v5/internal/private"
	"go.podman.io/image/v5/types"
)

// This does not define a Compat struct similar to imagesource/impl, because that
// would ~require the ImageReference implementations to use a pointer receiver,
// and structurally they are much closer to value types.
//
// (In particular, the c/storage transport copies reference struct by value in some code,
// and it’s easier to keep it that way.)

// NewImageSource implements types.ImageReference.NewImageSource for private.ImageReference.
func NewImageSource(ref private.ImageReferenceInternalOnly, ctx context.Context, sys *types.SystemContext) (types.ImageSource, error) {
	return ref.NewImageSourceWithOptions(ctx, private.NewImageSourceOptions{
		Sys:     sys,
		Digests: digests.CanonicalDefault(),
	})
}
