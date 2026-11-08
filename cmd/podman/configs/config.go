package configs

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

// Command: podman _config_
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configs",
	Long:  "Manage configs",
	RunE:  validate.SubCommandExists,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: configCmd,
	})
}
