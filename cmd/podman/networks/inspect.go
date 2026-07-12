package network

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/inspect"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	networkinspectDescription = `Inspect network`
	networkinspectCommand     = &cobra.Command{
		Use:               "inspect [options] NETWORK [NETWORK...]",
		Long:              "Displays the network configuration for one or more networks.",
		Short:             networkinspectDescription,
		RunE:              networkInspect,
		Example:           `podman network inspect podman`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteNetworks,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkinspectCommand,
		Parent:  networkCmd,
	})
	inspectOpts = new(entities.InspectOptions)
	flags := networkinspectCommand.Flags()

	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "", "Pretty-print network to JSON or using a Go template")
	_ = networkinspectCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.NetworkInspectReport{}))
}

func networkInspect(_ *cobra.Command, args []string) error {
	inspectOpts.Type = common.NetworkType
	return inspect.Inspect(args, *inspectOpts)
}
