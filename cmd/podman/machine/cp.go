//go:build amd64 || arm64

package machine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/containers/podman/v5/cmd/podman/registry"
	"github.com/containers/podman/v5/libpod/events"
	"github.com/containers/podman/v5/pkg/copy"
	"github.com/containers/podman/v5/pkg/machine"
	"github.com/containers/podman/v5/pkg/machine/define"
	"github.com/containers/podman/v5/pkg/machine/env"
	"github.com/containers/podman/v5/pkg/machine/vmconfigs"
	"github.com/containers/podman/v5/pkg/specgen"
	"github.com/spf13/cobra"
)

type cpOptions struct {
	Quiet    bool
	Machine  *vmconfigs.MachineConfig
	IsSrc    bool
	SrcPath  string
	DestPath string
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

	mc, err := resolveMachine(srcMachine, destMachine)
	if err != nil {
		return err
	}

	state, err := provider.State(mc, false)
	if err != nil {
		return err
	}
	if state != define.Running {
		return fmt.Errorf("vm %q is not running", mc.Name)
	}

	cpOpts.Machine = mc
	cpOpts.SrcPath = srcPath
	cpOpts.DestPath = destPath

	err = localhostSSHCopy(&cpOpts)
	if err != nil {
		return fmt.Errorf("copy failed: %s", err.Error())
	}

	fmt.Println("Copy successful")
	newMachineEvent(events.Copy, events.Event{Name: mc.Name})
	return nil
}

// localhostSSHCopy uses scp to copy files from/to a localhost machine using ssh.
func localhostSSHCopy(opts *cpOptions) error {
	srcPath := opts.SrcPath
	destPath := opts.DestPath
	sshConfig := opts.Machine.SSH

	username := sshConfig.RemoteUsername
	if cpOpts.Machine.HostUser.Rootful {
		username = "root"
	}
	username += "@localhost:"

	if opts.IsSrc {
		srcPath = username + srcPath
	} else {
		destPath = username + destPath
	}

	sshArgs := machine.LocalhostSSHArgs()
	args := make([]string, 0, 7+len(sshArgs))
	args = append(args, "-r", "-i", sshConfig.IdentityPath, "-P", strconv.Itoa(sshConfig.Port))
	args = append(args, sshArgs...) // Warning: This MUST NOT be generalized to allow communication over untrusted networks.
	args = append(args, srcPath, destPath)

	cmd := exec.Command("scp", args...)
	if !opts.Quiet {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func resolveMachine(srcMachine, destMachine string) (*vmconfigs.MachineConfig, error) {
	if len(srcMachine) > 0 && len(destMachine) > 0 {
		return nil, errors.New("copying between two machines is unsupported")
	}

	if len(srcMachine) == 0 && len(destMachine) == 0 {
		return nil, errors.New("a machine name must prefix either the source path or destination path")
	}

	dirs, err := env.GetMachineDirs(provider.VMType())
	if err != nil {
		return nil, err
	}

	name := destMachine
	if len(srcMachine) > 0 {
		cpOpts.IsSrc = true
		name = srcMachine
	}

	return vmconfigs.LoadMachineByName(name, dirs)
}
