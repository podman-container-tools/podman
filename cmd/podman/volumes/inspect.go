package volumes

import (
	"errors"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/inspect"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	volumeInspectDescription = `Display detailed information on one or more volumes.

  Use a Go template to change the format from JSON.`
	inspectCommand = &cobra.Command{
		Use:               "inspect [options] VOLUME [VOLUME...]",
		Short:             "Display detailed information on one or more volumes",
		Long:              volumeInspectDescription,
		RunE:              volumeInspect,
		ValidArgsFunction: common.AutocompleteVolumes,
		Example: `podman volume inspect myvol
podman volume inspect --all
podman volume inspect --format "{{.Driver}} {{.Scope}}" myvol`,
	}
)

var inspectOpts *entities.InspectOptions

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCommand,
		Parent:  volumeCmd,
	})
	inspectOpts = new(entities.InspectOptions)
	flags := inspectCommand.Flags()
	flags.BoolVarP(&inspectOpts.All, "all", "a", false, "Inspect all volumes")

	formatFlagName := "format"
	flags.StringVarP(&inspectOpts.Format, formatFlagName, "f", "json", "Format volume output using Go template")
	_ = inspectCommand.RegisterFlagCompletionFunc(formatFlagName, common.AutocompleteFormat(&define.InspectVolumeData{}))
}

func volumeInspect(_ *cobra.Command, args []string) error {
	if (inspectOpts.All && len(args) > 0) || (!inspectOpts.All && len(args) < 1) {
		return errors.New("provide one or more volume names or use --all")
	}
	inspectOpts.Type = common.VolumeType
	return inspect.Inspect(args, *inspectOpts)
}
