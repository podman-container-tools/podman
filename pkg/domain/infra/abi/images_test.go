//go:build !remote && (linux || freebsd)

package abi

import (
	"testing"

	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"go.podman.io/common/libimage"
	commonDefine "go.podman.io/common/libimage/define"
	"go.podman.io/image/v5/manifest"
)

// This is really intended to verify what happens with a
// nil pointer in layer.Created, but we'll just sanity
// check round tripping 42.
func TestToDomainHistoryLayer(t *testing.T) {
	var layer libimage.ImageHistory
	layer.Size = 42
	newLayer := toDomainHistoryLayer(&layer)
	assert.Equal(t, layer.Size, newLayer.Size)
}

func TestMatchesPlatform(t *testing.T) {
	tests := []struct {
		name      string
		requested v1.Platform
		actual    v1.Platform
		want      bool
	}{
		{
			name:      "normalizes architecture",
			requested: v1.Platform{OS: "linux", Architecture: "x86_64"},
			actual:    v1.Platform{OS: "linux", Architecture: "amd64"},
			want:      true,
		},
		{
			name:      "omitted variant matches",
			requested: v1.Platform{OS: "linux", Architecture: "arm64"},
			actual:    v1.Platform{OS: "linux", Architecture: "arm64", Variant: "v8"},
			want:      true,
		},
		{
			name:      "different variant does not match",
			requested: v1.Platform{OS: "linux", Architecture: "arm", Variant: "v7"},
			actual:    v1.Platform{OS: "linux", Architecture: "arm", Variant: "v6"},
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, matchesPlatform(tt.requested, tt.actual))
		})
	}
}

func TestManifestDigestsForPlatformsIncludesAttestations(t *testing.T) {
	imageDigest := digest.Digest("sha256:image")
	attestationDigest := digest.Digest("sha256:attestation")
	unrelatedAttestationDigest := digest.Digest("sha256:unrelated-attestation")
	descriptors := []commonDefine.ManifestListDescriptor{
		{
			Schema2Descriptor: manifest.Schema2Descriptor{Digest: imageDigest},
			Platform:          manifest.Schema2PlatformSpec{OS: "linux", Architecture: "amd64"},
		},
		{
			Schema2Descriptor: manifest.Schema2Descriptor{Digest: attestationDigest},
			Platform:          manifest.Schema2PlatformSpec{OS: "unknown", Architecture: "unknown"},
			Annotations:       map[string]string{"vnd.docker.reference.digest": imageDigest.String()},
		},
		{
			Schema2Descriptor: manifest.Schema2Descriptor{Digest: unrelatedAttestationDigest},
			Platform:          manifest.Schema2PlatformSpec{OS: "unknown", Architecture: "unknown"},
			Annotations:       map[string]string{"vnd.docker.reference.digest": "sha256:other"},
		},
	}

	got, matched := manifestDigestsForPlatforms(descriptors, []v1.Platform{
		{OS: "linux", Architecture: "amd64"},
	})

	assert.Equal(t, []digest.Digest{imageDigest, attestationDigest}, got)
	assert.Equal(t, []bool{true}, matched)
}
