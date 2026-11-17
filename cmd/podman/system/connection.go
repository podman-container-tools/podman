package system

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

// ConnectionCmd skips creating engines (PersistentPreRunE/PersistentPostRunE are No-Op's) since
// sub-commands will obtain connection information to said engines
var ConnectionCmd = &cobra.Command{
	Use:                "connection",
	Short:              "Manage remote API service destinations",
	Long:               `Manage remote API service destination information in podman configuration`,
	PersistentPreRunE:  validate.NoOp,
	RunE:               validate.SubCommandExists,
	PersistentPostRunE: validate.NoOp,
	TraverseChildren:   false,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: ConnectionCmd,
		Parent:  systemCmd,
	})
}
