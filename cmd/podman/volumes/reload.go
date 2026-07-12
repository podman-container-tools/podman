package volumes

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	reloadDescription = `Check all configured volume plugins and update the libpod database with all available volumes.

  Existing volumes are also removed from the database when they are no longer present in the plugin.`
	reloadCommand = &cobra.Command{
		Use:               "reload",
		Args:              validate.NoArgs,
		Short:             "Reload all volumes from volume plugins",
		Long:              reloadDescription,
		RunE:              reload,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: reloadCommand,
		Parent:  volumeCmd,
	})
}

func reload(_ *cobra.Command, _ []string) error {
	report, err := registry.ContainerEngine().VolumeReload(registry.Context())
	if err != nil {
		return err
	}
	printReload("Added", report.Added)
	printReload("Removed", report.Removed)
	errs := (utils.OutputErrors)(report.Errors)
	return errs.PrintErrors()
}

func printReload(typ string, values []string) {
	if len(values) > 0 {
		fmt.Println(typ + ":")
		for _, name := range values {
			fmt.Println(name)
		}
	}
}
