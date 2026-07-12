package pods

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/inspect"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	inspectDescription = `Display the configuration for a pod by name or id

	By default, this will render all results in a JSON array.`

	inspectCmd = &cobra.Command{
		Use:               "inspect [options] POD [POD...]",
		Short:             "Display a pod configuration",
		Long:              inspectDescription,
		RunE:              inspectExec,
		ValidArgsFunction: common.AutocompletePods,
		Example:           `podman pod inspect podID`,
	}

	inspectOpts = &entities.InspectOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  podCmd,
	})
	flags := inspectCmd.Flags()

	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&entities.PodInspectReport{}))

	validate.AddLatestFlag(inspectCmd, &inspectOpts.Latest)
}

func inspectExec(_ *cobra.Command, args []string) error {
	inspectOpts.Type = common.PodType
	return inspect.Inspect(args, *inspectOpts)
}
