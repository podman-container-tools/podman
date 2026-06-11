package containers

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	renameDescription = "The podman rename command allows you to rename an existing container"
	renameCommand     = &cobra.Command{
		Use:               "rename CONTAINER NAME",
		Short:             "Rename an existing container",
		Long:              renameDescription,
		RunE:              rename,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: common.AutocompleteContainerOneArg,
		Example:           "podman rename containerA newName",
	}

	containerRenameCommand = &cobra.Command{
		Use:               renameCommand.Use,
		Short:             renameCommand.Short,
		Long:              renameCommand.Long,
		RunE:              rename,
		Args:              renameCommand.Args,
		ValidArgsFunction: common.AutocompleteContainerOneArg,
		Example:           "podman container rename containerA newName",
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: renameCommand,
	})

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerRenameCommand,
		Parent:  containerCmd,
	})
}

func rename(_ *cobra.Command, args []string) error {
	args = utils.RemoveSlash(args)
	return renameContainer(args)
}

func renameContainer(args []string) error {
	renameOpts := entities.ContainerRenameOptions{
		NewName: args[1],
	}
	return registry.ContainerEngine().ContainerRename(registry.Context(), args[0], renameOpts)
}
