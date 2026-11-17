package metadata

import (
	"github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"go.podman.io/buildah/docker"
)

// Build constructs a map containing the passed-in information about a just-committed or reused-as-cache image.
func Build(imageConfigDigest digest.Digest, descriptor v1.Descriptor) (map[string]any, error) {
	metadata := make(map[string]any)
	if imageConfigDigest.Validate() == nil {
		metadata[docker.ExporterImageConfigDigestKey] = imageConfigDigest.String()
	}
	if descriptor.MediaType != "" && descriptor.Digest.Validate() == nil && descriptor.Size > 0 {
		metadata[docker.ExporterImageDescriptorKey] = descriptor
	}
	if descriptor.Digest.Validate() == nil {
		metadata[docker.ExporterImageDigestKey] = descriptor.Digest
	}
	return metadata, nil
}
