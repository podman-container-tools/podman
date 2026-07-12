package manifest

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var existsCmd = &cobra.Command{
	Use:               "exists MANIFEST",
	Short:             "Check if a manifest list exists in local storage",
	Long:              `If the manifest list exists in local storage, podman manifest exists exits with 0, otherwise the exit code will be 1.`,
	Args:              cobra.ExactArgs(1),
	RunE:              exists,
	ValidArgsFunction: common.AutocompleteImages,
	Example:           "podman manifest exists mylist",
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: existsCmd,
		Parent:  manifestCmd,
	})
}

func exists(_ *cobra.Command, args []string) error {
	found, err := registry.ImageEngine().ManifestExists(registry.Context(), args[0])
	if err != nil {
		return err
	}
	if !found.Value {
		registry.SetExitCode(1)
	}
	return nil
}
