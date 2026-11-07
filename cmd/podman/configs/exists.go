package configs

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var existsCmd = &cobra.Command{
	Use:               "exists CONFIG",
	Short:             "Check if a config exists in local storage",
	Long:              `If the named config exists in local storage, podman config exists exits with 0, otherwise the exit code will be 1.`,
	Args:              cobra.ExactArgs(1),
	RunE:              exists,
	ValidArgsFunction: common.AutocompleteConfigs,
	Example: `podman config exists ID
podman config exists CONFIG || podman config create CONFIG <config source>`,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: existsCmd,
		Parent:  configCmd,
	})
}

func exists(cmd *cobra.Command, args []string) error {
	found, err := registry.ContainerEngine().ConfigExists(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	if !found.Value {
		registry.SetExitCode(1)
	}
	return nil
}
