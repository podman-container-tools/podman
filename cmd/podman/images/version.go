package images

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/buildah/define"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
)

var (
	versionDescription = `Print build version`
	versionCmd         = &cobra.Command{
		Use:               "version",
		Args:              validate.NoArgs,
		Short:             "Print build version",
		Long:              versionDescription,
		RunE:              version,
		ValidArgsFunction: completion.AutocompleteNone,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: versionCmd,
		Parent:  buildxCmd,
	})
}

func version(_ *cobra.Command, _ []string) error {
	fmt.Printf("%s %s\n", define.Package, define.Version)
	return nil
}
