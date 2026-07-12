package pods

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgenutil"
)

// allows for splitting API and CLI-only options
type podRmOptionsWrapper struct {
	entities.PodRmOptions

	PodIDFiles []string
}

var (
	rmOptions        = podRmOptionsWrapper{}
	podRmDescription = `podman rm will remove one or more stopped pods and their containers from the host.

  The pod name or ID can be used.  A pod with containers will not be removed without --force. If --force is specified, all containers will be stopped, then removed.`
	rmCommand = &cobra.Command{
		Use:   "rm [options] POD [POD...]",
		Short: "Remove one or more pods",
		Long:  podRmDescription,
		RunE:  rm,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "pod-id-file")
		},
		ValidArgsFunction: common.AutocompletePods,
		Example: `podman pod rm mywebserverpod
podman pod rm -f 860a4b23
podman pod rm -f -a`,
	}
	stopTimeout int
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: rmCommand,
		Parent:  podCmd,
	})

	flags := rmCommand.Flags()
	flags.BoolVarP(&rmOptions.All, "all", "a", false, "Remove all running pods")
	flags.BoolVarP(&rmOptions.Force, "force", "f", false, "Force removal of a running pod by first stopping all containers, then removing all containers in the pod.  The default is false")
	flags.BoolVarP(&rmOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified pod is missing")

	podIDFileFlagName := "pod-id-file"
	flags.StringArrayVarP(&rmOptions.PodIDFiles, podIDFileFlagName, "", nil, "Read the pod ID from the file")
	_ = rmCommand.RegisterFlagCompletionFunc(podIDFileFlagName, completion.AutocompleteDefault)

	timeFlagName := "time"
	flags.IntVarP(&stopTimeout, timeFlagName, "t", int(containerConfig.Engine.StopTimeout), "Seconds to wait for pod stop before killing the container")
	_ = rmCommand.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)

	validate.AddLatestFlag(rmCommand, &rmOptions.Latest)

	if registry.IsRemote() {
		_ = flags.MarkHidden("ignore")
	}
}

func rm(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors

	if cmd.Flag("time").Changed {
		if !rmOptions.Force {
			return errors.New("--force option must be specified to use the --time option")
		}
		timeout := uint(stopTimeout)
		rmOptions.Timeout = &timeout
	}

	if rmOptions.Force {
		rmOptions.Ignore = true
	}

	errs = append(errs, removePods(args, rmOptions.PodRmOptions, true)...)

	for _, idFile := range rmOptions.PodIDFiles {
		id, err := specgenutil.ReadPodIDFile(idFile)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		rmErrs := removePods([]string{id}, rmOptions.PodRmOptions, true)
		errs = append(errs, rmErrs...)
		if len(rmErrs) == 0 {
			if err := os.Remove(idFile); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errs.PrintErrors()
}

// removePods removes the specified pods (names or IDs).  Allows for sharing
// pod-removal logic across commands.
func removePods(namesOrIDs []string, rmOptions entities.PodRmOptions, printIDs bool) utils.OutputErrors {
	var errs utils.OutputErrors

	responses, err := registry.ContainerEngine().PodRm(context.Background(), namesOrIDs, rmOptions)
	if err != nil {
		setExitCode(err)
		errs = append(errs, err)
		if !strings.Contains(err.Error(), define.ErrRemovingCtrs.Error()) {
			return errs
		}
	}

	// in the cli, first we print out all the successful attempts
	for _, r := range responses {
		if r.Err == nil {
			if printIDs {
				fmt.Println(r.Id)
			}
		} else {
			setExitCode(r.Err)
			errs = append(errs, r.Err)
			for ctr, err := range r.RemovedCtrs {
				if err != nil {
					errs = append(errs, fmt.Errorf("error removing container %s from pod %s: %w", ctr, r.Id, err))
				}
			}
		}
	}
	return errs
}

func setExitCode(err error) {
	if errors.Is(err, define.ErrNoSuchPod) || strings.Contains(err.Error(), define.ErrNoSuchPod.Error()) {
		registry.SetExitCode(1)
	}
}
