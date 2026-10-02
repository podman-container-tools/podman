//go:build !remote && (linux || freebsd)

package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
	"go.podman.io/podman/v6/pkg/k8s.io/apimachinery/pkg/api/resource"
)

func TestVolumeFromEmptyDir(t *testing.T) {
	emptyDirSource := v1.EmptyDirVolumeSource{}
	emptyDirVol, err := VolumeFromEmptyDir(&emptyDirSource, "emptydir")
	assert.NoError(t, err)
	assert.Equal(t, emptyDirVol.Type, KubeVolumeTypeEmptyDir)

	memEmptyDirSource := v1.EmptyDirVolumeSource{
		Medium: v1.StorageMediumMemory,
	}
	memEmptyDirVol, err := VolumeFromEmptyDir(&memEmptyDirSource, "emptydir")
	assert.NoError(t, err)
	assert.Equal(t, memEmptyDirVol.Type, KubeVolumeTypeEmptyDirTmpfs)
	assert.Zero(t, memEmptyDirVol.SizeLimit)

	sizeLimit := resource.MustParse("64Mi")
	sizedEmptyDirSource := v1.EmptyDirVolumeSource{
		Medium:    v1.StorageMediumMemory,
		SizeLimit: &sizeLimit,
	}
	sizedEmptyDirVol, err := VolumeFromEmptyDir(&sizedEmptyDirSource, "emptydir")
	assert.NoError(t, err)
	assert.Equal(t, sizedEmptyDirVol.Type, KubeVolumeTypeEmptyDirTmpfs)
	assert.Equal(t, int64(64*1024*1024), sizedEmptyDirVol.SizeLimit)
}

func TestMemoryEmptyDirOptions(t *testing.T) {
	assert.Equal(t, []string{"volume-opt=type=tmpfs"}, MemoryEmptyDirOptions(0))
	assert.Equal(t, []string{"volume-opt=type=tmpfs", "volume-opt=o=size=67108864"}, MemoryEmptyDirOptions(64*1024*1024))
}
