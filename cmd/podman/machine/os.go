//go:build amd64 || arm64

package machine

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var OSCmd = &cobra.Command{
	Use:               "os",
	Short:             "Manage a Podman virtual machine's OS",
	Long:              "Manage a Podman virtual machine's operating system",
	PersistentPreRunE: validate.NoOp,
	RunE:              validate.SubCommandExists,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: OSCmd,
		Parent:  machineCmd,
	})
}
