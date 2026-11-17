package connection

import (
	"slices"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/system"
)

var (
	// Skip creating engines since this command will obtain connection information to said engines.
	rmCmd = &cobra.Command{
		Use:     "remove [options] NAME",
		Aliases: []string{"rm"},
		Long:    `Delete named destination from podman configuration`,
		Short:   "Delete named destination",
		Args: func(cmd *cobra.Command, args []string) error {
			if rmOpts.All {
				return nil
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		ValidArgsFunction: common.AutocompleteSystemConnections,
		RunE:              rm,
		Example: `podman system connection remove devl
podman system connection rm devl`,
	}

	rmOpts = struct {
		All bool
	}{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  system.ContextCmd,
	})

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  system.ConnectionCmd,
	})

	flags := rmCmd.Flags()
	flags.BoolVarP(&rmOpts.All, "all", "a", false, "Remove all connections")

	flags.BoolP("force", "f", false, "Ignored: for Docker compatibility")
	_ = flags.MarkHidden("force")
}

func rm(_ *cobra.Command, args []string) error {
	return config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
		if rmOpts.All {
			cfg.Connection.Connections = nil
			cfg.Connection.Default = ""

			// Clear all the connections in any existing farms
			for k := range cfg.Farm.List {
				cfg.Farm.List[k] = []string{}
			}
			return nil
		}

		delete(cfg.Connection.Connections, args[0])
		if cfg.Connection.Default == args[0] {
			cfg.Connection.Default = ""
		}

		// If there are existing farm, remove the deleted connection that might be part of a farm
		for k, v := range cfg.Farm.List {
			index := slices.Index(v, args[0])
			if index > -1 {
				cfg.Farm.List[k] = append(v[:index], v[index+1:]...)
			}
		}

		return nil
	})
}
