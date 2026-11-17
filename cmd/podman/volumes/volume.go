package volumes

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()

	// Command: podman _volume_
	volumeCmd = &cobra.Command{
		Use:   "volume",
		Short: "Manage volumes",
		Long:  "Volumes are created in and can be shared between containers",
		RunE:  validate.SubCommandExists,
	}
	containerConfig = registry.PodmanConfig().ContainersConfDefaultsRO
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: volumeCmd,
	})
}
