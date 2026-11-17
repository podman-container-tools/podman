//go:build linux && amd64

package qemu

var QemuCommand = "qemu-system-x86_64"

func (q *QEMUStubber) addArchOptions(_ *setNewMachineCMDOpts) []string {
	opts := []string{
		"-accel", "kvm",
		"-cpu", "host",
		"-M", "memory-backend=mem",
	}
	return opts
}
