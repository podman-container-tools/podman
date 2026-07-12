package main

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/inspect"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	inspectDescription = `Displays the low-level information on an object identified by name or ID.
  For more inspection options, see:

      podman artifact inspect
      podman container inspect
      podman image inspect
      podman network inspect
      podman pod inspect
      podman volume inspect`

	// Command: podman _inspect_ Object_ID
	inspectCmd = &cobra.Command{
		Use:               "inspect [options] {ARTIFACT|CONTAINER|IMAGE|POD|NETWORK|VOLUME} [...]",
		Short:             "Display the configuration of object denoted by ID",
		RunE:              inspectExec,
		Long:              inspectDescription,
		TraverseChildren:  true,
		ValidArgsFunction: common.AutocompleteInspect,
		Example: `podman inspect fedora
podman inspect --type image fedora
podman inspect --type artifact quay.io/myimage/myartifact:latest
podman inspect CtrID ImgID
podman inspect --format "imageId: {{.Id}} size: {{.Size}}" fedora`,
	}
	inspectOpts *entities.InspectOptions
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: inspectCmd,
	})
	inspectOpts = inspect.AddInspectFlagSet(inspectCmd)
}

func inspectExec(_ *cobra.Command, args []string) error {
	return inspect.Inspect(args, *inspectOpts)
}
