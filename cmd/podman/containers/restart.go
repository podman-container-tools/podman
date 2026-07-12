package containers

import (
	"context"
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
)

var (
	restartDescription = fmt.Sprintf(`Restarts one or more running containers. The container ID or name can be used.

  A timeout before forcibly stopping can be set, but defaults to %d seconds.`, containerConfig.Engine.StopTimeout)

	restartCommand = &cobra.Command{
		Use:   "restart [options] CONTAINER [CONTAINER...]",
		Short: "Restart one or more containers",
		Long:  restartDescription,
		RunE:  restart,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman restart ctrID
podman restart ctrID1 ctrID2`,
	}

	containerRestartCommand = &cobra.Command{
		Use:               restartCommand.Use,
		Short:             restartCommand.Short,
		Long:              restartCommand.Long,
		RunE:              restartCommand.RunE,
		Args:              restartCommand.Args,
		ValidArgsFunction: restartCommand.ValidArgsFunction,
		Example: `podman container restart ctrID
podman container restart ctrID1 ctrID2`,
	}
)

var (
	restartOpts = entities.RestartOptions{
		Filters: make(map[string][]string),
	}
	restartCidFiles = []string{}
	restartTimeout  int
)

func restartFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&restartOpts.All, "all", "a", false, "Restart all non-running containers")
	flags.BoolVar(&restartOpts.Running, "running", false, "Restart only running containers")

	cidfileFlagName := "cidfile"
	flags.StringArrayVar(&restartCidFiles, cidfileFlagName, nil, "Read the container ID from the file")
	_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)

	filterFlagName := "filter"
	flags.StringArrayVarP(&filters, filterFlagName, "f", []string{}, "Filter output based on conditions given")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePsFilters)

	timeFlagName := "time"
	flags.IntVarP(&restartTimeout, timeFlagName, "t", int(containerConfig.Engine.StopTimeout), "Seconds to wait for stop before killing the container")
	_ = cmd.RegisterFlagCompletionFunc(timeFlagName, completion.AutocompleteNone)

	if registry.IsRemote() {
		_ = flags.MarkHidden("cidfile")
	}

	flags.SetNormalizeFunc(utils.AliasFlags)
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: restartCommand,
	})
	restartFlags(restartCommand)
	validate.AddLatestFlag(restartCommand, &restartOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerRestartCommand,
		Parent:  containerCmd,
	})
	restartFlags(containerRestartCommand)
	validate.AddLatestFlag(containerRestartCommand, &restartOpts.Latest)
}

func restart(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	args = utils.RemoveSlash(args)

	if cmd.Flag("time").Changed {
		timeout := uint(restartTimeout)
		restartOpts.Timeout = &timeout
	}

	for _, cidFile := range restartCidFiles {
		content, err := os.ReadFile(cidFile)
		if err != nil {
			return fmt.Errorf("reading CIDFile: %w", err)
		}
		id, _, _ := strings.Cut(string(content), "\n")
		args = append(args, id)
	}

	for _, f := range filters {
		fname, filter, hasFilter := strings.Cut(f, "=")
		if !hasFilter {
			return fmt.Errorf("invalid filter %q", f)
		}
		restartOpts.Filters[fname] = append(restartOpts.Filters[fname], filter)
	}

	responses, err := registry.ContainerEngine().ContainerRestart(context.Background(), args, restartOpts)
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
