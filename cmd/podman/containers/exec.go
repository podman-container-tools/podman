package containers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	envLib "go.podman.io/podman/v6/pkg/env"
	"go.podman.io/podman/v6/pkg/rootless"
)

var (
	execDescription = `Execute the specified command inside a running container.
`
	execCommand = &cobra.Command{
		Use:               "exec [options] CONTAINER COMMAND [ARG...]",
		Short:             "Run a process in a running container",
		Long:              execDescription,
		RunE:              exec,
		ValidArgsFunction: common.AutocompleteExecCommand,
		Example: `podman exec -it ctrID ls
podman exec -it -w /tmp myCtr pwd
podman exec --user root ctrID ls`,
	}

	containerExecCommand = &cobra.Command{
		Use:               execCommand.Use,
		Short:             execCommand.Short,
		Long:              execCommand.Long,
		RunE:              execCommand.RunE,
		ValidArgsFunction: execCommand.ValidArgsFunction,
		Example: `podman container exec -it ctrID ls
podman container exec -it -w /tmp myCtr pwd
podman container exec --user root ctrID ls`,
	}
)

var (
	envInput, envFile []string
	execOpts          entities.ExecOptions
	execDetach        bool
	execCidFile       string
	execNoSession     bool
)

func execFlags(cmd *cobra.Command) {
	podmanConfig := registry.PodmanConfig()
	flags := cmd.Flags()

	flags.SetInterspersed(false)
	flags.BoolVarP(&execDetach, "detach", "d", false, "Run the exec session in detached mode (backgrounded)")

	detachKeysFlagName := "detach-keys"
	flags.StringVar(&execOpts.DetachKeys, detachKeysFlagName, containerConfig.DetachKeys(), "Select the key sequence for detaching a container. Format is a single character [a-Z] or ctrl-<value> where <value> is one of: a-z, @, ^, [, , or _")
	_ = cmd.RegisterFlagCompletionFunc(detachKeysFlagName, common.AutocompleteDetachKeys)

	cidfileFlagName := "cidfile"
	flags.StringVar(&execCidFile, cidfileFlagName, "", "File to read the container ID from")
	_ = cmd.RegisterFlagCompletionFunc(cidfileFlagName, completion.AutocompleteDefault)

	envFlagName := "env"
	flags.StringArrayVarP(&envInput, envFlagName, "e", []string{}, "Set environment variables")
	_ = cmd.RegisterFlagCompletionFunc(envFlagName, completion.AutocompleteNone)

	envFileFlagName := "env-file"
	flags.StringArrayVar(&envFile, envFileFlagName, []string{}, "Read in a file of environment variables")
	_ = cmd.RegisterFlagCompletionFunc(envFileFlagName, completion.AutocompleteDefault)

	flags.BoolVarP(&execOpts.Interactive, "interactive", "i", false, "Make STDIN available to the contained process")
	flags.BoolVar(&execOpts.Privileged, "privileged", podmanConfig.ContainersConfDefaultsRO.Containers.Privileged, "Give the process extended Linux capabilities inside the container.  The default is false")
	flags.BoolVarP(&execOpts.Tty, "tty", "t", false, "Allocate a pseudo-TTY. The default is false")

	userFlagName := "user"
	flags.StringVarP(&execOpts.User, userFlagName, "u", "", "Sets the username or UID used and optionally the groupname or GID for the specified command")
	_ = cmd.RegisterFlagCompletionFunc(userFlagName, common.AutocompleteUserFlag)

	preserveFdsFlagName := "preserve-fds"
	flags.UintVar(&execOpts.PreserveFDs, preserveFdsFlagName, 0, "Pass N additional file descriptors to the container")
	_ = cmd.RegisterFlagCompletionFunc(preserveFdsFlagName, completion.AutocompleteNone)

	preserveFdFlagName := "preserve-fd"
	flags.UintSliceVar(&execOpts.PreserveFD, preserveFdFlagName, nil, "Pass a list of additional file descriptors to the container")
	_ = cmd.RegisterFlagCompletionFunc(preserveFdFlagName, completion.AutocompleteNone)

	workdirFlagName := "workdir"
	flags.StringVarP(&execOpts.WorkDir, workdirFlagName, "w", "", "Working directory inside the container")
	_ = cmd.RegisterFlagCompletionFunc(workdirFlagName, completion.AutocompleteDefault)

	waitFlagName := "wait"
	flags.Int32(waitFlagName, 0, "Total seconds to wait for container to start")
	_ = flags.MarkHidden(waitFlagName)

	if !registry.IsRemote() {
		flags.BoolVar(&execNoSession, "no-session", false, "Do not create a database session for the exec process")
	}

	if registry.IsRemote() {
		_ = flags.MarkHidden("preserve-fds")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: execCommand,
	})
	execFlags(execCommand)
	validate.AddLatestFlag(execCommand, &execOpts.Latest)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerExecCommand,
		Parent:  containerCmd,
	})
	execFlags(containerExecCommand)
	validate.AddLatestFlag(containerExecCommand, &execOpts.Latest)
}

func exec(cmd *cobra.Command, args []string) error {
	if execNoSession {
		if execDetach || cmd.Flags().Changed("detach-keys") {
			return errors.New("--no-session cannot be used with --detach or --detach-keys")
		}
	}

	nameOrID, command, err := determineTargetCtrAndCmd(args, execOpts.Latest, execCidFile != "")
	if err != nil {
		return err
	}
	execOpts.Cmd = command

	// Validate given environment variables
	execOpts.Envs = make(map[string]string)
	for _, f := range envFile {
		fileEnv, err := envLib.ParseFile(f)
		if err != nil {
			return err
		}
		execOpts.Envs = envLib.Join(execOpts.Envs, fileEnv)
	}

	cliEnv, err := envLib.ParseSlice(envInput)
	if err != nil {
		return fmt.Errorf("parsing environment variables: %w", err)
	}

	execOpts.Envs = envLib.Join(execOpts.Envs, cliEnv)

	for _, fd := range execOpts.PreserveFD {
		if !rootless.IsFdInherited(int(fd)) {
			return fmt.Errorf("file descriptor %d is not available - the preserve-fd option requires that file descriptors must be passed", fd)
		}
	}

	for fd := 3; fd < int(3+execOpts.PreserveFDs); fd++ {
		if !rootless.IsFdInherited(fd) {
			return fmt.Errorf("file descriptor %d is not available - the preserve-fds option requires that file descriptors must be passed", fd)
		}
	}

	if cmd.Flags().Changed("wait") {
		seconds, err := cmd.Flags().GetInt32("wait")
		if err != nil {
			return err
		}
		if err := execWait(nameOrID, seconds); err != nil {
			if errors.Is(err, define.ErrCanceled) {
				return fmt.Errorf("timed out waiting for container: %s", nameOrID)
			}
		}
	}

	streams := define.AttachStreams{}
	streams.OutputStream = os.Stdout
	streams.ErrorStream = os.Stderr
	if execOpts.Interactive {
		streams.InputStream = bufio.NewReader(os.Stdin)
		streams.AttachInput = true
	}
	streams.AttachOutput = true
	streams.AttachError = true

	if execNoSession {
		exitCode, err := registry.ContainerEngine().ContainerExecNoSession(registry.Context(), nameOrID, execOpts, streams)
		registry.SetExitCode(exitCode)
		return err
	} else if !execDetach {
		exitCode, err := registry.ContainerEngine().ContainerExec(registry.Context(), nameOrID, execOpts, streams)
		registry.SetExitCode(exitCode)
		return err
	}

	id, err := registry.ContainerEngine().ContainerExecDetached(registry.Context(), nameOrID, execOpts)
	if err != nil {
		return err
	}
	fmt.Println(id)
	return nil
}

// determineTargetCtrAndCmd determines which command exec should run in which container
func determineTargetCtrAndCmd(args []string, latestSpecified bool, execCidFileProvided bool) (string, []string, error) {
	var nameOrID string
	var command []string

	if len(args) == 0 && !latestSpecified && !execCidFileProvided {
		return "", nil, errors.New("exec requires the name or ID of a container or the --latest or --cidfile flag")
	} else if latestSpecified && execCidFileProvided {
		return "", nil, errors.New("--latest and --cidfile can not be used together")
	}
	command = args
	if !latestSpecified {
		if !execCidFileProvided {
			// assume first arg to be name or ID
			command = args[1:]
			nameOrID = strings.TrimPrefix(args[0], "/")
		} else {
			content, err := os.ReadFile(execCidFile)
			if err != nil {
				return "", nil, fmt.Errorf("reading CIDFile: %w", err)
			}
			nameOrID, _, _ = strings.Cut(string(content), "\n")
		}
	}
	return nameOrID, command, nil
}

func execWait(ctr string, seconds int32) error {
	maxDuration := time.Duration(seconds) * time.Second
	interval := 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(registry.Context(), maxDuration)
	defer cancel()

	waitOptions.Conditions = []string{define.ContainerStateRunning.String()}

	startTime := time.Now()
	for time.Since(startTime) < maxDuration {
		_, err := registry.ContainerEngine().ContainerWait(ctx, []string{ctr}, waitOptions)
		if err == nil {
			return nil
		}

		if !errors.Is(err, define.ErrNoSuchCtr) {
			return err
		}

		interval *= 2
		since := time.Since(startTime)
		if since+interval > maxDuration {
			interval = maxDuration - since
			if interval <= 0 {
				break
			}
		}
		time.Sleep(interval)
	}
	return define.ErrCanceled
}
