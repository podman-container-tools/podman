package containers

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	description = `Container storage increments a mount counter each time a container is mounted.

  When a container is unmounted, the mount counter is decremented. The container's root filesystem is physically unmounted only when the mount counter reaches zero indicating no other processes are using the mount.

  An unmount can be forced with the --force flag.
`
	unmountCommand = &cobra.Command{
		Annotations: map[string]string{registry.EngineMode: registry.ABIMode},
		Use:         "unmount [options] CONTAINER [CONTAINER...]",
		Aliases:     []string{"umount"},
		Short:       "Unmount working container's root filesystem",
		Long:        description,
		RunE:        unmount,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "")
		},
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman unmount ctrID
podman unmount ctrID1 ctrID2 ctrID3
podman unmount --all`,
	}

	containerUnmountCommand = &cobra.Command{
		Annotations: unmountCommand.Annotations,
		Use:         unmountCommand.Use,
		Short:       unmountCommand.Short,
		Aliases:     unmountCommand.Aliases,
		Long:        unmountCommand.Long,
		RunE:        unmountCommand.RunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "")
		},
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman container unmount ctrID
podman container unmount ctrID1 ctrID2 ctrID3
podman container unmount --all`,
	}
)

var unmountOpts entities.ContainerUnmountOptions

func unmountFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&unmountOpts.All, "all", "a", false, "Unmount all of the currently mounted containers")
	flags.BoolVarP(&unmountOpts.Force, "force", "f", false, "Force the complete unmount of the specified mounted containers")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: unmountCommand,
	})
	unmountFlags(unmountCommand.Flags())
	validate.AddLatestFlag(unmountCommand, &unmountOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerUnmountCommand,
		Parent:  containerCmd,
	})
	unmountFlags(containerUnmountCommand.Flags())
	validate.AddLatestFlag(containerUnmountCommand, &unmountOpts.Latest)
}

func unmount(_ *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	args = utils.RemoveSlash(args)
	reports, err := registry.ContainerEngine().ContainerUnmount(registry.Context(), args, unmountOpts)
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
