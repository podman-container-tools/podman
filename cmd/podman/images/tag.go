package images

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	tagDescription = "Adds one or more additional names to locally-stored image."
	tagCommand     = &cobra.Command{
		Use:               "tag IMAGE TARGET_NAME [TARGET_NAME...]",
		Short:             "Add an additional name to a local image",
		Long:              tagDescription,
		RunE:              tag,
		Args:              cobra.MinimumNArgs(2),
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman tag 0e3bbc2 fedora:latest
podman tag imageID:latest myNewImage:newTag
podman tag httpd myregistryhost:5000/fedora/httpd:v2`,
	}

	imageTagCommand = &cobra.Command{
		Args:              tagCommand.Args,
		Use:               tagCommand.Use,
		Short:             tagCommand.Short,
		Long:              tagCommand.Long,
		RunE:              tagCommand.RunE,
		ValidArgsFunction: tagCommand.ValidArgsFunction,
		Example: `podman image tag 0e3bbc2 fedora:latest
podman image tag imageID:latest myNewImage:newTag
podman image tag httpd myregistryhost:5000/fedora/httpd:v2`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: tagCommand,
	})
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageTagCommand,
		Parent:  imageCmd,
	})
}

func tag(_ *cobra.Command, args []string) error {
	return registry.ImageEngine().Tag(registry.Context(), args[0], args[1:], entities.ImageTagOptions{})
}
