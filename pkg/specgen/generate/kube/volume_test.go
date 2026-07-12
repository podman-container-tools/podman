//go:build !remote && (linux || freebsd)

package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "go.podman.io/podman/v6/pkg/k8s.io/api/core/v1"
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
}
