//go:build amd64 || arm64

package machine

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"go.podman.io/podman/v6/cmd/podman/registry"
	"go.podman.io/podman/v6/libpod/events"
	"go.podman.io/podman/v6/pkg/copy"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/shim"
	"go.podman.io/podman/v6/pkg/specgen"
)

type cpOptions struct {
	Quiet bool
	IsSrc bool
}

var (
	cpCmd = &cobra.Command{
		Use:               "cp [options] SRC_PATH DEST_PATH",
		Short:             "Securely copy contents between the virtual machine",
		Long:              "Securely copy files or directories between the virtual machine and your host",
		PersistentPreRunE: machinePreRunE,
		RunE:              cp,
		Args:              cobra.ExactArgs(2),
		Example:           `podman machine cp ~/ca.crt podman-machine-default:/etc/containers/certs.d/ca.crt`,
		ValidArgsFunction: autocompleteMachineCp,
	}

	cpOpts = cpOptions{}
)

func init() {
	registry.Commands = append(registry.Commands, registry.CliCommand{
		Command: cpCmd,
		Parent:  machineCmd,
	})

	flags := cpCmd.Flags()
	quietFlagName := "quiet"
	flags.BoolVarP(&cpOpts.Quiet, quietFlagName, "q", false, "Suppress copy status output")
}

func cp(_ *cobra.Command, args []string) error {
	var err error

	srcMachine, srcPath, destMachine, destPath, err := copy.ParseSourceAndDestination(args[0], args[1])
	if err != nil {
		return err
	}

	// NOTE: This will most likely break hyperv or wsl machines with single-letter
	// names. It is most likely similar to https://github.com/containers/podman/issues/25218
	//
	// Passing an absolute windows path of the format <volume>:\<path> will cause
	// `copy.ParseSourceAndDestination` to think the volume is a Machine. Check
	// if the raw cmdline argument is a Windows host path.
	if specgen.IsHostWinPath(args[0]) {
		srcMachine = ""
		srcPath = args[0]
	}

	if specgen.IsHostWinPath(args[1]) {
		destMachine = ""
		destPath = args[1]
	}

	vmName, err := resolveMachineName(srcMachine, destMachine)
	if err != nil {
		return err
	}
	mc, vmProvider, err := shim.VMExists(vmName)
	if err != nil {
		return err
	}

	state, err := vmProvider.State(mc, false)
	if err != nil {
		return err
	}
	if state != define.Running {
		return fmt.Errorf("vm %q is not running", mc.Name)
	}

	sshConfig := mc.SSH
	username := sshConfig.RemoteUsername
	if mc.HostUser.Rootful {
		username = "root"
	}
	err = machine.LocalhostSSHCopy(username,
		sshConfig.IdentityPath,
		sshConfig.Port,
		srcPath,
		destPath,
		cpOpts.IsSrc,
		cpOpts.Quiet)
	if err != nil {
		return fmt.Errorf("copy failed: %s", err.Error())
	}

	fmt.Println("Copy successful")
	newMachineEvent(events.Copy, events.Event{Name: mc.Name})
	return nil
}

func resolveMachineName(srcMachine, destMachine string) (string, error) {
	if len(srcMachine) > 0 && len(destMachine) > 0 {
		return "", errors.New("copying between two machines is unsupported")
	}

	if len(srcMachine) == 0 && len(destMachine) == 0 {
		return "", errors.New("a machine name must prefix either the source path or destination path")
	}
	name := destMachine
	if len(srcMachine) > 0 {
		cpOpts.IsSrc = true
		name = srcMachine
	}
	return name, nil
}
