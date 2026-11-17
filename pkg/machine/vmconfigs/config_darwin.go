package vmconfigs

import (
	"os"

	"go.podman.io/podman/v6/pkg/machine/apple/vfkit"
)

type AppleHVConfig struct {
	// The VFKit endpoint where we can interact with the VM
	Vfkit vfkit.Helper
}

type LibKrunConfig struct {
	KRun vfkit.Helper
}

// Stubs
type (
	HyperVConfig struct{}
	WSLConfig    struct{}
	QEMUConfig   struct{}
)

func getHostUID() int {
	return os.Getuid()
}
