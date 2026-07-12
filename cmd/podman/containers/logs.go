package containers

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

// logsOptionsWrapper wraps entities.LogsOptions and prevents leaking
// CLI-only fields into the API types.
type logsOptionsWrapper struct {
	entities.ContainerLogsOptions

	SinceRaw string

	UntilRaw string
}

var (
	logsOptions     logsOptionsWrapper
	logsDescription = `Retrieves logs for one or more containers.

  This does not guarantee execution order when combined with podman run (i.e., your run may not have generated any logs at the time you execute podman logs).
`
	logsCommand = &cobra.Command{
		Use:   "logs [options] CONTAINER [CONTAINER...]",
		Short: "Fetch the logs of one or more containers",
		Long:  logsDescription,
		Args: func(cmd *cobra.Command, args []string) error {
			switch {
			case registry.IsRemote() && logsOptions.Latest:
				return errors.New(cmd.Name() + " does not support 'latest' when run remotely")
			case registry.IsRemote() && len(args) > 1:
				return errors.New(cmd.Name() + " does not support multiple containers when run remotely")
			case logsOptions.Latest && len(args) > 0:
				return errors.New("--latest and containers cannot be used together")
			case !logsOptions.Latest && len(args) < 1:
				return errors.New("specify at least one container name or ID to log")
			}
			return nil
		},
		RunE:              logs,
		ValidArgsFunction: common.AutocompleteContainers,
		Example: `podman logs ctrID
podman logs --names ctrID1 ctrID2
podman logs --tail 2 mywebserver
podman logs --follow=true --since 10m ctrID
podman logs mywebserver mydbserver`,
	}

	containerLogsCommand = &cobra.Command{
		Use:               logsCommand.Use,
		Short:             logsCommand.Short,
		Long:              logsCommand.Long,
		Args:              logsCommand.Args,
		RunE:              logsCommand.RunE,
		ValidArgsFunction: logsCommand.ValidArgsFunction,
		Example: `podman container logs ctrID
podman container logs --names ctrID1 ctrID2
podman container logs --color --names ctrID1 ctrID2
podman container logs --tail 2 mywebserver
podman container logs --follow=true --since 10m ctrID
podman container logs mywebserver mydbserver`,
	}
)

func init() {
	// if run remotely we only allow one container arg
	if registry.IsRemote() {
		logsCommand.Use = "logs [options] CONTAINER"
		containerLogsCommand.Use = logsCommand.Use
	}

	// logs
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: logsCommand,
	})
	logsFlags(logsCommand)
	validate.AddLatestFlag(logsCommand, &logsOptions.Latest)

	// container logs
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerLogsCommand,
		Parent:  containerCmd,
	})
	logsFlags(containerLogsCommand)
	validate.AddLatestFlag(containerLogsCommand, &logsOptions.Latest)
}

func logsFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVar(&logsOptions.Details, "details", false, "Show extra details provided to the logs")
	flags.BoolVarP(&logsOptions.Follow, "follow", "f", false, "Follow log output.  The default is false")

	sinceFlagName := "since"
	flags.StringVar(&logsOptions.SinceRaw, sinceFlagName, "", "Show logs since TIMESTAMP")
	_ = cmd.RegisterFlagCompletionFunc(sinceFlagName, completion.AutocompleteNone)

	untilFlagName := "until"
	flags.StringVar(&logsOptions.UntilRaw, untilFlagName, "", "Show logs until TIMESTAMP")
	_ = cmd.RegisterFlagCompletionFunc(untilFlagName, completion.AutocompleteNone)

	tailFlagName := "tail"
	flags.Int64Var(&logsOptions.Tail, tailFlagName, -1, "Output the specified number of LINES at the end of the logs.  Defaults to -1, which prints all lines")
	_ = cmd.RegisterFlagCompletionFunc(tailFlagName, completion.AutocompleteNone)

	flags.BoolVarP(&logsOptions.Timestamps, "timestamps", "t", false, "Output the timestamps in the log")
	flags.BoolVarP(&logsOptions.Colors, "color", "", false, "Output the containers with different colors in the log.")
	flags.BoolVarP(&logsOptions.Names, "names", "n", false, "Output the container name in the log")

	_ = flags.MarkHidden("details")
}

func logs(_ *cobra.Command, args []string) error {
	args = utils.RemoveSlash(args)
	if logsOptions.SinceRaw != "" {
		// parse time, error out if something is wrong
		since, err := util.ParseInputTime(logsOptions.SinceRaw, true)
		if err != nil {
			return fmt.Errorf("parsing --since %q: %w", logsOptions.SinceRaw, err)
		}
		logsOptions.Since = since
	}
	if logsOptions.UntilRaw != "" {
		// parse time, error out if something is wrong
		until, err := util.ParseInputTime(logsOptions.UntilRaw, false)
		if err != nil {
			return fmt.Errorf("parsing --until %q: %w", logsOptions.UntilRaw, err)
		}
		logsOptions.Until = until
	}
	logsOptions.StdoutWriter = os.Stdout
	logsOptions.StderrWriter = os.Stderr
	return registry.ContainerEngine().ContainerLogs(registry.Context(), args, logsOptions.ContainerLogsOptions)
}
