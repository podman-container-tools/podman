package volumes

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var (
	volumeExistsDescription = `If the given volume exists, podman volume exists exits with 0, otherwise the exit code will be 1.`
	volumeExistsCommand     = &cobra.Command{
		Use:               "exists VOLUME",
		Short:             "Check if volume exists",
		Long:              volumeExistsDescription,
		RunE:              volumeExists,
		Example:           `podman volume exists myvol`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteVolumes,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: volumeExistsCommand,
		Parent:  volumeCmd,
	})
}

func volumeExists(_ *cobra.Command, args []string) error {
	response, err := registry.ContainerEngine().VolumeExists(registry.Context(), args[0])
	if err != nil {
		return err
	}
	if !response.Value {
		registry.SetExitCode(1)
	}
	return nil
}
