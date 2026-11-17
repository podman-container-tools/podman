package shim

import (
	"fmt"
	"os/exec"
	"syscall"

	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/env"
	sc "go.podman.io/podman/v6/pkg/machine/sockets"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"golang.org/x/sys/windows"
)

func setGvproxyProcessAttributes(c *exec.Cmd) {
	// Set SysProcAttr DETACHED_PROCESS or the gvproxy process may be killed
	// when the parent window is closed.
	// This should not happen because gvproxy is built as a Windows GUI application
	// and doesn't inherit the parent console. But a console version of gvproxy is
	// also available, and using DETACHED_PROCESS makes sure that the behavior
	// is the same nevertheless.
	c.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS,
	}
}

func cleanupStaleHostForwarder(mc *vmconfigs.MachineConfig, provider vmconfigs.VMProvider) error {
	if provider.VMType() == define.WSLVirt {
		if err := machine.CleanupStaleWinProxy(mc.Name, provider.VMType()); err != nil {
			return fmt.Errorf("could not recover api proxy for %s: %w", env.WithPodmanPrefix(mc.Name), err)
		}
		return nil
	}
	if provider.UseProviderNetworkSetup() {
		return nil
	}

	dirs, err := env.GetMachineDirs(provider.VMType())
	if err != nil {
		return err
	}
	pidFile, err := dirs.RuntimeDir.AppendToNewVMFile("gvproxy.pid", nil)
	if err != nil {
		return err
	}

	pipeName := env.WithPodmanPrefix(mc.Name)
	if err := machine.CleanupStaleGVProxy(pipeName, *pidFile); err != nil {
		return fmt.Errorf("could not recover api proxy for %s: %w", pipeName, err)
	}
	return nil
}

func setupMachineSockets(mc *vmconfigs.MachineConfig, _ *define.MachineDirs) ([]string, string, machine.APIForwardingState, error) {
	machinePipe := env.WithPodmanPrefix(mc.Name)
	if !machine.PipeNameAvailable(machinePipe, machine.MachineNameWait) {
		return nil, "", 0, fmt.Errorf("could not start api proxy since expected pipe is not available: %s", machinePipe)
	}
	sockets := []string{machine.NamedPipePrefix + machinePipe}
	state := machine.MachineLocal

	if machine.PipeNameAvailable(machine.GlobalNamedPipe, machine.GlobalNameWait) {
		sockets = append(sockets, machine.NamedPipePrefix+machine.GlobalNamedPipe)
		state = machine.DockerGlobal
	}

	hostSocket, err := mc.APISocket()
	if err != nil {
		return nil, "", 0, err
	}

	hostURL, err := sc.ToUnixURL(hostSocket)
	if err != nil {
		return nil, "", 0, err
	}
	sockets = append(sockets, hostURL.String())

	return sockets, sockets[len(sockets)-2], state, nil
}
