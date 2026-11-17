package connection

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/system"
)

var (
	// Skip creating engines since this command will obtain connection information to said engines.
	dfltCmd = &cobra.Command{
		Use:               "default NAME",
		Args:              cobra.ExactArgs(1),
		Short:             "Set named destination as default",
		Long:              `Set named destination as default for the Podman service`,
		ValidArgsFunction: common.AutocompleteSystemConnections,
		RunE:              defaultRunE,
		Example:           `podman system connection default testing`,
	}

	useCmd = &cobra.Command{
		Use:               "use NAME",
		Args:              cobra.ExactArgs(1),
		Short:             dfltCmd.Short,
		Long:              dfltCmd.Long,
		ValidArgsFunction: dfltCmd.ValidArgsFunction,
		RunE:              dfltCmd.RunE,
		Example:           `podman context use testing`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: useCmd,
		Parent:  system.ContextCmd,
	})
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: dfltCmd,
		Parent:  system.ConnectionCmd,
	})
}

func defaultRunE(_ *cobra.Command, args []string) error {
	connection := args[0]
	return config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
		if _, found := cfg.Connection.Connections[connection]; !found {
			return fmt.Errorf("%q destination is not defined. See \"podman system connection add ...\" to create a connection", connection)
		}

		cfg.Connection.Default = connection
		return nil
	})
}
