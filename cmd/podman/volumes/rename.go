package volumes

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	volumeRenameDescription = "Rename an existing volume. The volume must not be in use by any containers."
	volumeRenameCommand     = &cobra.Command{
		Use:               "rename VOLUME NEWNAME",
		Short:             "Rename a volume",
		Long:              volumeRenameDescription,
		RunE:              volumeRename,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: common.AutocompleteVolumes,
		Example:           "podman volume rename oldName newName",
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: volumeRenameCommand,
		Parent:  volumeCmd,
	})
}

func volumeRename(_ *cobra.Command, args []string) error {
	return registry.ContainerEngine().VolumeRename(registry.Context(), args[0], entities.VolumeRenameOptions{
		NewName: args[1],
	})
}
