package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

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

// PodmanConfig returns an entities.PodmanConfig built up from
// environment and CLI.
func PodmanConfig() *entities.PodmanConfig {
	podmanSync.Do(newPodmanConfig)
	return &podmanOptions
}

// Return the index of os.Args where to start parsing CLI flags.
// An index > 1 implies Podman is running in shell completion.
func parseIndex() int {
	// The shell completion logic will call a command called "__complete" or "__completeNoDesc"
	// This command will always be the second argument
	// To still parse --remote correctly in this case we have to set args offset to two in this case
	if len(os.Args) > 1 && (os.Args[1] == cobra.ShellCompRequestCmd || os.Args[1] == cobra.ShellCompNoDescRequestCmd) {
		return 2
	}
	return 1
}

// Return the containers.conf modules to load.
func containersConfModules() ([]string, error) {
	index := parseIndex()
	if index > 1 {
		// Do not load the modules during shell completion.
		return nil, nil
	}

	var modules []string
	fs := pflag.NewFlagSet("module", pflag.ContinueOnError)
	fs.ParseErrorsAllowlist.UnknownFlags = true
	fs.Usage = func() {}
	fs.SetInterspersed(false)
	fs.StringArrayVar(&modules, "module", nil, "")
	fs.BoolP("help", "h", false, "") // Need a fake help flag to avoid the `pflag: help requested` error
	return modules, fs.Parse(os.Args[index:])
}

func newPodmanConfig() {
	modules, err := containersConfModules()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing containers.conf modules: %v\n", err)
		os.Exit(1)
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
	switch runtime.GOOS {
	case "darwin", "windows":
		mode = entities.TunnelMode
	case "linux", "freebsd":
		// Some linux clients might only be compiled without ABI
		// support (e.g., podman-remote).
		if abiSupport && !IsRemote() {
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
