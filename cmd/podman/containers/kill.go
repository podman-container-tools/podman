package containers

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
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/signal"
)

var (
	killDescription = "The main process inside each container specified will be sent SIGKILL, or any signal specified with option --signal."
	killCommand     = &cobra.Command{
		Use:   "kill [options] CONTAINER [CONTAINER...]",
		Short: "Kill one or more running containers with a specific signal",
		Long:  killDescription,
		RunE:  kill,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: common.AutocompleteContainersRunning,
		Example: `podman kill mywebserver
podman kill 860a4b23
podman kill --signal TERM ctrID`,
	}

	containerKillCommand = &cobra.Command{
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		Use:               killCommand.Use,
		Short:             killCommand.Short,
		Long:              killCommand.Long,
		RunE:              killCommand.RunE,
		ValidArgsFunction: killCommand.ValidArgsFunction,
		Example: `podman container kill mywebserver
podman container kill 860a4b23
podman container kill --signal TERM ctrID`,
	}
)

var (
	killOptions  = entities.KillOptions{}
	killCidFiles = []string{}
)

func killFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&killOptions.All, "all", "a", false, "Signal all running containers")

	signalFlagName := "signal"
	flags.StringVarP(&killOptions.Signal, signalFlagName, "s", "KILL", "Signal to send to the container")
	_ = cmd.RegisterFlagCompletionFunc(signalFlagName, common.AutocompleteStopSignal)
	cidfileFlagName := "cidfile"
	flags.StringArrayVar(&killCidFiles, cidfileFlagName, nil, "Read the container ID from the file")
	_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: killCommand,
	})
	killFlags(killCommand)
	validate.AddLatestFlag(killCommand, &killOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerKillCommand,
		Parent:  containerCmd,
	})
	killFlags(containerKillCommand)
	validate.AddLatestFlag(containerKillCommand, &killOptions.Latest)
}

func kill(_ *cobra.Command, args []string) error {
	var (
		err  error
		errs utils.OutputErrors
	)
	args = utils.RemoveSlash(args)
	// Check if the signalString provided by the user is valid
	// Invalid signals will return err
	sig, err := signal.ParseSignalNameOrNumber(killOptions.Signal)
	if err != nil {
		return err
	}
	if sig < 1 || sig > 64 {
		return errors.New("valid signals are 1 through 64")
	}
	for _, cidFile := range killCidFiles {
		content, err := os.ReadFile(cidFile)
		if err != nil {
			return fmt.Errorf("reading CIDFile: %w", err)
		}
		id, _, _ := strings.Cut(string(content), "\n")
		args = append(args, id)
	}

	responses, err := registry.ContainerEngine().ContainerKill(context.Background(), args, killOptions)
	if err != nil {
		return err
	}
	for _, r := range responses {
		switch {
		case r.Err != nil:
			errs = append(errs, r.Err)
		case r.RawInput != "":
			fmt.Println(r.RawInput)
		default:
			fmt.Println(r.Id)
		}
	}
	return errs.PrintErrors()
}
