//go:build windows

package wsl

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/podman/v6/pkg/machine/wsl/wutil"

	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/ignition"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/podman/v6/pkg/machine/windows"
)

type WSLStubber struct {
	vmconfigs.WSLConfig
}

func (w WSLStubber) CreateVM(opts define.CreateVMOpts, mc *vmconfigs.MachineConfig, _ *ignition.IgnitionBuilder) error {
	var err error
	// cleanup half-baked files if init fails at any point
	callbackFuncs := machine.CleanUp()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal(false)
	mc.WSLHypervisor = new(vmconfigs.WSLConfig)

	_ = setupWslProxyEnv()

	if opts.UserModeNetworking {
		if err = verifyWSLUserModeCompat(); err != nil {
			return err
		}
		mc.WSLHypervisor.UserModeNetworking = true
	}

	const prompt = "Importing operating system into WSL (this may take a few minutes on a new WSL install)..."
	dist, err := provisionWSLDist(mc.Name, mc.ImagePath.GetPath(), prompt)
	if err != nil {
		if errors.Is(err, ErrWslNotSupported) {
			// If error is Wsl/Service/RegisterDistro/CreateVm/HCS/ERROR_NOT_SUPPORTED
			// or Wsl/Service/RegisterDistro/CreateVm/HCS/HCS_E_SERVICE_NOT_AVAILABLE
			// it means WSL's VM creation failed, likely due to virtualization features not being enabled.
			// Relaunching 'podman machine init' in elevated mode will attempt to reconfigure the WSL machine.
			admin := windows.HasAdminRights()

			return attemptFeatureInstall(opts.ReExec, admin)
		}
		return err
	}

	unprovisionCallbackFunc := func() error {
		return unprovisionWSL(mc)
	}
	callbackFuncs.Add(unprovisionCallbackFunc)

	if mc.WSLHypervisor.UserModeNetworking {
		if err = installUserModeDist(dist, mc.ImagePath.GetPath()); err != nil {
			_ = unregisterDist(dist)
			return err
		}
	}

	fmt.Println("Configuring system...")
	if err = configureSystem(mc, dist, mc.Ansible); err != nil {
		return err
	}

	if err = installScripts(dist); err != nil {
		return err
	}

	if err = createKeys(mc, dist); err != nil {
		return err
	}

	// recycle vm
	return terminateDist(dist)
}

func (w WSLStubber) PrepareIgnition(_ *vmconfigs.MachineConfig, _ *ignition.IgnitionBuilder) (*ignition.ReadyUnitOpts, error) {
	return nil, nil
}

func (w WSLStubber) Exists(name string) (bool, error) {
	if !wutil.IsWSLInstalled() {
		return false, nil
	}
	return isWSLExist(env.WithPodmanPrefix(name))
}

func (w WSLStubber) MountType() vmconfigs.VolumeMountType {
	return vmconfigs.Unknown
}

func (w WSLStubber) MountVolumesToVM(_ *vmconfigs.MachineConfig, _ bool) error {
	return nil
}

func (w WSLStubber) Remove(mc *vmconfigs.MachineConfig) ([]string, func() error, error) {
	// Note: we could consider swapping the two conditionals
	// below if we wanted to hard error on the wsl unregister
	// of the vm
	wslRemoveFunc := func() error {
		cmd := wutil.NewWSLCommand("--unregister", env.WithPodmanPrefix(mc.Name))
		if err := runCmdPassThrough(cmd); err != nil {
			return err
		}
		return nil
	}

	return []string{}, wslRemoveFunc, nil
}

func (w WSLStubber) RemoveAndCleanMachines(_ *define.MachineDirs) error {
	return nil
}

func (w WSLStubber) SetProviderAttrs(mc *vmconfigs.MachineConfig, opts define.SetOptions) error {
	state, err := w.State(mc, false)
	if err != nil {
		return err
	}
	if state != define.Stopped {
		return errors.New("unable to change settings unless vm is stopped")
	}

	if opts.Rootful != nil && mc.HostUser.Rootful != *opts.Rootful {
		if err := mc.SetRootful(*opts.Rootful); err != nil {
			return err
		}
	}

	if opts.CPUs != nil {
		return errors.New("changing CPUs not supported for WSL machines")
	}

	if opts.Memory != nil {
		return errors.New("changing memory not supported for WSL machines")
	}

	if opts.USBs != nil {
		return errors.New("changing USBs not supported for WSL machines")
	}

	if opts.DiskSize != nil {
		return errors.New("changing disk size not supported for WSL machines")
	}

	if opts.UserModeNetworking != nil && mc.WSLHypervisor.UserModeNetworking != *opts.UserModeNetworking {
		if running, _ := isRunning(mc.Name); running {
			return errors.New("user-mode networking can only be changed when the machine is not running")
		}

		dist := env.WithPodmanPrefix(mc.Name)
		if err := changeDistUserModeNetworking(dist, mc.SSH.RemoteUsername, mc.ImagePath.GetPath(), *opts.UserModeNetworking); err != nil {
			return fmt.Errorf("failure changing state of user-mode networking setting: %w", err)
		}

		mc.WSLHypervisor.UserModeNetworking = *opts.UserModeNetworking
	}

	return nil
}

func (w WSLStubber) StartNetworking(mc *vmconfigs.MachineConfig, _ *gvproxy.GvproxyCommand) error {
	// Startup user-mode networking if enabled
	if mc.WSLHypervisor.UserModeNetworking {
		return startUserModeNetworking(mc)
	}
	return nil
}

func (w WSLStubber) UserModeNetworkEnabled(mc *vmconfigs.MachineConfig) bool {
	return mc.WSLHypervisor.UserModeNetworking
}

func (w WSLStubber) UseProviderNetworkSetup() bool {
	return true
}

func (w WSLStubber) RequireExclusiveActive() bool {
	return false
}

func (w WSLStubber) PostStartNetworking(mc *vmconfigs.MachineConfig, noInfo bool) error {
	socket, err := mc.APISocket()
	if err != nil {
		return err
	}
	winProxyOpts := machine.WinProxyOpts{
		Name:           mc.Name,
		IdentityPath:   mc.SSH.IdentityPath,
		Port:           mc.SSH.Port,
		RemoteUsername: mc.SSH.RemoteUsername,
		Rootful:        mc.HostUser.Rootful,
		VMType:         w.VMType(),
		Socket:         socket,
	}
	machine.LaunchWinProxy(winProxyOpts, noInfo)

	return nil
}

func (w WSLStubber) StartVM(mc *vmconfigs.MachineConfig) (func() error, func() error, error) {
	dist := env.WithPodmanPrefix(mc.Name)

	err := wslInvoke(dist, "/root/bootstrap")
	if err != nil {
		err = fmt.Errorf("the WSL bootstrap script failed: %w", err)
	}

	readyFunc := func() error {
		return nil
	}

	return nil, readyFunc, err
}

func (w WSLStubber) State(mc *vmconfigs.MachineConfig, _ bool) (define.Status, error) {
	running, err := isRunning(mc.Name)
	if err != nil {
		return "", err
	}
	if running {
		return define.Running, nil
	}
	return define.Stopped, nil
}

func (w WSLStubber) StopVM(mc *vmconfigs.MachineConfig, _ bool) error {
	var err error

	if running, err := isRunning(mc.Name); !running {
		return err
	}

	dist := env.WithPodmanPrefix(mc.Name)

	// Stop user-mode networking if enabled
	if err := stopUserModeNetworking(mc); err != nil {
		fmt.Fprintf(os.Stderr, "Could not cleanly stop user-mode networking: %s\n", err.Error())
	}

	if err := machine.StopWinProxy(mc.Name, vmtype); err != nil {
		fmt.Fprintf(os.Stderr, "Could not stop API forwarding service (win-sshproxy.exe): %s\n", err.Error())
	}

	cmd := wutil.NewWSLCommand("-u", "root", "-d", dist, "sh")
	cmd.Stdin = strings.NewReader(waitTerm)
	out := &bytes.Buffer{}
	cmd.Stderr = out
	cmd.Stdout = out

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("executing wait command: %w", err)
	}

	exitCmd := wutil.NewWSLCommand("-u", "root", "-d", dist, "/usr/local/bin/enterns", "systemctl", "exit", "0")
	if err = exitCmd.Run(); err != nil {
		return fmt.Errorf("stopping systemd: %w", err)
	}

	if err = cmd.Wait(); err != nil {
		logrus.Warnf("Failed to wait for systemd to exit: (%s)", strings.TrimSpace(out.String()))
	}

	return terminateDist(dist)
}

func (w WSLStubber) StopHostNetworking(mc *vmconfigs.MachineConfig, _ define.VMType) error {
	return stopUserModeNetworking(mc)
}

func (w WSLStubber) UpdateSSHPort(mc *vmconfigs.MachineConfig, port int) error {
	dist := env.WithPodmanPrefix(mc.Name)

	if err := wslInvoke(dist, "sh", "-c", fmt.Sprintf(changePort, port)); err != nil {
		return fmt.Errorf("could not change SSH port for guest OS: %w", err)
	}

	return nil
}

func (w WSLStubber) VMType() define.VMType {
	return define.WSLVirt
}

func (w WSLStubber) GetRosetta(_ *vmconfigs.MachineConfig) (bool, error) {
	return false, nil
}
