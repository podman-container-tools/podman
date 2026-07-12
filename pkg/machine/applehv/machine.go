//go:build darwin

package applehv

import (
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func (a *AppleHVStubber) Remove(_ *vmconfigs.MachineConfig) ([]string, func() error, error) {
	return []string{}, func() error { return nil }, nil
}

func (a *AppleHVStubber) State(mc *vmconfigs.MachineConfig, _ bool) (define.Status, error) {
	vmStatus, err := mc.AppleHypervisor.Vfkit.State()
	if err != nil {
		return "", err
	}
	return vmStatus, nil
}

func (a *AppleHVStubber) StopVM(mc *vmconfigs.MachineConfig, _ bool) error {
	return mc.AppleHypervisor.Vfkit.Stop(false, true)
}
