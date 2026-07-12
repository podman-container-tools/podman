package artifact

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	rmCmd = &cobra.Command{
		Use:               "rm [options] ARTIFACT [ARTIFACT...]",
		Short:             "Remove one or more OCI artifacts",
		Long:              "Remove one or more OCI artifacts from local storage",
		RunE:              rm,
		Aliases:           []string{"remove"},
		Args:              checkAllAndArgs,
		ValidArgsFunction: common.AutocompleteArtifacts,
		Example: `podman artifact rm quay.io/myimage/myartifact:latest
podman artifact rm -a
podman artifact rm c4dfb1609ee2 93fd78260bd1 c0ed59d05ff7
podman artifact rm -i c4dfb1609ee2`,
	}

	rmOptions = entities.ArtifactRemoveOptions{}
)

func rmFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all artifacts")
	flags.BoolVarP(&rmOptions.Ignore, "ignore", "i", false, "Ignore error if artifact does not exist")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  artifactCmd,
	})
	rmFlags(rmCmd)
}

func rm(_ *cobra.Command, args []string) error {
	rmOptions.Artifacts = args

	artifactRemoveReport, err := registry.ImageEngine().ArtifactRm(registry.Context(), rmOptions)
	if err != nil {
		return err
	}
	for _, d := range artifactRemoveReport.ArtifactDigests {
		fmt.Println(d.Encoded())
	}
	return nil
}

// checkAllAndArgs takes a cobra command and args and checks if
// all is used, then no args can be passed. note: this was created
// as an unexported local func for now and could be moved to pkg
// validate.  if we add "--latest" to the command, then perhaps
// one of the existing plg validate funcs would be appropriate.
func checkAllAndArgs(c *cobra.Command, args []string) error {
	all, _ := c.Flags().GetBool("all")
	if all && len(args) > 0 {
		return fmt.Errorf("when using the --all switch, you may not pass any artifact names or digests")
	}
	if !all {
		if len(args) < 1 {
			return errors.New("at least one artifact name or digest must be specified")
		}
	}
	return nil
}
