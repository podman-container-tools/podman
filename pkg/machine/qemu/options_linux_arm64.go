//go:build linux && arm64

package qemu

import (
	"path/filepath"

	"go.podman.io/storage/pkg/fileutils"
)

var QemuCommand = "qemu-system-aarch64"

func (q *QEMUStubber) addArchOptions(_ *setNewMachineCMDOpts) []string {
	opts := []string{
		"-accel", "kvm",
		"-cpu", "host",
		"-M", "virt,gic-version=max,memory-backend=mem",
		"-bios", getQemuUefiFile("QEMU_EFI.fd"),
	}
	return opts
}

func getQemuUefiFile(name string) string {
	dirs := []string{
		"/usr/share/qemu-efi-aarch64",
		"/usr/share/edk2/aarch64",
	}
	for _, dir := range dirs {
		if err := fileutils.Exists(dir); err == nil {
			return filepath.Join(dir, name)
		}
	}
	return name
}
