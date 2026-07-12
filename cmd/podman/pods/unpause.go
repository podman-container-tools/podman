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
	podUnpauseDescription = `The podman unpause command will unpause all "paused" containers assigned to the pod.

  The pod name or ID can be used.`
	unpauseCommand = &cobra.Command{
		Use:   "unpause [options] POD [POD...]",
		Short: "Unpause one or more pods",
		Long:  podUnpauseDescription,
		RunE:  unpause,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "")
		},
		ValidArgsFunction: common.AutoCompletePodsPause,
		Example: `podman pod unpause podID1 podID2
podman pod unpause --all`,
	}
)

var unpauseOptions entities.PodunpauseOptions

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: unpauseCommand,
		Parent:  podCmd,
	})
	flags := unpauseCommand.Flags()
	flags.BoolVarP(&unpauseOptions.All, "all", "a", false, "Unpause all running pods")
	validate.AddLatestFlag(unpauseCommand, &unpauseOptions.Latest)
}

func unpause(_ *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	responses, err := registry.ContainerEngine().PodUnpause(context.Background(), args, unpauseOptions)
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
