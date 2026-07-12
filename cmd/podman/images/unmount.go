package images

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	description = `Image storage increments a mount counter each time an image is mounted.

  When an image is unmounted, the mount counter is decremented. The image's root filesystem is physically unmounted only when the mount counter reaches zero indicating no other processes are using the mount.

  An unmount can be forced with the --force flag.
`
	unmountCommand = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "unmount [options] IMAGE [IMAGE...]",
		Aliases:           []string{"umount"},
		Short:             "Unmount an image's root filesystem",
		Long:              description,
		RunE:              unmount,
		ValidArgsFunction: common.AutocompleteImages,
		Example: `podman unmount imgID
podman unmount imgID1 imgID2 imgID3
podman unmount --all`,
	}
)

var unmountOpts entities.ImageUnmountOptions

func unmountFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&unmountOpts.All, "all", "a", false, "Unmount all of the currently mounted images")
	flags.BoolVarP(&unmountOpts.Force, "force", "f", false, "Force the complete unmount of the specified mounted images")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Parent:  imageCmd,
		Command: unmountCommand,
	})
	unmountFlags(unmountCommand.Flags())
}

func unmount(_ *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	if len(args) < 1 && !unmountOpts.All {
		return errors.New("image name or ID must be specified")
	}
	if len(args) > 0 && unmountOpts.All {
		return errors.New("when using the --all switch, you may not pass any image names or IDs")
	}
	reports, err := registry.ImageEngine().Unmount(registry.Context(), args, unmountOpts)
	if err != nil {
		return err
	}
	for _, r := range reports {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}
