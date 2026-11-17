package images

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()

	// Command: podman _image_
	imageCmd = &cobra.Command{
		Use:   "image",
		Short: "Manage images",
		Long:  "Manage images",
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: imageCmd,
	})
}
