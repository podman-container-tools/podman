//go:build freebsd && arm64

package qemu

var QemuCommand = "qemu-system-aarch64"

func (q *QEMUStubber) addArchOptions(_ *setNewMachineCMDOpts) []string {
	opts := []string{
		"-machine", "virt",
		"-accel", "tcg",
		"-cpu", "host",
	}
	return opts
}
