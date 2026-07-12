package healthcheck

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var healthCmd = &cobra.Command{
	Use:   "healthcheck",
	Short: "Manage health checks on containers",
	Long:  "Run health checks on containers",
	RunE:  validate.SubCommandExists,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: healthCmd,
	})
}
