package libartifact

import (
	"errors"

	"go.podman.io/common/pkg/libartifact/types"
	"go.podman.io/image/v5/docker/reference"
)

type ArtifactStoreReference struct {
	ref            *reference.Named
	possibleDigest string
}

// NewArtifactStorageReference refers to a theoretical object already in the artifact store.  It
// can be a name or a full or partial digest. If this object exists in the store is unknown at this
// time.
func NewArtifactStorageReference(nameOrDigest string) (ArtifactStoreReference, error) {
	asr := ArtifactStoreReference{}
	if len(nameOrDigest) == 0 {
		return asr, errors.New("nameOrDigest cannot be empty")
	}
	// Try to parse as a valid OCI reference
	named, err := stringToNamed(nameOrDigest)
	if errors.Is(err, types.ErrTaggedAndDigested) {
		return asr, err
	}
	if err == nil {
		asr.ref = &named
		return asr, nil
	}
	// The input is not a valid oci ref, so we store the input as possible digest
	asr.possibleDigest = nameOrDigest
	return asr, nil
}
