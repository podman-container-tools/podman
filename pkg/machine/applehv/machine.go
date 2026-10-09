//go:build darwin

package applehv

import (
	"context"

	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func (a *AppleHVStubber) Remove(context.Context, *vmconfigs.MachineConfig) ([]string, func(context.Context) error, error) {
	return []string{}, func(context.Context) error { return nil }, nil
}

func (a *AppleHVStubber) State(_ context.Context, mc *vmconfigs.MachineConfig, _ bool) (define.Status, error) {
	vmStatus, err := mc.AppleHypervisor.Vfkit.State()
	if err != nil {
		return "", err
	}
	return vmStatus, nil
}

func (a *AppleHVStubber) StopVM(_ context.Context, mc *vmconfigs.MachineConfig, _ bool) error {
	return mc.AppleHypervisor.Vfkit.Stop(false, true)
}
