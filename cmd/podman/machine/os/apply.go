//go:build amd64 || arm64

package os

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/machine"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/machine/os"
)

var applyCmd = &cobra.Command{
	Use:               "apply [options] URI|IMAGE [MACHINE]",
	Short:             "Apply an OCI image to a Podman Machine's OS",
	Long:              "Apply custom layers from a containerized Fedora CoreOS OCI image on top of an existing VM",
	PersistentPreRunE: validate.NoOp,
	Args:              cobra.RangeArgs(1, 2),
	RunE:              apply,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			images, _ := common.AutocompleteImages(cmd, args, toComplete)
			// We also accept an URI so ignore ShellCompDirectiveNoFileComp and use the default one instead to get file paths completed by the shell.
			return images, cobra.ShellCompDirectiveDefault
		case 1:
			return machine.AutocompleteMachine(cmd, args, toComplete)
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	},
	Example: `podman machine os apply myimage`,
}

var restart bool

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: applyCmd,
		Parent:  machine.OSCmd,
	})
	flags := applyCmd.Flags()

	restartFlagName := "restart"
	flags.BoolVar(&restart, restartFlagName, false, "Restart VM to apply changes")
}

func apply(_ *cobra.Command, args []string) error {
	vmName := ""
	if len(args) == 2 {
		vmName = args[1]
	}
	managerOpts := ManagerOpts{
		VMName:  vmName,
		CLIArgs: args,
		Restart: restart,
	}

	osManager, err := NewOSManager(managerOpts)
	if err != nil {
		return err
	}

	applyOpts := os.ApplyOptions{
		Image: args[0],
	}
	return osManager.Apply(args[0], applyOpts)
}
