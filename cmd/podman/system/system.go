package system

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()

	// Command: podman _system_
	systemCmd = &cobra.Command{
		Use:   "system",
		Short: "Manage podman",
		Long:  "Manage podman",
		RunE:  validate.SubCommandExists,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: systemCmd,
	})
}
