//go:build amd64 || arm64

package ignition

import (
	"errors"
	"fmt"

	"go.podman.io/podman/v6/pkg/machine/define"
)

// ReadyUnitOpts are options for creating the ready unit that reports back to podman
// when the system is booted
type ReadyUnitOpts struct {
	Port uint64
}

// CreateReadyUnitFile makes the ready unit to report back to the host that the system is running
func CreateReadyUnitFile(provider define.VMType, opts *ReadyUnitOpts) (string, error) {
	readyUnit := DefaultReadyUnitFile()
	switch provider {
	case define.QemuVirt:
		readyUnit.Add("Unit", "Requires", "dev-virtio\\x2dports-vport1p1.device")
		readyUnit.Add("Unit", "After", "systemd-user-sessions.service")
		readyUnit.Add("Service", "ExecStart", "/bin/sh -c '/usr/bin/echo Ready >/dev/vport1p1'")
	case define.AppleHvVirt, define.LibKrun:
		readyUnit.Add("Unit", "Requires", "dev-virtio\\x2dports-vsock.device")
		readyUnit.Add("Service", "ExecStart", "/bin/sh -c '/usr/bin/echo Ready | socat - VSOCK-CONNECT:2:1025'")
	case define.HyperVVirt:
		if opts == nil || opts.Port == 0 {
			return "", errors.New("no port provided for hyperv ready unit")
		}
		readyUnit.Add("Unit", "Requires", "sys-devices-virtual-net-vsock0.device")
		readyUnit.Add("Unit", "After", "systemd-user-sessions.service")
		readyUnit.Add("Unit", "After", "vsock-network.service")
		readyUnit.Add("Service", "ExecStart", fmt.Sprintf("/bin/sh -c '/usr/bin/echo Ready | socat - VSOCK-CONNECT:2:%d'", opts.Port))
	case define.WSLVirt: // WSL does not use ignition
		return "", nil
	default:
		return "", fmt.Errorf("unable to generate ready unit for provider %q", provider.String())
	}
	return readyUnit.ToString()
}
