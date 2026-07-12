package containers

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/inspect"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	// podman container _inspect_
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] CONTAINER [CONTAINER...]",
		Short:             "Display the configuration of a container",
		Long:              `Displays the low-level information on a container identified by name or ID.`,
		RunE:              inspectExec,
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman container inspect myCtr
podman container inspect -l --format '{{.Id}} {{.Config.Labels}}'`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  containerCmd,
	})
	inspectOpts = new(entities.InspectOptions)
	flags := inspectCmd.Flags()
	flags.BoolVarP(&inspectOpts.Size, "size", "s", false, "Display total file size")

	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&define.InspectContainerData{}))

	validate.AddLatestFlag(inspectCmd, &inspectOpts.Latest)
}

func inspectExec(_ *cobra.Command, args []string) error {
	// Force container type
	inspectOpts.Type = common.ContainerType
	return inspect.Inspect(args, *inspectOpts)
}
