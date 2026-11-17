package system

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

// ContextCmd skips creating engines (PersistentPreRunE/PersistentPostRunE are No-Op's) since
// sub-commands will obtain connection information to said engines
var ContextCmd = &cobra.Command{
	Use:                "context",
	Short:              "Manage remote API service destinations",
	Long:               `Manage remote API service destination information in podman configuration`,
	PersistentPreRunE:  validate.NoOp,
	RunE:               validate.SubCommandExists,
	PersistentPostRunE: validate.NoOp,
	Hidden:             true,
	TraverseChildren:   false,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: ContextCmd,
	})
}
