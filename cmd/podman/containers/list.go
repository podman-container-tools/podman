package containers

import (
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

// podman container _list_
var listCmd = &cobra.Command{
	Use:               "list [options]",
	Aliases:           []string{"ls"},
	Args:              validate.NoArgs,
	Short:             "List containers",
	Long:              "Prints out information about the containers",
	RunE:              ps,
	ValidArgsFunction: completion.AutocompleteNone,
	Example: `podman container list -a
podman container list -a --format "{{.ID}}  {{.Image}}  {{.Labels}}  {{.Mounts}}"
podman container list --size --sort names`,
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: listCmd,
		Parent:  containerCmd,
	})
	listFlagSet(listCmd)
	validate.AddLatestFlag(listCmd, &listOpts.Latest)
}
