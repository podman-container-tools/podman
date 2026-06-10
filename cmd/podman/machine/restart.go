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

var (
	restartCmd = &cobra.Command{
		Use:               "restart [options] [MACHINE]",
		Short:             "Restart an existing machine",
		Long:              "Restart a managed virtual machine",
		PersistentPreRunE: machinePreRunE,
		RunE:              restart,
		Args:              cobra.MaximumNArgs(1),
		Example:           `podman machine restart podman-machine-default`,
		ValidArgsFunction: AutocompleteMachine,
	}
	restartOpts = machine.StartOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: restartCmd,
		Parent:  machineCmd,
	})

	flags := restartCmd.Flags()
	noInfoFlagName := "no-info"
	flags.BoolVar(&restartOpts.NoInfo, noInfoFlagName, false, "Suppress informational tips")

	quietFlagName := "quiet"
	flags.BoolVarP(&restartOpts.Quiet, quietFlagName, "q", false, "Suppress machine restarting status output")
}

func restart(_ *cobra.Command, args []string) error {
	restartOpts.NoInfo = restartOpts.Quiet || restartOpts.NoInfo
	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	mc, vmProvider, err := shim.VMExists(vmName)
	if err != nil {
		return err
	}

	if !restartOpts.Quiet {
		fmt.Printf("Restarting machine %q\n", vmName)
	}

	updateConnection := false
	if err := shim.StopThenStart(mc, vmProvider, false, restartOpts, &updateConnection); err != nil {
		return err
	}
	fmt.Printf("Machine %q restarted successfully\n", vmName)

	newMachineEvent(events.Restart, events.Event{Name: vmName})
	return nil
}
