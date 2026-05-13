//go:build amd64 || arm64

package machine

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/shim"
)

var restartCmd = &cobra.Command{
	Use:               "restart [MACHINE]",
	Short:             "Restart an existing machine",
	Long:              "Restart a managed virtual machine",
	PersistentPreRunE: machinePreRunE,
	RunE:              restart,
	Args:              cobra.MaximumNArgs(1),
	Example:           `podman machine restart podman-machine-default`,
	ValidArgsFunction: AutocompleteMachine,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: restartCmd,
		Parent:  machineCmd,
	})
}

func restart(_ *cobra.Command, args []string) error {
	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	mc, vmProvider, err := shim.VMExists(vmName)
	if err != nil {
		return err
	}

	if err := shim.Stop(mc, vmProvider, false); err != nil {
		return err
	}

	newMachineEvent(events.Stop, events.Event{Name: vmName})

	updateConnection := false
	if err := shim.Start(mc, vmProvider, machine.StartOptions{}, &updateConnection); err != nil {
		return err
	}
	fmt.Printf("Machine %q restarted successfully\n", vmName)
	newMachineEvent(events.Start, events.Event{Name: vmName})
	return nil
}
