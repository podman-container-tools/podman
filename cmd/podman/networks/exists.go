package network

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var (
	networkExistsDescription = `If the named network exists, podman network exists exits with 0, otherwise the exit code will be 1.`
	networkExistsCommand     = &cobra.Command{
		Use:               "exists NETWORK",
		Short:             "Check if network exists",
		Long:              networkExistsDescription,
		RunE:              networkExists,
		Example:           `podman network exists net1`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteNetworks,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkExistsCommand,
		Parent:  networkCmd,
	})
}

func networkExists(_ *cobra.Command, args []string) error {
	response, err := registry.ContainerEngine().NetworkExists(registry.Context(), args[0])
	if err != nil {
		return err
	}
	if !response.Value {
		registry.SetExitCode(1)
	}
	return nil
}
