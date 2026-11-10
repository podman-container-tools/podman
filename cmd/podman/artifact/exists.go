package artifact

import (
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var existsCmd = &cobra.Command{
	Use:               "exists ARTIFACT",
	Short:             "Check if an OCI artifact exists in local storage",
	Long:              `If the named artifact exists in local storage, podman artifact exists exits with 0, otherwise the exit code will be 1.`,
	Args:              cobra.ExactArgs(1),
	RunE:              exists,
	ValidArgsFunction: common.AutocompleteArtifacts,
	Example: `podman artifact exists quay.io/myimage/myartifact:latest
podman artifact exists c4dfb1609ee2
podman artifact exists localhost/myartifact || podman artifact add localhost/myartifact /tmp/file.txt`,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: existsCmd,
		Parent:  artifactCmd,
	})
}

func exists(cmd *cobra.Command, args []string) error {
	found, err := registry.ImageEngine().ArtifactExists(cmd.Context(), args[0])
	if err != nil {
		return err
	}
	if !found.Value {
		registry.SetExitCode(1)
	}
	return nil
}
