//go:build amd64 || arm64

package os

import (
	"context"
	"fmt"

	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/shim"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

// MachineOS manages machine OS's from outside the machine.
type MachineOS struct {
	Args     []string
	VM       *vmconfigs.MachineConfig
	Provider vmconfigs.VMProvider
	VMName   string
	Restart  bool
}

// Apply applies the image by sshing into the machine and running apply from inside the VM.
func (m *MachineOS) Apply(image string, _ ApplyOptions) error {
	var off bool
	args := []string{"podman", "machine", "os", "apply", image}

	if err := machine.LocalhostSSHShellForceTerm(m.VM.SSH.RemoteUsername, m.VM.SSH.IdentityPath, m.VMName, m.VM.SSH.Port, args); err != nil {
		return err
	}

	if m.Restart {
		if err := shim.Stop(m.VM, m.Provider, off); err != nil {
			return err
		}
		if err := shim.Start(m.VM, m.Provider, machine.StartOptions{NoInfo: true}, &off); err != nil {
			return err
		}
		fmt.Printf("Machine %q restarted successfully\n", m.VMName)
	}
	return nil
}

func (m *MachineOS) Upgrade(_ context.Context, opts UpgradeOptions) error {
	isDryRun := opts.DryRun
	if len(opts.Format) > 0 {
		isDryRun = true
	}
	args := []string{"podman", "machine", "os", "upgrade", "--host-version=" + opts.ClientVersion.String()}
	if opts.DryRun {
		args = append(args, "-n")
	}
	if opts.Format != "" {
		args = append(args, "-f", opts.Format)
	}
	if err := machine.LocalhostSSHShellForceTerm(m.VM.SSH.RemoteUsername, m.VM.SSH.IdentityPath, m.VMName, m.VM.SSH.Port, args); err != nil {
		return err
	}
	if m.Restart && !isDryRun {
		var off bool
		if err := shim.Stop(m.VM, m.Provider, off); err != nil {
			return err
		}
		if err := shim.Start(m.VM, m.Provider, machine.StartOptions{NoInfo: true}, &off); err != nil {
			return err
		}
		fmt.Printf("Machine %q restarted successfully\n", m.VMName)
	}
	return nil
}
