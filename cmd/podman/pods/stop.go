package pods

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/specgenutil"
)

// allows for splitting API and CLI-only options
type podStopOptionsWrapper struct {
	entities.PodStopOptions

	podIDFiles []string
	timeoutCLI int
}

var (
	stopOptions = podStopOptionsWrapper{
		PodStopOptions: entities.PodStopOptions{Timeout: -1},
	}
	podStopDescription = `The pod name or ID can be used.

  This command will stop all running containers in each of the specified pods.`

	stopCommand = &cobra.Command{
		Use:   "stop [options] POD [POD...]",
		Short: "Stop one or more pods",
		Long:  podStopDescription,
		RunE:  stop,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "pod-id-file")
		},
		ValidArgsFunction: common.AutocompletePodsRunning,
		Example: `podman pod stop mywebserverpod
podman pod stop --time 0 490eb 3557fb`,
	}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: stopCommand,
		Parent:  podCmd,
	})
	flags := stopCommand.Flags()
	flags.BoolVarP(&stopOptions.All, "all", "a", false, "Stop all running pods")
	flags.BoolVarP(&stopOptions.Ignore, "ignore", "i", false, "Ignore errors when a specified pod is missing")

	timeFlagName := "time"
	flags.IntVarP(&stopOptions.timeoutCLI, timeFlagName, "t", int(containerConfig.Engine.StopTimeout), "Seconds to wait for pod stop before killing the container")
	_ = stopCommand.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)

	podIDFileFlagName := "pod-id-file"
	flags.StringArrayVarP(&stopOptions.podIDFiles, podIDFileFlagName, "", nil, "Write the pod ID to the file")
	_ = stopCommand.RegisterFlagCompletionFunc(podIDFileFlagName, completion.AutocompleteDefault)

	validate.AddLatestFlag(stopCommand, &stopOptions.Latest)

	if registry.IsRemote() {
		_ = flags.MarkHidden("ignore")
	}

	flags.SetNormalizeFunc(utils.AliasFlags)
}

func stop(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	if cmd.Flag("time").Changed {
		stopOptions.Timeout = stopOptions.timeoutCLI
	}

	ids, err := specgenutil.ReadPodIDFiles(stopOptions.podIDFiles)
	if err != nil {
		return err
	}
	args = append(args, ids...)

	responses, err := registry.ContainerEngine().PodStop(context.Background(), args, stopOptions.PodStopOptions)
	if err != nil {
		return err
	}
	// in the cli, first we print out all the successful attempts
	for _, r := range responses {
		if len(r.Errs) == 0 {
			fmt.Println(r.RawInput)
		} else {
			errs = append(errs, r.Errs...)
		}
	}
	return errs.PrintErrors()
}
