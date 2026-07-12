package quadlet

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	quadletRmDescription = `Remove one or more installed Quadlets from the current user`

	quadletRmCmd = &cobra.Command{
		Use:               "rm [options] [QUADLET|APPLICATION...]",
		Short:             "Remove Quadlets",
		Long:              quadletRmDescription,
		RunE:              rm,
		ValidArgsFunction: common.AutocompleteQuadlets,
		Example: `podman quadlet rm test.container
podman quadlet rm --force mysql.container
podman quadlet rm --all --reload-systemd=false
podman quadlet rm --recursive app`,
	}

	removeOptions entities.QuadletRemoveOptions
)

func rmFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&removeOptions.Force, "force", "f", false, "Remove running quadlets")
	flags.BoolVarP(&removeOptions.All, "all", "a", false, "Remove all Quadlets for the current user")
	flags.BoolVarP(&removeOptions.Ignore, "ignore", "i", false, "Do not error for Quadlets that do not exist")
	flags.BoolVar(&removeOptions.ReloadSystemd, "reload-systemd", true, "Reload systemd after removal")
	flags.BoolVar(&removeOptions.Recursive, "recursive", false, "Remove all Quadlets belonging to the specified application and its directory")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: quadletRmCmd,
		Parent:  quadletCmd,
	})
	rmFlags(quadletRmCmd)
}

func rm(_ *cobra.Command, args []string) error {
	if len(args) < 1 && !removeOptions.All {
		return errors.New("at least one quadlet file must be selected")
	}
	var errs utils.OutputErrors
	removeReport, err := registry.ContainerEngine().QuadletRemove(registry.Context(), args, removeOptions)
	if err != nil {
		errs = append(errs, fmt.Errorf("unable to remove Quadlet: %v", err))
	}
	// We can get a report back even if err != nil if systemd reload failed
	if removeReport != nil {
		for _, rq := range removeReport.Removed {
			fmt.Println(rq)
		}
		for quadlet, quadletErr := range removeReport.Errors {
			errs = append(errs, fmt.Errorf("unable to remove Quadlet %s: %v", quadlet, quadletErr))
		}
		if err == nil && len(removeReport.Errors) > 0 {
			errs = append(errs, errors.New("some quadlets could not be removed"))
		}
	}
	return errs.PrintErrors()
}
