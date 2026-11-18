package quadlet

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var quadletExistsCmd = &cobra.Command{
	Use:               "exists QUADLET",
	Short:             "Check if a quadlet exists",
	Long:              `If a quadlet with the given name is present, podman quadlet exists exits with 0, otherwise the exit code will be 1.`,
	Args:              cobra.ExactArgs(1),
	RunE:              exists,
	ValidArgsFunction: common.AutocompleteQuadlets,
	Example: `podman quadlet exists myquadlet.container
podman quadlet exists myquadlet.container || podman quadlet install ./myquadlet.container`,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: quadletExistsCmd,
		Parent:  quadletCmd,
	})
}

func exists(cmd *cobra.Command, args []string) error {
	found, err := registry.ContainerEngine().QuadletExists(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	if !found.Value {
		registry.SetExitCode(1)
	}
	return nil
}
