//go:build amd64 || arm64

package machine

import (
	"errors"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/shim"
)

var rmCmd = &cobra.Command{
	Use:               "rm [options] [MACHINE]",
	Short:             "Remove an existing machine",
	Long:              "Remove a managed virtual machine ",
	PersistentPreRunE: machinePreRunE,
	RunE:              rm,
	Args:              cobra.MaximumNArgs(1),
	Example:           `podman machine rm podman-machine-default`,
	ValidArgsFunction: AutocompleteMachine,
}

var destroyOptions machine.RemoveOptions

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  machineCmd,
	})

	flags := rmCmd.Flags()
	formatFlagName := "force"
	flags.BoolVarP(&destroyOptions.Force, formatFlagName, "f", false, "Stop and do not prompt before rming")

	ignitionFlagName := "save-ignition"
	flags.BoolVar(&destroyOptions.SaveIgnition, ignitionFlagName, false, "Do not delete ignition file")

	imageFlagName := "save-image"
	flags.BoolVar(&destroyOptions.SaveImage, imageFlagName, false, "Do not delete the image file")

	flags.BoolVar(
		&destroyOptions.ReExec,
		"reexec", false,
		"process was rexeced",
	)
	_ = flags.MarkHidden("reexec")
}

func rm(_ *cobra.Command, args []string) error {
	var err error
	vmName := defaultMachineName
	if len(args) > 0 && len(args[0]) > 0 {
		vmName = args[0]
	}

	mc, vmProvider, err := shim.VMExists(vmName)
	if err != nil {
		return err
	}

	if err := shim.Remove(mc, vmProvider, destroyOptions); err != nil {
		// ErrRelaunchSucceeded is not a real error: it signals that
		// an elevated child process completed the removal successfully.
		// Exit gracefully.
		if errors.Is(err, define.ErrRelaunchSucceeded) {
			return nil
		}
		return err
	}
	newMachineEvent(events.Remove, events.Event{Name: vmName})
	return nil
}
