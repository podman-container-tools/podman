package shim

import (
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func CmdLineVolumesToMounts(volumes []string, volumeType vmconfigs.VolumeMountType) []*vmconfigs.Mount {
	mounts := []*vmconfigs.Mount{}
	for i, volume := range volumes {
		if volume == "" {
			continue
		}
		var mount vmconfigs.Mount
		tag, source, target, readOnly, _ := vmconfigs.SplitVolume(i, volume)
		switch volumeType {
		case vmconfigs.VirtIOFS:
			virtioMount := machine.NewVirtIoFsMount(source, target, readOnly)
			mount = virtioMount.ToMount()
		default:
			mount = vmconfigs.Mount{
				Type:          volumeType.String(),
				Tag:           tag,
				Source:        source,
				Target:        target,
				ReadOnly:      readOnly,
				OriginalInput: volume,
			}
		}
		mounts = append(mounts, &mount)
	}
	return mounts
}
