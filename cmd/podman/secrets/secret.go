package secrets

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

// Command: podman _secret_
var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Manage secrets",
	Long:  "Manage secrets",
	RunE:  validate.SubCommandExists,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: secretCmd,
	})
}
