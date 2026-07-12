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
	unpauseDescription = `Unpauses one or more previously paused containers.  The container name or ID can be used.`
	unpauseCommand     = &cobra.Command{
		Use:   "unpause [options] CONTAINER [CONTAINER...]",
		Short: "Unpause the processes in one or more containers",
		Long:  unpauseDescription,
		RunE:  unpause,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: common.AutocompleteContainersPaused,
		Example: `podman unpause ctrID
podman unpause --all`,
	}

	containerUnpauseCommand = &cobra.Command{
		Use:   unpauseCommand.Use,
		Short: unpauseCommand.Short,
		Long:  unpauseCommand.Long,
		RunE:  unpauseCommand.RunE,
		Args: func(cmd *cobra.Command, args []string) error {
			return validate.CheckAllLatestAndIDFile(cmd, args, false, "cidfile")
		},
		ValidArgsFunction: unpauseCommand.ValidArgsFunction,
		Example: `podman container unpause ctrID
podman container unpause --all`,
	}
)

var (
	unpauseOpts = entities.PauseUnPauseOptions{
		Filters: make(map[string][]string),
	}
	unpauseCidFiles = []string{}
)

func unpauseFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&unpauseOpts.All, "all", "a", false, "Unpause all paused containers")

	cidfileFlagName := "cidfile"
	flags.StringArrayVar(&unpauseCidFiles, cidfileFlagName, nil, "Read the container ID from the file")
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
		Command: unpauseCommand,
	})
	unpauseFlags(unpauseCommand)
	validate.AddLatestFlag(unpauseCommand, &unpauseOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerUnpauseCommand,
		Parent:  containerCmd,
	})
	unpauseFlags(containerUnpauseCommand)
	validate.AddLatestFlag(containerUnpauseCommand, &unpauseOpts.Latest)
}

func unpause(_ *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	args = utils.RemoveSlash(args)

	for _, cidFile := range unpauseCidFiles {
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
		unpauseOpts.Filters[fname] = append(unpauseOpts.Filters[fname], filter)
	}

	responses, err := registry.ContainerEngine().ContainerUnpause(context.Background(), args, unpauseOpts)
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
