package machine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func TestNewVirtIoFsMount(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		target   string
		readOnly bool
	}{
		{
			name:     "read-only virtiofs mount",
			src:      "/host/path",
			target:   "/guest/path",
			readOnly: true,
		},
		{
			name:     "read-write virtiofs mount",
			src:      "/host/path",
			target:   "/guest/path",
			readOnly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vfs := NewVirtIoFsMount(tt.src, tt.target, tt.readOnly)
			assert.Equal(t, tt.readOnly, vfs.ReadOnly)
			assert.Equal(t, tt.src, vfs.Source)
			assert.Equal(t, tt.target, vfs.Target)
			assert.NotEmpty(t, vfs.Tag)
			assert.Equal(t, "virtiofs", vfs.Kind())
		})
	}
}

func TestVirtIoFsToMount(t *testing.T) {
	tests := []struct {
		name string
		vfs  VirtIoFs
	}{
		{
			name: "converts read-only VirtIoFs to Mount",
			vfs: VirtIoFs{
				VolumeKind: VirtIOFsVk,
				ReadOnly:   true,
				Source:     "/host/ro",
				Tag:        "tag-ro",
				Target:     "/guest/ro",
			},
		},
		{
			name: "converts read-write VirtIoFs to Mount",
			vfs: VirtIoFs{
				VolumeKind: VirtIOFsVk,
				ReadOnly:   false,
				Source:     "/host/rw",
				Tag:        "tag-rw",
				Target:     "/guest/rw",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mnt := tt.vfs.ToMount()
			assert.Equal(t, tt.vfs.ReadOnly, mnt.ReadOnly)
			assert.Equal(t, tt.vfs.Source, mnt.Source)
			assert.Equal(t, tt.vfs.Target, mnt.Target)
			assert.Equal(t, tt.vfs.Tag, mnt.Tag)
			assert.Equal(t, "virtiofs", mnt.Type)
		})
	}
}

func TestMountToVirtIOFs(t *testing.T) {
	tests := []struct {
		name  string
		mount *vmconfigs.Mount
	}{
		{
			name: "translates read-only vmconfigs.Mount to VirtIoFs",
			mount: &vmconfigs.Mount{
				ReadOnly: true,
				Source:   "/host/ro",
				Tag:      "tag-ro",
				Target:   "/guest/ro",
				Type:     "virtiofs",
			},
		},
		{
			name: "translates read-write vmconfigs.Mount to VirtIoFs",
			mount: &vmconfigs.Mount{
				ReadOnly: false,
				Source:   "/host/rw",
				Tag:      "tag-rw",
				Target:   "/guest/rw",
				Type:     "virtiofs",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vfs := MountToVirtIOFs(tt.mount)
			assert.Equal(t, tt.mount.ReadOnly, vfs.ReadOnly)
			assert.Equal(t, tt.mount.Source, vfs.Source)
			assert.Equal(t, tt.mount.Target, vfs.Target)
			assert.Equal(t, tt.mount.Tag, vfs.Tag)
			assert.Equal(t, VirtIOFsVk, vfs.VolumeKind)
		})
	}
}
