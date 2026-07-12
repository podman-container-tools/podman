package pods

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/util"
)

// logsOptionsWrapper wraps entities.LogsOptions and prevents leaking
// CLI-only fields into the API types.
type logsOptionsWrapper struct {
	entities.PodLogsOptions

	SinceRaw string

	UntilRaw string
}

var (
	logsPodOptions     logsOptionsWrapper
	logsPodDescription = `Displays logs for pod with one or more containers.`
	podLogsCommand     = &cobra.Command{
		Use:   "logs [options] POD",
		Short: "Fetch logs for pod with one or more containers",
		Long:  logsPodDescription,
		// We dont want users to invoke latest and pod together
		Args: func(cmd *cobra.Command, args []string) error {
			switch {
			case registry.IsRemote() && logsPodOptions.Latest:
				return errors.New(cmd.Name() + " does not support 'latest' when run remotely")
			case len(args) > 1:
				return errors.New("requires exactly 1 arg")
			case logsPodOptions.Latest && len(args) > 0:
				return errors.New("--latest and pods cannot be used together")
			case !logsPodOptions.Latest && len(args) < 1:
				return errors.New("specify at least one pod name or ID to log")
			}
			return nil
		},
		RunE:              logs,
		ValidArgsFunction: common.AutocompletePods,
		Example: `podman pod logs podID
podman pod logs -c ctrname podName
podman pod logs --tail 2 mywebserver
podman pod logs --follow=true --since 10m podID
podman pod logs mywebserver`,
	}
)

func init() {
	// pod logs
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: podLogsCommand,
		Parent:  podCmd,
	})
	logsFlags(podLogsCommand)
	validate.AddLatestFlag(podLogsCommand, &logsPodOptions.Latest)
}

func logsFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.BoolVar(&logsPodOptions.Details, "details", false, "Show extra details provided to the logs")
	flags.BoolVarP(&logsPodOptions.Follow, "follow", "f", false, "Follow log output.")

	containerNameFlag := "container"
	flags.StringVarP(&logsPodOptions.ContainerName, containerNameFlag, "c", "", "Filter logs by container name or id which belongs to pod")
	_ = cmd.RegisterFlagCompletionFunc(containerNameFlag, common.AutocompleteContainers)

	sinceFlagName := "since"
	flags.StringVar(&logsPodOptions.SinceRaw, sinceFlagName, "", "Show logs since TIMESTAMP")
	_ = cmd.RegisterFlagCompletionFunc(sinceFlagName, completion.AutocompleteNone)

	untilFlagName := "until"
	flags.StringVar(&logsPodOptions.UntilRaw, untilFlagName, "", "Show logs until TIMESTAMP")
	_ = cmd.RegisterFlagCompletionFunc(untilFlagName, completion.AutocompleteNone)

	tailFlagName := "tail"
	flags.Int64Var(&logsPodOptions.Tail, tailFlagName, -1, "Output the specified number of LINES at the end of the logs.")
	_ = cmd.RegisterFlagCompletionFunc(tailFlagName, completion.AutocompleteNone)

	flags.BoolVarP(&logsPodOptions.Names, "names", "n", false, "Output container names instead of container IDs in the log")
	flags.BoolVarP(&logsPodOptions.Timestamps, "timestamps", "t", false, "Output the timestamps in the log")
	flags.BoolVarP(&logsPodOptions.Colors, "color", "", false, "Output the containers within a pod with different colors in the log")

	_ = flags.MarkHidden("details")
}

func logs(_ *cobra.Command, args []string) error {
	if logsPodOptions.SinceRaw != "" {
		// parse time, error out if something is wrong
		since, err := util.ParseInputTime(logsPodOptions.SinceRaw, true)
		if err != nil {
			return fmt.Errorf("parsing --since %q: %w", logsPodOptions.SinceRaw, err)
		}
		logsPodOptions.Since = since
	}
	if logsPodOptions.UntilRaw != "" {
		// parse time, error out if something is wrong
		until, err := util.ParseInputTime(logsPodOptions.UntilRaw, false)
		if err != nil {
			return fmt.Errorf("parsing --until %q: %w", logsPodOptions.UntilRaw, err)
		}
		logsPodOptions.Until = until
	}

	// Remote can only process one container at a time
	if registry.IsRemote() && logsPodOptions.ContainerName == "" {
		return fmt.Errorf("-c or --container cannot be empty: %w", define.ErrInvalidArg)
	}

	logsPodOptions.StdoutWriter = os.Stdout
	logsPodOptions.StderrWriter = os.Stderr

	podName := ""
	if len(args) > 0 {
		podName = args[0]
	}
	return registry.ContainerEngine().PodLogs(registry.Context(), podName, logsPodOptions.PodLogsOptions)
}
