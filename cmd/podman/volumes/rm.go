package volumes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	volumeRmDescription = `Remove one or more existing volumes.

  By default only volumes that are not being used by any containers will be removed. To remove the volumes anyways, use the --force flag.`
	rmCommand = &cobra.Command{
		Use:               "rm [options] VOLUME [VOLUME...]",
		Aliases:           []string{"remove"},
		Short:             "Remove one or more volumes",
		Long:              volumeRmDescription,
		RunE:              rm,
		ValidArgsFunction: common.AutocompleteVolumes,
		Example: `podman volume rm myvol1 myvol2
podman volume rm --all
podman volume rm --force myvol`,
	}
)

var (
	rmOptions   = entities.VolumeRmOptions{}
	stopTimeout int
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCommand,
		Parent:  volumeCmd,
	})
	flags := rmCommand.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all volumes")
	flags.BoolVarP(&rmOptions.Force, "force", "f", false, "Remove a volume by force, even if it is being used by a container")
	timeFlagName := "time"
	flags.IntVarP(&stopTimeout, timeFlagName, "t", int(containerConfig.Engine.StopTimeout), "Seconds to wait for running containers to stop before killing the container")
	_ = rmCommand.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)
}

func rm(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	if (len(args) > 0 && rmOptions.All) || (len(args) < 1 && !rmOptions.All) {
		return errors.New("choose either one or more volumes or all")
	}
	if cmd.Flag("time").Changed {
		if !rmOptions.Force {
			return errors.New("--force option must be specified to use the --time option")
		}
		timeout := uint(stopTimeout)
		rmOptions.Timeout = &timeout
	}
	responses, err := registry.ContainerEngine().VolumeRm(context.Background(), args, rmOptions)
	if err != nil {
		if rmOptions.Force && strings.Contains(err.Error(), define.ErrNoSuchVolume.Error()) {
			return nil
		}
		setExitCode(err)
		return err
	}
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.Id)
		} else {
			if rmOptions.Force && strings.Contains(r.Err.Error(), define.ErrNoSuchVolume.Error()) {
				continue
			}
			setExitCode(r.Err)
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}

func setExitCode(err error) {
	if errors.Is(err, define.ErrNoSuchVolume) || strings.Contains(err.Error(), define.ErrNoSuchVolume.Error()) {
		registry.SetExitCode(1)
	} else if errors.Is(err, define.ErrVolumeBeingUsed) || strings.Contains(err.Error(), define.ErrVolumeBeingUsed.Error()) {
		registry.SetExitCode(2)
	}
}
