//go:build amd64 || arm64

package machine

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/machine/shim"
)

var stopCmd = &cobra.Command{
	Use:               "stop [MACHINE]",
	Short:             "Stop an existing machine",
	Long:              "Stop a managed virtual machine ",
	PersistentPreRunE: machinePreRunE,
	RunE:              stop,
	Args:              cobra.MaximumNArgs(1),
	Example:           `podman machine stop podman-machine-default`,
	ValidArgsFunction: AutocompleteMachine,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: stopCmd,
		Parent:  machineCmd,
	})
}

// TODO  Name shouldn't be required, need to create a default vm
func stop(_ *cobra.Command, args []string) error {
	var err error

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

	fmt.Printf("Machine %q stopped successfully\n", vmName)
	newMachineEvent(events.Stop, events.Event{Name: vmName})
	return nil
}
