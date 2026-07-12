//go:build linux || freebsd

package vmconfigs

import (
	"os"

	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/qemu/command"
)

type QEMUConfig struct {
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor command.Monitor
	// QEMUPidPath is where to write the PID for QEMU when running
	QEMUPidPath *define.VMFile
}

// Stubs
type (
	AppleHVConfig struct{}
	HyperVConfig  struct{}
	LibKrunConfig struct{}
	WSLConfig     struct{}
)

func getHostUID() int {
	return os.Getuid()
}
