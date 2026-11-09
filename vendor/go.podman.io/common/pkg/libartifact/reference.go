package libartifact

import (
	"github.com/opencontainers/go-digest"
	"go.podman.io/common/pkg/libartifact/types"
	"go.podman.io/image/v5/docker/reference"
)

// ArtifactReference is a fully qualified oci reference except for tag, where we add
// "latest" as the tag if tag is empty.  Valid references:
//
// quay.io/podman/machine-os:latest
// quay.io/podman/machine-os
// quay.io/podman/machine-os@sha256:916ede4b2b9012f91f63100f8ba82d07ed81bf8a55d23c1503285a22a9759a1e
//
// Note: Partial sha references and digests (IDs) are not allowed.
// Note: A zero value of ArtifactReference{} is not valid because it violates
// the format promise.
// Note: repo:tag@digest are invalid.
type ArtifactReference struct {
	ref reference.Named
}

// Name returns the full reference without the tag.
func (ar ArtifactReference) RepoName() string {
	return ar.ref.Name()
}

// String returns the full reference.
func (ar ArtifactReference) String() string {
	return ar.ref.String()
}

// Named returns the parsed reference for c/image docker transport and related APIs.
func (ar ArtifactReference) Named() reference.Named {
	return ar.ref
}

// DigestReference returns repo@digest for OCI index org.opencontainers.image.ref.name annotations.
func (ar ArtifactReference) DigestReference(d digest.Digest) (string, error) {
	digested, err := reference.WithDigest(reference.TrimNamed(ar.ref), d)
	if err != nil {
		return "", err
	}
	return digested.String(), nil
}

func (ar ArtifactReference) ToArtifactStoreReference() ArtifactStoreReference {
	afr := ArtifactStoreReference{
		ref: &ar.ref,
	}
	return afr
}

// NewArtifactReference is a theoretical reference to an artifact.
func NewArtifactReference(input string) (ArtifactReference, error) {
	named, err := stringToNamed(input)
	if err != nil {
		return ArtifactReference{}, err
	}
	return ArtifactReference{ref: named}, nil
}

// stringToNamed converts a string to a reference.Named.
func stringToNamed(s string) (reference.Named, error) {
	named, err := reference.ParseNamed(s)
	if err != nil {
		return nil, err
	}
	_, isTagged := named.(reference.NamedTagged)
	_, isDigested := named.(reference.Digested)

	if isTagged && isDigested {
		return nil, types.ErrTaggedAndDigested
	}

	return reference.TagNameOnly(named), nil
}
