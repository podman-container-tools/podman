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
type podStartOptionsWrapper struct {
	entities.PodStartOptions

	PodIDFiles []string
}

var (
	podStartDescription = `The pod name or ID can be used.

  All containers defined in the pod will be started.`
	startCommand = &cobra.Command{
		Use:   "start [options] POD [POD...]",
		Short: "Start one or more pods",
		Long:  podStartDescription,
		RunE:  start,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "pod-id-file")
		},
		ValidArgsFunction: common.AutocompletePods,
		Example: `podman pod start podID
podman pod start --all`,
	}
)

var startOptions = podStartOptionsWrapper{}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: startCommand,
		Parent:  podCmd,
	})

	flags := startCommand.Flags()
	flags.BoolVarP(&startOptions.All, "all", "a", false, "Restart all running pods")

	podIDFileFlagName := "pod-id-file"
	flags.StringArrayVarP(&startOptions.PodIDFiles, podIDFileFlagName, "", nil, "Read the pod ID from the file")
	_ = startCommand.RegisterFlagCompletionFunc(podIDFileFlagName, completion.AutocompleteDefault)

	validate.AddLatestFlag(startCommand, &startOptions.Latest)
}

func start(_ *cobra.Command, args []string) error {
	var errs utils.OutputErrors

	ids, err := specgenutil.ReadPodIDFiles(startOptions.PodIDFiles)
	if err != nil {
		return err
	}
	args = append(args, ids...)

	responses, err := registry.ContainerEngine().PodStart(context.Background(), args, startOptions.PodStartOptions)
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
