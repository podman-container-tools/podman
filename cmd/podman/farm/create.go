package farm

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	farmCreateDescription = `Create a new farm with connections added via podman system connection add.

	The "podman system connection add --farm" command can be used to add a new connection to a new or existing farm.`

	createCommand = &cobra.Command{
		Use:                "create NAME [CONNECTIONS...]",
		Args:               cobra.MinimumNArgs(1),
		Short:              "Create a new farm",
		Long:               farmCreateDescription,
		PersistentPreRunE:  validate.NoOp,
		RunE:               create,
		PersistentPostRunE: validate.NoOp,
		ValidArgsFunction:  completion.AutocompleteNone,
		Example: `podman farm create myfarm connection1
podman farm create myfarm`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: createCommand,
		Parent:  farmCmd,
	})
}

func create(_ *cobra.Command, args []string) error {
	farmName := args[0]
	connections := args[1:]

	err := config.EditConnectionConfig(func(cfg *config.ConnectionsFile) error {
		if _, ok := cfg.Farm.List[farmName]; ok {
			// if farm exists return an error
			return fmt.Errorf("farm with name %q already exists", farmName)
		}

		// Can create an empty farm without any connections
		if len(connections) == 0 {
			cfg.Farm.List[farmName] = []string{}
		}

		for _, c := range connections {
			if _, ok := cfg.Connection.Connections[c]; ok {
				if slices.Contains(cfg.Farm.List[farmName], c) {
					// Don't add duplicate connections to a farm
					continue
				}
				cfg.Farm.List[farmName] = append(cfg.Farm.List[farmName], c)
			} else {
				return fmt.Errorf("cannot create farm, %q is not a system connection", c)
			}
		}

		// If this is the first farm being created, set it as the default farm
		if len(cfg.Farm.List) == 1 {
			cfg.Farm.Default = farmName
		}

		return nil
	})
	if err != nil {
		return err
	}
	fmt.Printf("Farm %q created\n", farmName)
	return nil
}
