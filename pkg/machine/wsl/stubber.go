//go:build windows

package wsl

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

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

	// WSL v2.5.1 or later is required. Earlier versions will fail to
	// start Podman machines because they supported cgroupv1.
	if !wutil.WSLVersionAtLeast(2, 5, 1) {
		return fmt.Errorf("WSL version 2.5.1 or later is required to run Podman. Please upgrade WSL with: wsl --update")
	}

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
		dist := env.WithPodmanPrefix(mc.Name)
		if err := changeDistUserModeNetworking(dist, mc.ImagePath.GetPath(), *opts.UserModeNetworking); err != nil {
			return fmt.Errorf("failure changing state of user-mode networking setting: %w", err)
		}

		// Changing the setting runs commands in the distribution, which boots
		// it with systemd. Stop it so that the machine is still reported as
		// stopped and the new wsl.conf is used at the next start.
		if err := terminateDist(dist); err != nil {
			return fmt.Errorf("could not stop the WSL distribution: %w", err)
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

	// On legacy machines, created by Podman v6.1 and earlier, the bootstrap
	// script starts systemd in a separate namespace.
	err := wslInvoke(dist, "/root/bootstrap")
	if err != nil {
		return nil, nil, fmt.Errorf("the WSL bootstrap script failed: %w", err)
	}

	readyFunc := func() error {
		return nil
	}

	return nil, readyFunc, nil
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
	if running, err := isRunning(mc.Name); !running {
		return err
	}

	dist := env.WithPodmanPrefix(mc.Name)

	// Stop user-mode networking if enabled
	if err := stopUserModeNetworking(mc); err != nil {
		logrus.Warnf("Could not cleanly stop user-mode networking: %v", err)
	}

	if err := machine.StopWinProxy(mc.Name, vmtype); err != nil {
		logrus.Warnf("Could not stop API forwarding service (win-sshproxy.exe): %v", err)
	}

	if err := stopAndWaitSystemd(dist); err != nil {
		return err
	}

	return terminateDist(dist)
}

func stopAndWaitSystemd(dist string) error {
	legacySystemd, err := usesLegacySystemd(dist)
	if err != nil {
		logrus.Warnf("Could not check if %q used the legacy systemd trick: %v", dist, err)
	}
	if legacySystemd {
		return stopAndWaitLegacySystemd(dist)
	}
	return stopAndWaitNativeSystemd(dist)
}

func stopAndWaitNativeSystemd(dist string) error {
	if err := wslPipe(termNative, dist, "sh"); err != nil {
		return fmt.Errorf("terminating systemd: %w", err)
	}
	timeout, _ := strconv.Atoi(termTimeoutSeconds)
	for deadline := time.Now().Add(time.Duration(timeout) * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		running, err := isWSLRunning(dist)
		if err != nil || !running {
			return err
		}
	}
	logrus.Warnf("Timed out waiting for a systemd clean shutdown after %d seconds", timeout)
	return nil
}

func stopAndWaitLegacySystemd(dist string) error {
	cmd := wutil.NewWSLCommand("-u", "root", "-d", dist, "sh")
	logrus.Debugf("Running command: %s %v", cmd.Path, cmd.Args)
	cmd.Stdin = strings.NewReader(waitTermLegacy)
	out := &bytes.Buffer{}
	cmd.Stderr = out
	cmd.Stdout = out

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("executing wait command: %w", err)
	}

	if err := wslPipe(termLegacy, dist, "sh"); err != nil {
		return fmt.Errorf("terminating main process: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		logrus.Warnf("Failed to wait for systemd to shutdown cleanly: %v (%s)", err, strings.TrimSpace(out.String()))
	}
	return nil
}

// usesLegacySystemd checks if the file /usr/local/bin/enterns
// exists in the machine. If it does, the machine started a namespaced systemd
// at boot (a trick required by older versions of WSL), and doesn't use the
// native WSL systemd.
// Note: this function should be used carefully: if the WSL distribution
// is stopped it has the side effect to start it.
func usesLegacySystemd(dist string) (bool, error) {
	err := wslPipe(isLegacy, dist, "sh")
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (w WSLStubber) StopHostNetworking(mc *vmconfigs.MachineConfig, _ define.VMType) error {
	return stopUserModeNetworking(mc)
}

func (w WSLStubber) UpdateSSHPort(mc *vmconfigs.MachineConfig, port int) error {
	dist := env.WithPodmanPrefix(mc.Name)

	if err := wslInvoke(dist, "sh", "-c", fmt.Sprintf(changePort, port)); err != nil {
		return fmt.Errorf("could not change SSH port for guest OS: %w", err)
	}

	// Try restarting sshd with the new port. This is in case it was already started at
	// the WSL distribution boot.
	if err := wslPipe(restartSSHD, dist, "sh"); err != nil {
		return fmt.Errorf("could not restart sshd after SSH port change: %w", err)
	}

	return nil
}

func (w WSLStubber) VMType() define.VMType {
	return define.WSLVirt
}

func (w WSLStubber) GetRosetta(_ *vmconfigs.MachineConfig) (bool, error) {
	return false, nil
}
