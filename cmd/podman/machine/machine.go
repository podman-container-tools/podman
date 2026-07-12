//go:build (amd64 && !darwin) || arm64

package machine

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/common"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/cmd/podman/validate"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/machine/env"
	provider2 "go.podman.io/podman/v6/pkg/machine/provider"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/podman/v6/pkg/util"
)

var (
	// Pull in configured json library
	json = registry.JSONLibrary()

	openEventSock sync.Once  // Singleton support for opening sockets as needed
	sockets       []net.Conn // Opened sockets, if any

	// Command: podman _machine_
	machineCmd = &cobra.Command{
		Use:                "machine",
		Short:              "Manage a virtual machine",
		Long:               "Manage a virtual machine. Virtual machines are used to run Podman.",
		PersistentPreRunE:  validate.NoOp,
		PersistentPostRunE: closeMachineEvents,
		RunE:               validate.SubCommandExists,
	}
)

var machineProvider vmconfigs.VMProvider

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: machineCmd,
	})
}

func machinePreRunE(c *cobra.Command, args []string) error {
	var err error
	machineProvider, err = provider2.Get()
	if err != nil {
		return err
	}
	return rootlessOnly(c, args)
}

// autocompleteMachineSSH - Autocomplete machine ssh command.
func autocompleteMachineSSH(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return getMachines(toComplete)
	}
	return nil, cobra.ShellCompDirectiveDefault
}

// autocompleteMachineCp - Autocomplete machine cp command.
func autocompleteMachineCp(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) < 2 {
		if found := strings.Contains(toComplete, ":"); found {
			// TODO: offer virtual machine path completion

			// the user already set the machine name, so don't use the host file autocompletion
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		// suggest machine when they match the input otherwise normal shell completion is used
		machines, _ := getMachines(toComplete)
		for _, machine := range machines {
			if strings.HasPrefix(machine, toComplete) {
				for i := range machines {
					machines[i] += ":"
				}
				return machines, cobra.ShellCompDirectiveNoSpace
			}
		}

		return nil, cobra.ShellCompDirectiveNoSpace
	}
	// don't complete more than 2 args
	return nil, cobra.ShellCompDirectiveNoFileComp
}

// AutocompleteMachine - Autocomplete machines.
func AutocompleteMachine(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if common.ValidCurrentCmdLine(cmd, args, toComplete) {
		return getMachines(toComplete)
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func autocompleteMachineProvider(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	providers := provider2.GetAll()
	suggestions := make([]string, 0, len(providers))
	for _, p := range providers {
		suggestions = append(suggestions, p.VMType().String())
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func getMachines(toComplete string) ([]string, cobra.ShellCompDirective) {
	suggestions := []string{}
	provider, err := provider2.Get()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	dirs, err := env.GetMachineDirs(provider.VMType())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	machines, err := vmconfigs.LoadMachinesInDir(dirs)
	if err != nil {
		cobra.CompErrorln(err.Error())
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	for _, m := range machines {
		if strings.HasPrefix(m.Name, toComplete) {
			suggestions = append(suggestions, m.Name)
		}
	}
	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

func initMachineEvents() {
	sockPaths, err := resolveEventSock()
	if err != nil {
		logrus.Warnf("Failed to resolve machine event sockets, machine events will not be published: %v", err)
	}

	for _, path := range sockPaths {
		conn, err := (&net.Dialer{}).DialContext(registry.Context(), "unix", path)
		if err != nil {
			logrus.Warnf("Failed to open event socket %q: %v", path, err)
			continue
		}
		logrus.Debugf("Machine event socket %q found", path)
		sockets = append(sockets, conn)
	}
}

func resolveEventSock() ([]string, error) {
	// Used mostly for testing
	if sock, found := os.LookupEnv("PODMAN_MACHINE_EVENTS_SOCK"); found {
		return []string{sock}, nil
	}

	re := regexp.MustCompile(`machine_events.*\.sock`)
	sockPaths := make([]string, 0)
	fn := func(path string, info os.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case info.IsDir():
			return nil
		case !isUnixSocket(info):
			return nil
		case !re.MatchString(info.Name()):
			return nil
		}

		logrus.Debugf("Machine events will be published on: %q", path)
		sockPaths = append(sockPaths, path)
		return nil
	}
	sockDir, err := eventSockDir()
	if err != nil {
		logrus.Warnf("Failed to get runtime dir, machine events will not be published: %s", err)
	}

	if err := filepath.WalkDir(sockDir, fn); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return sockPaths, nil
}

func eventSockDir() (string, error) {
	xdg, err := util.GetRootlessRuntimeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(xdg, "podman"), nil
}

func newMachineEvent(status events.Status, event events.Event) {
	openEventSock.Do(initMachineEvents)

	event.Status = status
	event.Time = time.Now()
	event.Type = events.Machine

	payload, err := json.Marshal(event)
	if err != nil {
		logrus.Errorf("Unable to format machine event: %q", err)
		return
	}

	for _, sock := range sockets {
		if _, err := sock.Write(payload); err != nil {
			logrus.Errorf("Unable to write machine event: %q", err)
		}
	}
}

func closeMachineEvents(cmd *cobra.Command, _ []string) error {
	logrus.Debugf("Called machine %s.PersistentPostRunE(%s)", cmd.Name(), strings.Join(os.Args, " "))
	for _, sock := range sockets {
		_ = sock.Close()
	}
	return nil
}
