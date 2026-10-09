//go:build windows

package wsl

import (
	"bytes"
	"context"
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

func (w WSLStubber) CreateVM(ctx context.Context, opts define.CreateVMOpts, mc *vmconfigs.MachineConfig, _ *ignition.IgnitionBuilder) error {
	var err error
	// cleanup half-baked files if init fails at any point
	callbackFuncs := machine.CleanUp()
	defer callbackFuncs.CleanIfErr(ctx, &err)
	go callbackFuncs.CleanOnSignal(ctx, false)
	mc.WSLHypervisor = new(vmconfigs.WSLConfig)

	_ = setupWslProxyEnv()

	if opts.UserModeNetworking {
		if err = verifyWSLUserModeCompat(ctx); err != nil {
			return err
		}
		mc.WSLHypervisor.UserModeNetworking = true
	}

	const prompt = "Importing operating system into WSL (this may take a few minutes on a new WSL install)..."
	dist, err := provisionWSLDist(ctx, mc.Name, mc.ImagePath.GetPath(), prompt)
	if err != nil {
		if errors.Is(err, ErrWslNotSupported) {
			// If error is Wsl/Service/RegisterDistro/CreateVm/HCS/ERROR_NOT_SUPPORTED
			// or Wsl/Service/RegisterDistro/CreateVm/HCS/HCS_E_SERVICE_NOT_AVAILABLE
			// it means WSL's VM creation failed, likely due to virtualization features not being enabled.
			// Relaunching 'podman machine init' in elevated mode will attempt to reconfigure the WSL machine.
			admin := windows.HasAdminRights()

			return attemptFeatureInstall(ctx, opts.ReExec, admin)
		}
		return err
	}

	unprovisionCallbackFunc := func(ctx context.Context) error {
		return unprovisionWSL(ctx, mc)
	}
	callbackFuncs.Add(unprovisionCallbackFunc)

	if mc.WSLHypervisor.UserModeNetworking {
		if err = installUserModeDist(ctx, dist, mc.ImagePath.GetPath()); err != nil {
			_ = unregisterDist(ctx, dist)
			return err
		}
	}

	fmt.Println("Configuring system...")
	if err = configureSystem(ctx, mc, dist, mc.Ansible); err != nil {
		return err
	}

	if err = installScripts(ctx, dist); err != nil {
		return err
	}

	if err = createKeys(ctx, mc, dist); err != nil {
		return err
	}

	// recycle vm
	return terminateDist(ctx, dist)
}

func (w WSLStubber) PrepareIgnition(_ *vmconfigs.MachineConfig, _ *ignition.IgnitionBuilder) (*ignition.ReadyUnitOpts, error) {
	return nil, nil
}

func (w WSLStubber) Exists(ctx context.Context, name string) (bool, error) {
	if !wutil.IsWSLInstalled(ctx) {
		return false, nil
	}
	return isWSLExist(ctx, env.WithPodmanPrefix(name))
}

func (w WSLStubber) MountType() vmconfigs.VolumeMountType {
	return vmconfigs.Unknown
}

func (w WSLStubber) MountVolumesToVM(_ context.Context, _ *vmconfigs.MachineConfig, _ bool) error {
	return nil
}

func (w WSLStubber) Remove(_ context.Context, mc *vmconfigs.MachineConfig) ([]string, func(context.Context) error, error) {
	// Note: we could consider swapping the two conditionals
	// below if we wanted to hard error on the wsl unregister
	// of the vm
	wslRemoveFunc := func(ctx context.Context) error {
		cmd := wutil.NewWSLCommand(ctx, "--unregister", env.WithPodmanPrefix(mc.Name))
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

func (w WSLStubber) SetProviderAttrs(ctx context.Context, mc *vmconfigs.MachineConfig, opts define.SetOptions) error {
	state, err := w.State(ctx, mc, false)
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
		if running, _ := isRunning(ctx, mc.Name); running {
			return errors.New("user-mode networking can only be changed when the machine is not running")
		}

		dist := env.WithPodmanPrefix(mc.Name)
		if err := changeDistUserModeNetworking(ctx, dist, mc.SSH.RemoteUsername, mc.ImagePath.GetPath(), *opts.UserModeNetworking); err != nil {
			return fmt.Errorf("failure changing state of user-mode networking setting: %w", err)
		}

		mc.WSLHypervisor.UserModeNetworking = *opts.UserModeNetworking
	}

	return nil
}

func (w WSLStubber) StartNetworking(ctx context.Context, mc *vmconfigs.MachineConfig, _ *gvproxy.GvproxyCommand) error {
	// Startup user-mode networking if enabled
	if mc.WSLHypervisor.UserModeNetworking {
		return startUserModeNetworking(ctx, mc)
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

func (w WSLStubber) PostStartNetworking(ctx context.Context, mc *vmconfigs.MachineConfig, noInfo bool) error {
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
	machine.LaunchWinProxy(ctx, winProxyOpts, noInfo)

	return nil
}

func (w WSLStubber) StartVM(ctx context.Context, mc *vmconfigs.MachineConfig) (func(context.Context) error, func() error, error) {
	dist := env.WithPodmanPrefix(mc.Name)

	err := wslInvoke(ctx, dist, "/root/bootstrap")
	if err != nil {
		return nil, nil, fmt.Errorf("the WSL bootstrap script failed: %w", err)
	}

	// We need to set cgroup_manager="cgroupfs" in the [engine] section
	// of containers.conf until we use the WSL supported systemd.
	// The details are in this GitHub issue:
	// https://github.com/podman-container-tools/podman/issues/29749
	// The issue about using the WSL supported systemd:
	// https://github.com/podman-container-tools/podman/issues/15967
	err = wslPipe(ctx, cgroupManager, dist, "sh", "-c", "cat > /usr/share/containers/containers.conf.d/999-podman-machine-wsl-cgroupfs.conf")
	if err != nil {
		return nil, nil, fmt.Errorf("failed creating the cgroupfs config file: %w", err)
	}

	readyFunc := func() error {
		return nil
	}

	return nil, readyFunc, nil
}

func (w WSLStubber) State(ctx context.Context, mc *vmconfigs.MachineConfig, _ bool) (define.Status, error) {
	running, err := isRunning(ctx, mc.Name)
	if err != nil {
		return "", err
	}
	if running {
		return define.Running, nil
	}
	return define.Stopped, nil
}

func (w WSLStubber) StopVM(ctx context.Context, mc *vmconfigs.MachineConfig, _ bool) error {
	var err error

	if running, err := isRunning(ctx, mc.Name); !running {
		return err
	}

	dist := env.WithPodmanPrefix(mc.Name)

	// Stop user-mode networking if enabled
	if err := stopUserModeNetworking(ctx, mc); err != nil {
		fmt.Fprintf(os.Stderr, "Could not cleanly stop user-mode networking: %v\n", err)
	}

	if err := machine.StopWinProxy(mc.Name, vmtype); err != nil {
		fmt.Fprintf(os.Stderr, "Could not stop API forwarding service (win-sshproxy.exe): %v\n", err)
	}

	cmd := wutil.NewWSLCommand(ctx, "-u", "root", "-d", dist, "sh")
	cmd.Stdin = strings.NewReader(waitTerm)
	out := &bytes.Buffer{}
	cmd.Stderr = out
	cmd.Stdout = out

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("executing wait command: %w", err)
	}

	exitCmd := wutil.NewWSLCommand(ctx, "-u", "root", "-d", dist, "/usr/local/bin/enterns", "systemctl", "exit", "0")
	if err = exitCmd.Run(); err != nil {
		return fmt.Errorf("stopping systemd: %w", err)
	}

	if err = cmd.Wait(); err != nil {
		logrus.Warnf("Failed to wait for systemd to exit: (%s)", strings.TrimSpace(out.String()))
	}

	return terminateDist(ctx, dist)
}

func (w WSLStubber) StopHostNetworking(ctx context.Context, mc *vmconfigs.MachineConfig, _ define.VMType) error {
	return stopUserModeNetworking(ctx, mc)
}

func (w WSLStubber) UpdateSSHPort(ctx context.Context, mc *vmconfigs.MachineConfig, port int) error {
	dist := env.WithPodmanPrefix(mc.Name)

	if err := wslInvoke(ctx, dist, "sh", "-c", fmt.Sprintf(changePort, port)); err != nil {
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
