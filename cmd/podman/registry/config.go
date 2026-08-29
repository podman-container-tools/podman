package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/podman/v6/pkg/rootless"
	"go.podman.io/podman/v6/pkg/util"
	"go.podman.io/storage/pkg/fileutils"
)

const (
	// NoMoveProcess used as cobra.Annotation when command doesn't need Podman to be moved to a separate cgroup
	NoMoveProcess = "NoMoveProcess"

	// ParentNSRequired used as cobra.Annotation when a command should not be run in the podman rootless user namespace, also requires updates in `pkg/rootless/rootless_linux.c` in function `can_use_shortcut()` to exclude the command name there.
	ParentNSRequired = "ParentNSRequired"

	// UnshareNSRequired used as cobra.Annotation when command requires modified user namespace
	UnshareNSRequired = "UnshareNSRequired"

	// EngineMode used as cobra.Annotation when command supports a limited number of Engines
	EngineMode = "EngineMode"
)

var (
	podmanOptions entities.PodmanConfig
	podmanSync    sync.Once
	abiSupport    = false

	// ABIMode used in cobra.Annotations registry.EngineMode when command only supports ABIMode
	ABIMode = entities.ABIMode.String()
	// TunnelMode used in cobra.Annotations registry.EngineMode when command only supports TunnelMode
	TunnelMode = entities.TunnelMode.String()
)

type earlyCLIOptions struct {
	completion bool
	debug      bool
	logLevel   string
	modules    []string
	parseErr   error
	remote     bool
}

// PodmanConfig returns an entities.PodmanConfig built up from
// environment and CLI.
func PodmanConfig() *entities.PodmanConfig {
	podmanSync.Do(newPodmanConfig)
	return &podmanOptions
}

// Return the index of args where to start parsing CLI flags.
// An index > 1 implies Podman is running in shell completion.
func parseIndex(args []string) int {
	// The shell completion logic will call a command called "__complete" or "__completeNoDesc"
	// This command will always be the second argument
	// To still parse --remote correctly in this case we have to set args offset to two in this case
	if len(args) > 1 && (args[1] == cobra.ShellCompRequestCmd || args[1] == cobra.ShellCompNoDescRequestCmd) {
		return 2
	}
	return 1
}

// parseEarlyCLIOptions parses flags needed during command initialization.
// Cobra parses and validates the complete command line later.
func parseEarlyCLIOptions(args []string) *earlyCLIOptions {
	options := new(earlyCLIOptions)
	index := parseIndex(args)
	options.completion = index > 1
	if _, found := os.LookupEnv("CONTAINER_HOST"); found {
		options.remote = true
	} else if _, found := os.LookupEnv("CONTAINER_CONNECTION"); found {
		options.remote = true
	}

	fs := pflag.NewFlagSet("early podman flags", pflag.ContinueOnError)
	fs.ParseErrorsAllowlist.UnknownFlags = true
	fs.Usage = func() {}
	fs.SetInterspersed(false)
	fs.StringArrayVar(&options.modules, "module", nil, "")
	fs.StringVar(&options.logLevel, "log-level", "", "")
	fs.BoolVarP(&options.debug, "debug", "D", false, "")
	fs.BoolVarP(&options.remote, "remote", "r", options.remote, "")
	connectionFlagName := "connection"
	fs.StringP(connectionFlagName, "c", "", "")
	contextFlagName := "context"
	fs.String(contextFlagName, "", "")
	hostFlagName := "host"
	fs.StringP(hostFlagName, "H", "", "")
	urlFlagName := "url"
	fs.String(urlFlagName, "", "")
	fs.BoolP("help", "h", false, "") // Need a fake help flag to avoid the `pflag: help requested` error

	options.parseErr = fs.Parse(args[index:])
	// --connection, --context, --host, or --url implies --remote.
	options.remote = options.remote || fs.Changed(connectionFlagName) || fs.Changed(contextFlagName) || fs.Changed(hostFlagName) || fs.Changed(urlFlagName)
	return options
}

// Set the log level before containers.conf is loaded.
func setEarlyLogLevel(options *earlyCLIOptions) {
	if options.completion {
		return
	}

	if options.debug && options.logLevel == "" {
		logrus.SetLevel(logrus.DebugLevel)
	} else if !options.debug && options.logLevel != "" {
		if level, err := logrus.ParseLevel(options.logLevel); err == nil {
			logrus.SetLevel(level)
		}
	}
}

func newPodmanConfig() {
	options := parseEarlyCLIOptions(os.Args)
	setEarlyLogLevel(options)
	if options.parseErr != nil && !options.completion {
		fmt.Fprintf(os.Stderr, "Error parsing command-line flags: %v\n", options.parseErr)
		os.Exit(1)
	}
	modules := options.modules
	if options.completion {
		// Do not load the modules during shell completion.
		modules = nil
	}

	if err := setXdgDirs(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	defaultConfig, err := config.New(&config.Options{
		SetDefault: true, // This makes sure that following calls to config.Default() return this config
		Modules:    modules,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to obtain podman configuration: %v\n", err)
		os.Exit(1)
	}

	var mode entities.EngineMode
	remote := options.remote && !isPodmanSh(os.Args)
	switch runtime.GOOS {
	case "darwin", "windows":
		mode = entities.TunnelMode
	case "linux", "freebsd":
		// Some linux clients might only be compiled without ABI
		// support (e.g., podman-remote).
		if abiSupport && !remote {
			mode = entities.ABIMode
		} else {
			mode = entities.TunnelMode
		}
	default:
		fmt.Fprintf(os.Stderr, "%s is not a supported OS\n", runtime.GOOS)
		os.Exit(1)
	}

	// If EngineMode==Tunnel has not been set on the command line or environment
	// but has been set in containers.conf...
	if mode == entities.ABIMode && defaultConfig.Engine.Remote {
		mode = entities.TunnelMode
	}

	podmanOptions = entities.PodmanConfig{ContainersConf: &config.Config{}, ContainersConfDefaultsRO: defaultConfig, EngineMode: mode}
}

// setXdgDirs ensures the XDG_RUNTIME_DIR env and XDG_CONFIG_HOME variables are set.
// containers/image uses XDG_RUNTIME_DIR to locate the auth file, XDG_CONFIG_HOME is
// use for the containers.conf configuration file.
func setXdgDirs() error {
	if !rootless.IsRootless() {
		return nil
	}

	// Set up XDG_RUNTIME_DIR
	if _, found := os.LookupEnv("XDG_RUNTIME_DIR"); !found {
		dir, err := util.GetRootlessRuntimeDir()
		if err != nil {
			return err
		}
		if err := os.Setenv("XDG_RUNTIME_DIR", dir); err != nil {
			return fmt.Errorf("cannot set XDG_RUNTIME_DIR=%s: %w", dir, err)
		}
	}

	if _, found := os.LookupEnv("DBUS_SESSION_BUS_ADDRESS"); !found {
		sessionAddr := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "bus")
		if err := fileutils.Exists(sessionAddr); err == nil {
			sessionAddr, err = filepath.EvalSymlinks(sessionAddr)
			if err != nil {
				return err
			}
			os.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+sessionAddr)
		}
	}

	// Set up XDG_CONFIG_HOME
	if _, found := os.LookupEnv("XDG_CONFIG_HOME"); !found {
		cfgHomeDir, err := util.GetRootlessConfigHomeDir()
		if err != nil {
			return err
		}
		if err := os.Setenv("XDG_CONFIG_HOME", cfgHomeDir); err != nil {
			return fmt.Errorf("cannot set XDG_CONFIG_HOME=%s: %w", cfgHomeDir, err)
		}
	}
	return nil
}

func RetryDefault() uint {
	if IsRemote() {
		return 0
	}

	return PodmanConfig().ContainersConfDefaultsRO.Engine.Retry
}

func RetryDelayDefault() string {
	if IsRemote() {
		return ""
	}

	return PodmanConfig().ContainersConfDefaultsRO.Engine.RetryDelay
}
