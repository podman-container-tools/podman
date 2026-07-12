package farm

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

// Command: podman _farm_
var farmCmd = &cobra.Command{
	Use:   "farm",
	Short: "Farm out builds to remote machines",
	Long:  "Farm out builds to remote machines that podman can connect to via podman system connection",
	RunE:  validate.SubCommandExists,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: farmCmd,
	})
}
