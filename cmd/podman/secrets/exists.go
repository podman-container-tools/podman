package secrets

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var existsCmd = &cobra.Command{
	Use:               "exists SECRET",
	Short:             "Check if a secret exists in local storage",
	Long:              `If the named secret exists in local storage, podman secret exists exits with 0, otherwise the exit code will be 1.`,
	Args:              cobra.ExactArgs(1),
	RunE:              exists,
	ValidArgsFunction: common.AutocompleteSecrets,
	Example: `podman secret exists ID
podman secret exists SECRET || podman secret create SECRET <secret source>`,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: existsCmd,
		Parent:  secretCmd,
	})
}

func exists(_ *cobra.Command, args []string) error {
	found, err := registry.ContainerEngine().SecretExists(registry.Context(), args[0])
	if err != nil {
		return err
	}
	if !found.Value {
		registry.SetExitCode(1)
	}
	return nil
}
