package pods

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	podKillDescription = `Signals are sent to the main process of each container inside the specified pod.

  The default signal is SIGKILL, or any signal specified with option --signal.`
	killCommand = &cobra.Command{
		Use:   "kill [options] POD [POD...]",
		Short: "Send the specified signal or SIGKILL to containers in pod",
		Long:  podKillDescription,
		RunE:  kill,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "")
		},
		ValidArgsFunction: common.AutocompletePodsRunning,
		Example: `podman pod kill podID
podman pod kill --signal TERM mywebserver`,
	}
)

var killOpts entities.PodKillOptions

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: killCommand,
		Parent:  podCmd,
	})
	flags := killCommand.Flags()
	flags.BoolVarP(&killOpts.All, "all", "a", false, "Kill all containers in all pods")

	signalFlagName := "signal"
	flags.StringVarP(&killOpts.Signal, signalFlagName, "s", "KILL", "Signal to send to the containers in the pod")
	_ = killCommand.RegisterFlagCompletionFunc(signalFlagName, common.AutocompleteStopSignal)

	validate.AddLatestFlag(killCommand, &killOpts.Latest)
}

func kill(_ *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	responses, err := registry.ContainerEngine().PodKill(context.Background(), args, killOpts)
	if err != nil {
		return err
	}
	// in the cli, first we print out all the successful attempts
	for _, r := range responses {
		if len(r.Errs) == 0 {
			fmt.Println(r.Id)
		} else {
			errs = append(errs, r.Errs...)
		}
	}
	return errs.PrintErrors()
}
