//go:build !containers_image_storage_stub

package storage

import (
	"fmt"
	"os"

	"go.podman.io/image/v5/types"
	"go.podman.io/storage"
)

// SplitFDStreamSocket returns a socket on which the store src reads from
// serves org.composefs.Oci for the image src reads.  src has to be an image
// source of this transport.  The caller is responsible for closing the
// returned file.
//
// The socket serves the image's layers by their position in it, as the
// store records them; see storage.SplitFDStreamStore.Socket for what the
// client is responsible for.
//
// This API is experimental and can be changed without bumping the major version number.
func SplitFDStreamSocket(src types.ImageSource) (*os.File, error) {
	s, ok := src.(*storageImageSource)
	if !ok {
		return nil, fmt.Errorf("image source %s is not from the containers-storage transport", src.Reference().Transport().Name())
	}
	sfds, err := storage.SplitFDStreamStoreFor(s.imageRef.transport.store)
	if err != nil {
		return nil, fmt.Errorf("store of image %q does not support splitfdstream: %w", s.image.ID, err)
	}
	return sfds.Socket(s.image.ID)
}
