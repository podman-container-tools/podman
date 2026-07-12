package containers

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()

	// Command: podman _container_
	containerCmd = &cobra.Command{
		Use:              "container",
		Short:            "Manage containers",
		Long:             "Manage containers",
		TraverseChildren: true,
		RunE:             validate.SubCommandExists,
	}

	containerConfig = registry.PodmanConfig().ContainersConfDefaultsRO
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerCmd,
	})
}
