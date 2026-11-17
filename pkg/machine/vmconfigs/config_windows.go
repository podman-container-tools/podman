package vmconfigs

import (
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/hyperv/vsock"
	"go.podman.io/podman/v6/pkg/machine/qemu/command"
)

type HyperVConfig struct {
	// ReadyVSock is the pipeline for the guest to alert the host
	// it is running
	ReadyVsock vsock.HVSockRegistryEntry
	// NetworkVSock is for the user networking
	NetworkVSock vsock.HVSockRegistryEntry
	// FileserverVSocks are for machine mounts (one entry per mount)
	FileserverVSocks []vsock.HVSockRegistryEntry
}

type WSLConfig struct {
	// Uses usermode networking
	UserModeNetworking bool
}

type QEMUConfig struct {
	// QMPMonitor is the qemu monitor object for sending commands
	QMPMonitor command.Monitor
	// QEMUPidPath is where to write the PID for QEMU when running
	QEMUPidPath *define.VMFile
}

// Stubs
type (
	AppleHVConfig struct{}
	LibKrunConfig struct{}
)

func getHostUID() int {
	return 1000
}
