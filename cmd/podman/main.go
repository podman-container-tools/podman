package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	_ "go.podman.io/podman/v6/cmd/podman/artifact"
	_ "go.podman.io/podman/v6/cmd/podman/completion"
	_ "go.podman.io/podman/v6/cmd/podman/farm"
	_ "go.podman.io/podman/v6/cmd/podman/generate"
	_ "go.podman.io/podman/v6/cmd/podman/healthcheck"
	_ "go.podman.io/podman/v6/cmd/podman/images"
	_ "go.podman.io/podman/v6/cmd/podman/kube"
	_ "go.podman.io/podman/v6/cmd/podman/machine"
	_ "go.podman.io/podman/v6/cmd/podman/machine/os"
	_ "go.podman.io/podman/v6/cmd/podman/manifest"
	_ "go.podman.io/podman/v6/cmd/podman/networks"
	_ "go.podman.io/podman/v6/cmd/podman/pods"
	_ "go.podman.io/podman/v6/cmd/podman/quadlet"
	"go.podman.io/podman/v6/cmd/podman/registry"
	_ "go.podman.io/podman/v6/cmd/podman/secrets"
	_ "go.podman.io/podman/v6/cmd/podman/system"
	_ "go.podman.io/podman/v6/cmd/podman/system/connection"
	"go.podman.io/podman/v6/cmd/podman/validate"
	_ "go.podman.io/podman/v6/cmd/podman/volumes"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/logiface"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/terminal"
	"go.podman.io/storage/pkg/reexec"
	"golang.org/x/term"
)

type logrusLogger struct{}

func (l logrusLogger) Errorf(format string, args ...any) {
	logrus.Errorf(format, args...)
}

func (l logrusLogger) Debugf(format string, args ...any) {
	logrus.Debugf(format, args...)
}

func main() {
	if reexec.Init() {
		// We were invoked with a different argv[0] indicating that we
		// had a specific job to do as a subprocess, and it's done.
		return
	}
	logiface.SetLogger(logrusLogger{})

	if filepath.Base(os.Args[0]) == registry.PodmanSh ||
		(len(os.Args[0]) > 0 && filepath.Base(os.Args[0][1:]) == registry.PodmanSh) {
		shell := strings.TrimPrefix(os.Args[0], "-")
		cfg := registry.PodmanConfig()

		args := []string{shell, "exec", "-i", "--wait", strconv.FormatUint(uint64(cfg.ContainersConfDefaultsRO.PodmanshTimeout()), 10)}
		if term.IsTerminal(0) || term.IsTerminal(1) || term.IsTerminal(2) {
			args = append(args, "-t")
		}
		args = append(args, cfg.ContainersConfDefaultsRO.Podmansh.Container, cfg.ContainersConfDefaultsRO.Podmansh.Shell)
		if len(os.Args) > 1 {
			args = append(args, os.Args[1:]...)
		}
		os.Args = args
	}

	rootCmd = parseCommands()

	Execute()
	os.Exit(0)
}

func parseCommands() *cobra.Command {
	cfg := registry.PodmanConfig()
	for _, c := range registry.Commands {
		if supported, found := c.Command.Annotations[registry.EngineMode]; found {
			if cfg.EngineMode.String() != supported {
				var client string
				switch cfg.EngineMode {
				case entities.TunnelMode:
					client = "remote"
				case entities.ABIMode:
					client = "local"
				}

				// add error message to the command so the user knows that this command is not supported with local/remote
				c.Command.RunE = func(cmd *cobra.Command, _ []string) error {
					return fmt.Errorf("cannot use command %q with the %s podman client", cmd.CommandPath(), client)
				}
				// turn off flag parsing to make we do not get flag errors
				c.Command.DisableFlagParsing = true

				// mark command as hidden so it is not shown in --help
				c.Command.Hidden = true

				// overwrite persistent pre/post function to skip setup
				c.Command.PersistentPostRunE = validate.NoOp
				c.Command.PersistentPreRunE = validate.NoOp
				addCommand(c)
				continue
			}
		}

		// Command cannot be run rootless
		_, found := c.Command.Annotations[registry.UnshareNSRequired]
		if found {
			if rootless.IsRootless() && os.Getuid() != 0 {
				c.Command.RunE = func(cmd *cobra.Command, _ []string) error {
					return fmt.Errorf("cannot run command %q in rootless mode, must execute `podman unshare` first", cmd.CommandPath())
				}
			}
		}
		addCommand(c)
	}

	if err := terminal.SetConsole(); err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	rootCmd.SetFlagErrorFunc(flagErrorFuncfunc)
	return rootCmd
}

func flagErrorFuncfunc(c *cobra.Command, e error) error {
	e = fmt.Errorf("%w\nSee '%s --help'", e, c.CommandPath())
	return e
}

func addCommand(c registry.CliCommand) {
	parent := rootCmd
	if c.Parent != nil {
		parent = c.Parent
	}
	parent.AddCommand(c.Command)

	c.Command.SetFlagErrorFunc(flagErrorFuncfunc)

	// - templates need to be set here, as PersistentPreRunE() is
	// not called when --help is used.
	// - rootCmd uses cobra default template not ours
	c.Command.SetHelpTemplate(helpTemplate)
	c.Command.SetUsageTemplate(usageTemplate)
	c.Command.DisableFlagsInUseLine = true
}
