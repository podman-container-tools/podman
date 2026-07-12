//go:build windows && amd64

package qemu

var QemuCommand = "qemu-system-x86_64w"

func (q *QEMUStubber) addArchOptions(_ *setNewMachineCMDOpts) []string {
	// "qemu64" level is used, because "host" is not supported with "whpx" acceleration.
	// It is a stable choice for running on bare metal and inside Hyper-V machine with nested virtualization.
	opts := []string{"-machine", "q35,accel=whpx:tcg", "-cpu", "qemu64"}
	return opts
}
