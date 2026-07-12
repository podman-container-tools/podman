package containers

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var (
	startDescription = `Starts one or more containers.  The container name or ID can be used.`
	startCommand     = &cobra.Command{
		Use:               "start [options] CONTAINER [CONTAINER...]",
		Short:             "Start one or more containers",
		Long:              startDescription,
		RunE:              start,
		Args:              validateStart,
		ValidArgsFunction: common.AutocompleteContainersStartable,
		Example: `podman start 860a4b231279 5421ab43b45
podman start --interactive --attach imageID`,
	}

	containerStartCommand = &cobra.Command{
		Use:               startCommand.Use,
		Short:             startCommand.Short,
		Long:              startCommand.Long,
		RunE:              startCommand.RunE,
		Args:              startCommand.Args,
		ValidArgsFunction: startCommand.ValidArgsFunction,
		Example: `podman container start 860a4b231279 5421ab43b45
podman container start --interactive --attach imageID`,
	}
)

var startOptions = entities.ContainerStartOptions{
	Filters: make(map[string][]string),
}

func startFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVarP(&startOptions.Attach, "attach", "a", false, "Attach container's STDOUT and STDERR")

	detachKeysFlagName := "detach-keys"
	flags.StringVar(&startOptions.DetachKeys, detachKeysFlagName, containerConfig.DetachKeys(), "Select the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-z`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	_ = cmd.RegisterFlagCompletionFunc(detachKeysFlagName, common.AutocompleteDetachKeys)

	flags.BoolVarP(&startOptions.Interactive, "interactive", "i", false, "Make STDIN available to the contained process")
	flags.BoolVar(&startOptions.SigProxy, "sig-proxy", false, "Proxy received signals to the process (default true if attaching, false otherwise)")

	filterFlagName := "filter"
	flags.StringArrayVarP(&filters, filterFlagName, "f", []string{}, "Filter output based on conditions given")
	_ = cmd.RegisterFlagCompletionFunc(filterFlagName, common.AutocompletePsFilters)

	flags.BoolVar(&startOptions.All, "all", false, "Start all containers regardless of their state or configuration")

	if registry.IsRemote() {
		_ = flags.MarkHidden("sig-proxy")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: startCommand,
	})
	startFlags(startCommand)
	validate.AddLatestFlag(startCommand, &startOptions.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerStartCommand,
		Parent:  containerCmd,
	})
	startFlags(containerStartCommand)
	validate.AddLatestFlag(containerStartCommand, &startOptions.Latest)
}

func validateStart(_ *cobra.Command, args []string) error {
	if len(args) == 0 && !startOptions.Latest && !startOptions.All && len(filters) < 1 {
		return errors.New("start requires at least one argument")
	}
	if startOptions.All && startOptions.Latest {
		return errors.New("--all and --latest cannot be used together")
	}
	if len(args) > 0 && startOptions.Latest {
		return errors.New("--latest and containers cannot be used together")
	}
	if len(args) > 1 && startOptions.Attach {
		return errors.New("you cannot start and attach multiple containers at once")
	}
	if (len(args) > 0 || startOptions.Latest) && startOptions.All {
		return errors.New("either start all containers or the container(s) provided in the arguments")
	}
	if startOptions.Attach && startOptions.All {
		return errors.New("you cannot start and attach all containers at once")
	}
	return nil
}

func start(cmd *cobra.Command, args []string) error {
	var errs utils.OutputErrors
	sigProxy := startOptions.SigProxy || startOptions.Attach
	if cmd.Flag("sig-proxy").Changed {
		sigProxy = startOptions.SigProxy
	}
	startOptions.SigProxy = sigProxy

	if sigProxy && !startOptions.Attach {
		return fmt.Errorf("you cannot use sig-proxy without --attach: %w", define.ErrInvalidArg)
	}
	if startOptions.Attach {
		startOptions.Stdin = os.Stdin
		startOptions.Stderr = os.Stderr
		startOptions.Stdout = os.Stdout
	}

	containers := utils.RemoveSlash(args)
	for _, f := range filters {
		fname, filter, hasFilter := strings.Cut(f, "=")
		if !hasFilter {
			return fmt.Errorf("invalid filter %q", f)
		}
		startOptions.Filters[fname] = append(startOptions.Filters[fname], filter)
	}

	responses, err := registry.ContainerEngine().ContainerStart(registry.Context(), containers, startOptions)
	if err != nil {
		return err
	}
	for _, r := range responses {
		switch {
		case r.Err != nil:
			errs = append(errs, r.Err)
		case startOptions.Attach:
			// Implement the exitcode when the only one container is enabled attach
			registry.SetExitCode(r.ExitCode)
		case r.RawInput != "":
			fmt.Println(r.RawInput)
		default:
			fmt.Println(r.Id)
		}
	}
	return errs.PrintErrors()
}
