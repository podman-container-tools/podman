package containers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	waitDescription = `Block until one or more containers stop and then print their exit codes.
`
	waitCommand = &cobra.Command{
		Use:               "wait [options] CONTAINER [CONTAINER...]",
		Short:             "Block on one or more containers",
		Long:              waitDescription,
		RunE:              wait,
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman wait --interval 5s ctrID
podman wait ctrID1 ctrID2`,
	}

	containerWaitCommand = &cobra.Command{
		Use:               waitCommand.Use,
		Short:             waitCommand.Short,
		Long:              waitCommand.Long,
		RunE:              waitCommand.RunE,
		ValidArgsFunction: waitCommand.ValidArgsFunction,
		Example: `podman container wait --interval 5s ctrID
podman container wait ctrID1 ctrID2`,
	}
)

var (
	waitOptions  = entities.WaitOptions{}
	waitInterval string
)

func waitFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	intervalFlagName := "interval"
	flags.StringVarP(&waitInterval, intervalFlagName, "i", "250ms", "Time Interval to wait before polling for completion")
	_ = cmd.RegisterFlagCompletionFunc(intervalFlagName, completion.AutocompleteNone)

	flags.BoolVarP(&waitOptions.Ignore, "ignore", "", false, "Ignore if a container does not exist")

	conditionFlagName := "condition"
	flags.StringSliceVar(&waitOptions.Conditions, conditionFlagName, []string{}, "Condition to wait on")
	_ = cmd.RegisterFlagCompletionFunc(conditionFlagName, common.AutocompleteWaitCondition)

	waitExitFirst := "exit-first-match"
	flags.BoolVar(&waitOptions.ExitFirstMatch, waitExitFirst, false, "Wait for exit of first container which matches conditions, ignore other ones")
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: waitCommand,
	})
	waitFlags(waitCommand)
	validate.AddLatestFlag(waitCommand, &waitOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerWaitCommand,
		Parent:  containerCmd,
	})
	waitFlags(containerWaitCommand)
	validate.AddLatestFlag(containerWaitCommand, &waitOptions.Latest)
}

func wait(cmd *cobra.Command, args []string) error {
	var (
		err  error
		errs utils.OutputErrors
	)
	args = utils.RemoveSlash(args)
	if waitOptions.Interval, err = time.ParseDuration(waitInterval); err != nil {
		var err1 error
		if waitOptions.Interval, err1 = time.ParseDuration(waitInterval + "ms"); err1 != nil {
			return err
		}
	}

	if !waitOptions.Latest && len(args) == 0 {
		return fmt.Errorf("%q requires a name, id, or the \"--latest\" flag", cmd.CommandPath())
	}
	if waitOptions.Latest && len(args) > 0 {
		return errors.New("--latest and containers cannot be used together")
	}

	responses, err := registry.ContainerEngine().ContainerWait(context.Background(), args, waitOptions)
	if err != nil {
		return err
	}
	for _, r := range responses {
		if r.Error == nil {
			fmt.Println(r.ExitCode)
		} else {
			errs = append(errs, r.Error)
		}
	}
	return errs.PrintErrors()
}
