package network

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	networkDisconnectDescription = `Remove container from a network`
	networkDisconnectCommand     = &cobra.Command{
		Use:               "disconnect [options] NETWORK CONTAINER",
		Short:             "Disconnect a container from a network",
		Long:              networkDisconnectDescription,
		RunE:              networkDisconnect,
		Example:           `podman network disconnect web secondary`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: common.AutocompleteNetworkConnectCmd,
	}
)

var networkDisconnectOptions entities.NetworkDisconnectOptions

func networkDisconnectFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&networkDisconnectOptions.Force, "force", "f", false, "force removal of container from network")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkDisconnectCommand,
		Parent:  networkCmd,
	})
	flags := networkDisconnectCommand.Flags()
	networkDisconnectFlags(flags)
}

func networkDisconnect(_ *cobra.Command, args []string) error {
	networkDisconnectOptions.Container = args[1]
	return registry.ContainerEngine().NetworkDisconnect(registry.Context(), args[0], networkDisconnectOptions)
}
