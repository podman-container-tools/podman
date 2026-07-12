package images

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/errorhandling"
)

var (
	rmDescription = "Removes one or more previously pulled or locally created images."
	rmCmd         = &cobra.Command{
		Use:               "rm [options] IMAGE [IMAGE...]",
		Short:             "Remove one or more images from local storage",
		Long:              rmDescription,
		RunE:              rm,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman image rm imageID
podman image rm --force alpine
podman image rm c4dfb1609ee2 93fd78260bd1 c0ed59d05ff7`,
	}

	rmiCmd = &cobra.Command{
		Use:               "rmi [options] IMAGE [IMAGE...]",
		Args:              rmCmd.Args,
		Short:             rmCmd.Short,
		Long:              rmCmd.Long,
		RunE:              rmCmd.RunE,
		ValidArgsFunction: rmCmd.ValidArgsFunction,
		Example: `podman rmi imageID
podman rmi --force alpine
podman rmi c4dfb1609ee2 93fd78260bd1 c0ed59d05ff7`,
	}

	imageOpts = entities.ImageRemoveOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCmd,
		Parent:  imageCmd,
	})

	imageRemoveFlagSet(rmCmd.Flags())

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmiCmd,
	})
	imageRemoveFlagSet(rmiCmd.Flags())
}

func imageRemoveFlagSet(flags *pflag.FlagSet) {
	flags.BoolVarP(&imageOpts.All, "all", "a", false, "Remove all images")
	flags.BoolVarP(&imageOpts.Ignore, "ignore", "i", false, "Ignore errors if a specified image does not exist")
	flags.BoolVarP(&imageOpts.Force, "force", "f", false, "Force Removal of the image")
	flags.BoolVar(&imageOpts.NoPrune, "no-prune", false, "Do not remove dangling images")
}

func rm(_ *cobra.Command, args []string) error {
	if len(args) < 1 && !imageOpts.All {
		return errors.New("image name or ID must be specified")
	}
	if len(args) > 0 && imageOpts.All {
		return errors.New("when using the --all switch, you may not pass any images names or IDs")
	}

	if imageOpts.Force {
		imageOpts.Ignore = true
	}

	// Note: certain image-removal errors are non-fatal.  Hence, the report
	// might be set even if err != nil.
	report, rmErrors := registry.ImageEngine().Remove(registry.Context(), args, imageOpts)
	if report != nil {
		for _, u := range report.Untagged {
			fmt.Println("Untagged: " + u)
		}
		for _, d := range report.Deleted {
			// Make sure an image was deleted (and not just untagged); else print it
			if len(d) > 0 {
				fmt.Println("Deleted: " + d)
			}
		}
	}
	if len(rmErrors) > 0 && report != nil {
		registry.SetExitCode(report.ExitCode)
	}

	return errorhandling.JoinErrors(rmErrors)
}
