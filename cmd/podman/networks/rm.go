package network

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	networkrmDescription = `Remove networks`
	networkrmCommand     = &cobra.Command{
		Use:               "rm [options] NETWORK [NETWORK...]",
		Aliases:           []string{"remove"},
		Short:             "Remove networks",
		Long:              networkrmDescription,
		RunE:              networkRm,
		Example:           `podman network rm podman`,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: common.AutocompleteNetworks,
	}
	stopTimeout int
)

var networkRmOptions entities.NetworkRmOptions

func networkRmFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&networkRmOptions.Force, "force", "f", false, "remove any containers using network")
	flags.BoolVarP(&networkRmOptions.Ignore, "ignore", "i", false, "ignore if a specified network does not exist")
	timeFlagName := "time"
	flags.IntVarP(&stopTimeout, timeFlagName, "t", int(containerConfig.Engine.StopTimeout), "Seconds to wait for running containers to stop before killing the container")
	_ = networkrmCommand.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: networkrmCommand,
		Parent:  networkCmd,
	})
	flags := networkrmCommand.Flags()
	networkRmFlags(flags)
}

func networkRm(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors

	if cmd.Flag("time").Changed {
		if !networkRmOptions.Force {
			return errors.New("--force option must be specified to use the --time option")
		}
		timeout := uint(stopTimeout)
		networkRmOptions.Timeout = &timeout
	}
	responses, err := registry.ContainerEngine().NetworkRm(registry.Context(), args, networkRmOptions)
	if err != nil {
		if (networkRmOptions.Force || networkRmOptions.Ignore) && strings.Contains(err.Error(), define.ErrNoSuchNetwork.Error()) {
			return nil
		}
		setExitCode(err)
		return err
	}
	for _, r := range responses {
		if r.Err == nil {
			fmt.Println(r.Name)
		} else {
			if (networkRmOptions.Force || networkRmOptions.Ignore) && strings.Contains(r.Err.Error(), define.ErrNoSuchNetwork.Error()) {
				continue
			}
			setExitCode(r.Err)
			errs = append(errs, r.Err)
		}
	}
	return errs.PrintErrors()
}

func setExitCode(err error) {
	if errors.Is(err, define.ErrNoSuchNetwork) || strings.Contains(err.Error(), define.ErrNoSuchNetwork.Error()) {
		registry.SetExitCode(1)
	} else if errors.Is(err, define.ErrNetworkInUse) || strings.Contains(err.Error(), define.ErrNetworkInUse.Error()) {
		registry.SetExitCode(2)
	}
}
