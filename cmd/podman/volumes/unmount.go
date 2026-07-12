package volumes

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
)

var (
	volumeUnmountDescription = `Unmount a volume`
	volumeUnmountCommand     = &cobra.Command{
		Annotations:       map[string]string{registry.EngineMode: registry.ABIMode},
		Use:               "unmount NAME",
		Short:             "Unmount volume",
		Long:              volumeUnmountDescription,
		RunE:              volumeUnmount,
		Example:           `podman volume unmount myvol`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: common.AutocompleteVolumes,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: volumeUnmountCommand,
		Parent:  volumeCmd,
	})
}

func volumeUnmount(_ *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	reports, err := registry.ContainerEngine().VolumeUnmount(registry.Context(), args)
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
