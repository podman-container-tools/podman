package containers

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/common/pkg/auth"
	"go.podman.io/common/pkg/completion"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/utils"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/specgen"
	"go.podman.io/podman/v6/pkg/specgenutil"
	"golang.org/x/term"
)

var (
	runDescription = "Runs a command in a new container from the given image"
	runCommand     = &cobra.Command{
		Args:              cobra.MinimumNArgs(1),
		Use:               "run [options] IMAGE [COMMAND [ARG...]]",
		Short:             "Run a command in a new container",
		Long:              runDescription,
		RunE:              run,
		ValidArgsFunction: common.AutocompleteCreateRun,
		Example: `podman run imageID ls -alF /etc
podman run --network=host imageID dnf -y install java
podman run --volume /var/hostdir:/var/ctrdir -i -t fedora /bin/bash`,
	}

	containerRunCommand = &cobra.Command{
		Args:              cobra.MinimumNArgs(1),
		Use:               runCommand.Use,
		Short:             runCommand.Short,
		Long:              runCommand.Long,
		RunE:              runCommand.RunE,
		ValidArgsFunction: runCommand.ValidArgsFunction,
		Example: `podman container run imageID ls -alF /etc
podman container run --network=host imageID dnf -y install java
podman container run --volume /var/hostdir:/var/ctrdir -i -t fedora /bin/bash`,
	}
)

var (
	runOpts entities.ContainerRunOptions
	runRmi  bool
)

func runFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	flags.SetInterspersed(false)
	common.DefineCreateDefaults(&cliVals)
	common.DefineCreateFlags(cmd, &cliVals, entities.CreateMode)
	common.DefineNetFlags(cmd)

	flags.SetNormalizeFunc(utils.AliasFlags)
	flags.BoolVar(&runOpts.SigProxy, "sig-proxy", true, "Proxy received signals to the process")
	flags.BoolVar(&runRmi, "rmi", false, "Remove image unless used by other containers, implies --rm")

	preserveFdsFlagName := "preserve-fds"
	flags.UintVar(&runOpts.PreserveFDs, preserveFdsFlagName, 0, "Pass a number of additional file descriptors into the container")
	_ = cmd.RegisterFlagCompletionFunc(preserveFdsFlagName, completion.AutocompleteNone)

	preserveFdFlagName := "preserve-fd"
	flags.UintSliceVar(&runOpts.PreserveFD, preserveFdFlagName, nil, "Pass a file descriptor into the container")
	_ = cmd.RegisterFlagCompletionFunc(preserveFdFlagName, completion.AutocompleteNone)

	flags.BoolVarP(&runOpts.Detach, "detach", "d", false, "Run container in background and print container ID")

	detachKeysFlagName := "detach-keys"
	flags.StringVar(&runOpts.DetachKeys, detachKeysFlagName, containerConfig.DetachKeys(), "Override the key sequence for detaching a container. Format is a single character `[a-Z]` or a comma separated sequence of `ctrl-<value>`, where `<value>` is one of: `a-cf`, `@`, `^`, `[`, `\\`, `]`, `^` or `_`")
	_ = cmd.RegisterFlagCompletionFunc(detachKeysFlagName, common.AutocompleteDetachKeys)

	passwdFlagName := "passwd"
	flags.BoolVar(&runOpts.Passwd, passwdFlagName, true, "add entries to /etc/passwd and /etc/group")

	if registry.IsRemote() {
		_ = flags.MarkHidden(preserveFdsFlagName)
		_ = flags.MarkHidden(preserveFdFlagName)
		_ = flags.MarkHidden("conmon-pidfile")
		_ = flags.MarkHidden("pidfile")
	}
}

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: runCommand,
	})

	runFlags(runCommand)

	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: containerRunCommand,
		Parent:  containerCmd,
	})

	runFlags(containerRunCommand)
}

func run(cmd *cobra.Command, args []string) error {
	if err := commonFlags(cmd); err != nil {
		return err
	}

	if runRmi {
		if cmd.Flags().Changed("rm") && !cliVals.Rm {
			return errors.New("the --rmi option does not work without --rm")
		}
		cliVals.Rm = true
	}

	if cliVals.TTY && cliVals.Interactive && !runOpts.Detach && !term.IsTerminal(int(os.Stdin.Fd())) {
		logrus.Warnf("The input device is not a TTY. The --tty and --interactive flags might not work properly")
	}

	if cmd.Flags().Changed("authfile") {
		if err := auth.CheckAuthFile(cliVals.Authfile); err != nil {
			return err
		}
	}

	runOpts.CIDFile = cliVals.CIDFile
	runOpts.Rm = cliVals.Rm
	cliVals, err := CreateInit(cmd, cliVals, false)
	if err != nil {
		return err
	}

	for _, fd := range runOpts.PreserveFD {
		if !rootless.IsFdInherited(int(fd)) {
			return fmt.Errorf("file descriptor %d is not available - the preserve-fd option requires that file descriptors must be passed", fd)
		}
	}
	for fd := 3; fd < int(3+runOpts.PreserveFDs); fd++ {
		if !rootless.IsFdInherited(fd) {
			return fmt.Errorf("file descriptor %d is not available - the preserve-fds option requires that file descriptors must be passed", fd)
		}
	}

	imageName := args[0]
	rawImageName := ""
	if !cliVals.RootFS {
		rawImageName = args[0]
		name, err := pullImage(cmd, args[0], &cliVals)
		if err != nil {
			return err
		}
		imageName = name
	}

	if cliVals.Replace {
		if err := replaceContainer(cliVals.Name); err != nil {
			return err
		}
	}

	// First set the default streams before they get modified by any flags.
	runOpts.OutputStream = os.Stdout
	runOpts.InputStream = os.Stdin
	runOpts.ErrorStream = os.Stderr

	// If -i is not set, clear stdin
	if !cliVals.Interactive {
		runOpts.InputStream = nil
	}

	passthrough := cliVals.LogDriver == define.PassthroughLogging || cliVals.LogDriver == define.PassthroughTTYLogging

	// If attach is set, clear stdin/stdout/stderr and only attach requested
	if cmd.Flag("attach").Changed {
		if passthrough {
			return fmt.Errorf("cannot specify --attach with --log-driver=passthrough: %w", define.ErrInvalidArg)
		}
		runOpts.OutputStream = nil
		runOpts.ErrorStream = nil
		if !cliVals.Interactive {
			runOpts.InputStream = nil
		}

		for _, stream := range cliVals.Attach {
			switch strings.ToLower(stream) {
			case "stdout":
				runOpts.OutputStream = os.Stdout
			case "stderr":
				runOpts.ErrorStream = os.Stderr
			case "stdin":
				runOpts.InputStream = os.Stdin
			default:
				return fmt.Errorf("invalid stream %q for --attach - must be one of stdin, stdout, or stderr: %w", stream, define.ErrInvalidArg)
			}
		}
	}

	cliVals.PreserveFDs = runOpts.PreserveFDs
	cliVals.PreserveFD = runOpts.PreserveFD
	s := specgen.NewSpecGenerator(imageName, cliVals.RootFS)
	if err := specgenutil.FillOutSpecGen(s, &cliVals, args); err != nil {
		return err
	}
	s.RawImageName = rawImageName

	// Include the command used to create the container.
	s.ContainerCreateCommand = os.Args

	s.ImageOS = cliVals.OS
	s.ImageArch = cliVals.Arch
	s.ImageVariant = cliVals.Variant
	s.Passwd = &runOpts.Passwd

	if runRmi {
		s.RemoveImage = &runRmi
	}

	runOpts.Spec = s

	if err := createPodIfNecessary(cmd, s, cliVals.Net); err != nil {
		return err
	}

	if s.HealthConfig == nil {
		s.HealthConfig, err = common.GetHealthCheckOverrideConfig(cmd, &cliVals)
		if err != nil {
			return err
		}
	}

	report, err := registry.ContainerEngine().ContainerRun(registry.Context(), runOpts)
	// report.ExitCode is set by ContainerRun even it returns an error
	if report != nil {
		registry.SetExitCode(report.ExitCode)
	}
	if err != nil {
		// if pod was created as part of run
		// remove it in case ctr creation fails
		if err := rmPodIfNecessary(cmd, s); err != nil {
			if !errors.Is(err, define.ErrNoSuchPod) {
				logrus.Error(err.Error())
			}
		}
		return err
	}

	if runOpts.Detach {
		if !passthrough {
			fmt.Println(report.Id)
		}
		return nil
	}
	if runRmi {
		_, rmErrors := registry.ImageEngine().Remove(registry.Context(), []string{imageName}, entities.ImageRemoveOptions{Ignore: true})
		for _, err := range rmErrors {
			logrus.Warnf("Failed to remove image: %v", err)
		}
	}
	return nil
}
