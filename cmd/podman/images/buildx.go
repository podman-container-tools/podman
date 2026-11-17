package images

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

// Command: podman _buildx_
// This is a hidden command, which was added to make converting
// from Docker to Podman easier.
// For now podman buildx build just calls into podman build
// If we are adding new buildx features, we will add them by default
// to podman build.
var buildxCmd = &cobra.Command{
	Use:     "buildx",
	Aliases: []string{"builder"},
	Short:   "Build images",
	Long:    "Build images",
	RunE:    validate.SubCommandExists,
	Hidden:  true,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: buildxCmd,
	})
}
