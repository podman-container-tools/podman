package images

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/inspect"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
	inspectTypes "go.podman.io/podman/v6/pkg/inspect"
)

var (
	// Command: podman image _inspect_
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] IMAGE [IMAGE...]",
		Short:             "Display the configuration of an image",
		Long:              `Displays the low-level information of an image identified by name or ID.`,
		RunE:              inspectExec,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman image inspect alpine
podman image inspect --format "imageId: {{.Id}} size: {{.Size}}" alpine
podman image inspect --format "image: {{.ImageName}} driver: {{.Driver}}" myctr`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
		Parent:  imageCmd,
	})
	inspectOpts = new(entities.InspectOptions)
	flags := inspectCmd.Flags()

	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "json", "Format the output to a Go template or json")
	_ = inspectCmd.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&inspectTypes.ImageData{}))
}

func inspectExec(_ *cobra.Command, args []string) error {
	inspectOpts.Type = common.ImageType
	return inspect.Inspect(args, *inspectOpts)
}
