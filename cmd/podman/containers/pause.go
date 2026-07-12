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
	pauseDescription = `Pauses one or more running containers.  The container name or ID can be used.`
	pauseCommand     = &cobra.Command{
		Use:   "pause [options] CONTAINER [CONTAINER...]",
		Short: "Pause all the processes in one or more containers",
		Long:  pauseDescription,
		RunE:  pause,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: common.AutocompleteContainersRunning,
		Example: `podman pause mywebserver
podman pause 860a4b23
podman pause --all`,
	}

	containerPauseCommand = &cobra.Command{
		Use:   pauseCommand.Use,
		Short: pauseCommand.Short,
		Long:  pauseCommand.Long,
		RunE:  pauseCommand.RunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: pauseCommand.ValidArgsFunction,
		Example: `podman container pause mywebserver
podman container pause 860a4b23
podman container pause --all`,
	}
)

var (
	pauseOpts = entities.PauseUnPauseOptions{
		Filters: make(map[string][]string),
	}
	pauseCidFiles = []string{}
)

func pauseFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&pauseOpts.All, "all", "a", false, "Pause all running containers")

	cidfileFlagName := "cidfile"
	flags.StringArrayVar(&pauseCidFiles, cidfileFlagName, nil, "Read the container ID from the file")
	_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)

	filterFlagName := "filter"
	flags.StringArrayVarP(&filters, filterFlagName, "f", []string{}, "Filter output based on conditions given")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePsFilters)

	if registry.IsRemote() {
		_ = flags.MarkHidden("cidfile")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: pauseCommand,
	})
	pauseFlags(pauseCommand)
	validate.AddLatestFlag(pauseCommand, &pauseOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerPauseCommand,
		Parent:  containerCmd,
	})
	pauseFlags(containerPauseCommand)
	validate.AddLatestFlag(containerPauseCommand, &pauseOpts.Latest)
}

func pause(_ *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	args = utils.RemoveSlash(args)

	for _, cidFile := range pauseCidFiles {
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
		pauseOpts.Filters[fname] = append(pauseOpts.Filters[fname], filter)
	}

	responses, err := registry.ContainerEngine().ContainerPause(context.Background(), args, pauseOpts)
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
