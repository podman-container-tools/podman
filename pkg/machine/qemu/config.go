//go:build !darwin

package qemu

import (
	"go.podman.io/common/pkg/config"
)

// setNewMachineCMDOpts are options needed to pass
// into setting up the qemu command line.  long term, this need
// should be eliminated
// TODO Podman5
type setNewMachineCMDOpts struct{}

// findQEMUBinary locates and returns the QEMU binary
func findQEMUBinary() (string, error) {
	cfg, err := config.Default()
	if err != nil {
		return "", err
	}
	return cfg.FindHelperBinary(QemuCommand, true)
}
