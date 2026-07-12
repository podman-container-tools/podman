package artifact

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

// Command: podman _artifact_
var artifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Manage OCI artifacts",
	Long:  "Manage OCI artifacts",
	RunE:  validate.SubCommandExists,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: artifactCmd,
	})
}
