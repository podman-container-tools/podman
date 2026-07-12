package manifest

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
)

var removeCmd = &cobra.Command{
	Use:               "remove LIST DIGEST",
	Short:             "Remove an item from a manifest list or image index",
	Long:              "Removes an item from a manifest list or image index.",
	RunE:              remove,
	Args:              cobra.ExactArgs(2),
	ValidArgsFunction: common.AutocompleteManifestListAndMember,
	Example:           `podman manifest remove mylist:v1.11 sha256:15352d97781ffdf357bf3459c037be3efac4133dc9070c2dce7eca7c05c3e736`,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: removeCmd,
		Parent:  manifestCmd,
	})
}

func remove(_ *cobra.Command, args []string) error {
	updatedListID, err := registry.ImageEngine().ManifestRemoveDigest(registry.Context(), args[0], args[1])
	if err != nil {
		return fmt.Errorf("removing from manifest list %s: %w", args[0], err)
	}
	fmt.Println(updatedListID)
	return nil
}
