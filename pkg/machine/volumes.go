package machine

import (
	"crypto/sha256"
	"encoding/hex"

	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

// NFSSELinuxContext is what is used by NFS mounts, which is allowed
// access by container_t.  We need to fix the Fedora selinux policy
// to just allow access to virtiofs_t.
const NFSSELinuxContext = "system_u:object_r:nfs_t:s0"

type Volume interface {
	Kind() VolumeKind
}

type VolumeKind string

var (
	VirtIOFsVk VolumeKind = "virtiofs"
	NinePVk    VolumeKind = "9p"
)

type VirtIoFs struct {
	VolumeKind
	ReadOnly bool
	Source   string
	Tag      string
	Target   string
}

func (v VirtIoFs) Kind() string {
	return string(VirtIOFsVk)
}

// generateTag generates a tag for VirtIOFs mounts.
// AppleHV requires tags to be 36 bytes or fewer.
// SHA256 the path, then truncate to 36 bytes
func (v VirtIoFs) generateTag() string {
	sum := sha256.Sum256([]byte(v.Target))
	stringSum := hex.EncodeToString(sum[:])
	return stringSum[:36]
}

func (v VirtIoFs) ToMount() vmconfigs.Mount {
	return vmconfigs.Mount{
		ReadOnly: v.ReadOnly,
		Source:   v.Source,
		Tag:      v.Tag,
		Target:   v.Target,
		Type:     v.Kind(),
	}
}

// NewVirtIoFsMount describes a machine volume mount for virtio-fs.  With virtio-fs
// the source/target are described as a "shared dir".  With this style of volume mount
// the Tag is used as the descriptor value for the mount (in Linux).
func NewVirtIoFsMount(src, target string, readOnly bool) VirtIoFs {
	vfs := VirtIoFs{
		ReadOnly: readOnly,
		Source:   src,
		Target:   target,
	}
	vfs.Tag = vfs.generateTag()
	return vfs
}

func MountToVirtIOFs(mnt *vmconfigs.Mount) VirtIoFs {
	return VirtIoFs{
		VolumeKind: VirtIOFsVk,
		ReadOnly:   mnt.ReadOnly,
		Source:     mnt.Source,
		Tag:        mnt.Tag,
		Target:     mnt.Target,
	}
}
