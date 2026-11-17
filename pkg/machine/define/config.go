package define

import "os"

const (
	UserCertsTargetPath = "/etc/containers/certs.d"
	DefaultIdentityName = "machine"
	DefaultMachineName  = "podman-machine-default"
	// TimeSyncVsockPort is the vsock port for host->guest time sync via qemu-guest-agent.
	// Podman passes this to vfkit/krunkit --timesync; podman-machine-os configures qemu-guest-agent on the same port.
	TimeSyncVsockPort = 1234
)

// MountTag is an identifier to mount a VirtioFS file system tag on a mount point in the VM.
// Ref: https://developer.apple.com/documentation/virtualization/running_intel_binaries_in_linux_vms_with_rosetta
const MountTag = "rosetta"

var DefaultFilePerm os.FileMode = 0o644

type CreateVMOpts struct {
	Name               string
	Dirs               *MachineDirs
	ReExec             bool
	UserModeNetworking bool
}

type MachineDirs struct {
	ConfigDir     *VMFile
	DataDir       *VMFile
	ImageCacheDir *VMFile
	RuntimeDir    *VMFile
}
