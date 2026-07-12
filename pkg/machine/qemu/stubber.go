//go:build linux || freebsd || windows

package qemu

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/config"
	"go.podman.io/common/pkg/strongunits"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/ignition"
	"go.podman.io/podman/v6/pkg/machine/qemu/command"
	"go.podman.io/podman/v6/pkg/machine/sockets"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

type QEMUStubber struct {
	vmconfigs.QEMUConfig
	// Command describes the final QEMU command line
	Command command.QemuCmd

	// virtiofsHelpers are virtiofsd child processes
	virtiofsHelpers []virtiofsdHelperCmd
}

var (
	gvProxyWaitBackoff        = 500 * time.Millisecond
	gvProxyMaxBackoffAttempts = 6
)

func (q *QEMUStubber) UserModeNetworkEnabled(*vmconfigs.MachineConfig) bool {
	return true
}

func (q *QEMUStubber) UseProviderNetworkSetup() bool {
	return false
}

func (q *QEMUStubber) RequireExclusiveActive() bool {
	return true
}

func (q *QEMUStubber) setQEMUCommandLine(mc *vmconfigs.MachineConfig) error {
	qemuBinary, err := findQEMUBinary()
	if err != nil {
		return err
	}

	ignitionFile, err := mc.IgnitionFile()
	if err != nil {
		return err
	}

	readySocket, err := mc.ReadySocket()
	if err != nil {
		return err
	}

	q.QEMUPidPath = mc.QEMUHypervisor.QEMUPidPath

	q.Command = command.NewQemuBuilder(qemuBinary, q.addArchOptions(nil))
	q.Command.SetBootableImage(mc.ImagePath.GetPath())
	q.Command.SetMemory(mc.Resources.Memory)
	q.Command.SetCPUs(mc.Resources.CPUs)
	q.Command.SetIgnitionFile(*ignitionFile)
	q.Command.SetQmpMonitor(mc.QEMUHypervisor.QMPMonitor)
	gvProxySock, err := mc.GVProxySocket()
	if err != nil {
		return err
	}
	if err := q.Command.SetNetwork(gvProxySock); err != nil {
		return err
	}
	q.Command.SetSerialPort(*readySocket, *mc.QEMUHypervisor.QEMUPidPath, mc.Name)

	q.Command.SetUSBHostPassthrough(mc.Resources.USBs)

	return nil
}

func (q *QEMUStubber) CreateVM(opts define.CreateVMOpts, mc *vmconfigs.MachineConfig, ignBuilder *ignition.IgnitionBuilder) error {
	monitor, err := command.NewQMPMonitor(opts.Name, opts.Dirs.RuntimeDir)
	if err != nil {
		return err
	}

	qemuConfig := vmconfigs.QEMUConfig{
		QMPMonitor: monitor,
	}
	machineRuntimeDir, err := mc.RuntimeDir()
	if err != nil {
		return err
	}

	qemuPidPath, err := machineRuntimeDir.AppendToNewVMFile(mc.Name+"_vm.pid", nil)
	if err != nil {
		return err
	}

	mc.QEMUHypervisor = &qemuConfig
	mc.QEMUHypervisor.QEMUPidPath = qemuPidPath

	virtiofsMounts := make([]machine.VirtIoFs, 0, len(mc.Mounts))
	for _, mnt := range mc.Mounts {
		virtiofsMounts = append(virtiofsMounts, machine.MountToVirtIOFs(mnt))
	}
	virtIOIgnitionMounts, err := machine.GenerateSystemDFilesForVirtiofsMounts(virtiofsMounts)
	if err != nil {
		return err
	}
	ignBuilder.WithUnit(virtIOIgnitionMounts...)

	return q.resizeDisk(mc.Resources.DiskSize, mc.ImagePath)
}

func runStartVMCommand(cmd *exec.Cmd) error {
	err := cmd.Start()
	if err != nil {
		// check if qemu was not found
		// look up qemu again maybe the path was changed, https://github.com/containers/podman/issues/13394
		cfg, err := config.Default()
		if err != nil {
			return err
		}
		qemuBinaryPath, err := cfg.FindHelperBinary(QemuCommand, true)
		if err != nil {
			return err
		}
		cmd.Path = qemuBinaryPath
		err = cmd.Start()
		if err != nil {
			return fmt.Errorf("unable to execute %q: %w", cmd, err)
		}
	}
	return nil
}

func (q *QEMUStubber) StartVM(mc *vmconfigs.MachineConfig) (func() error, func() error, error) {
	if err := q.setQEMUCommandLine(mc); err != nil {
		return nil, nil, fmt.Errorf("unable to generate qemu command line: %q", err)
	}

	readySocket, err := mc.ReadySocket()
	if err != nil {
		return nil, nil, err
	}

	gvProxySock, err := mc.GVProxySocket()
	if err != nil {
		return nil, nil, err
	}

	// Wait on gvproxy to be running and aware
	if err := sockets.WaitForSocketWithBackoffs(gvProxyMaxBackoffAttempts, gvProxyWaitBackoff, gvProxySock.GetPath(), "gvproxy"); err != nil {
		return nil, nil, err
	}

	dnr, dnw, err := machine.GetDevNullFiles()
	if err != nil {
		return nil, nil, err
	}
	defer dnr.Close()
	defer dnw.Close()

	runtime, err := mc.RuntimeDir()
	if err != nil {
		return nil, nil, err
	}

	var spawner *virtiofsdSpawner
	if len(mc.Mounts) > 0 {
		spawner, err = newVirtiofsdSpawner(runtime)
		if err != nil {
			return nil, nil, err
		}
	}

	for _, hostmnt := range mc.Mounts {
		qemuArgs, virtiofsdHelper, err := spawner.spawnForMount(hostmnt)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to init virtiofsd for mount %s: %w", hostmnt.Source, err)
		}
		q.Command = append(q.Command, qemuArgs...)
		q.virtiofsHelpers = append(q.virtiofsHelpers, *virtiofsdHelper)
	}

	cmdLine := q.Command

	// Disable graphic window when not in debug mode
	// Done in start, so we're not suck with the debug level we used on init
	if !logrus.IsLevelEnabled(logrus.DebugLevel) {
		cmdLine.SetDisplay("none")
	}

	logrus.Debugf("qemu cmd: %v", cmdLine)

	stderrBuf := &bytes.Buffer{}

	// actually run the command that starts the virtual machine
	cmd := &exec.Cmd{
		Args:   cmdLine,
		Path:   cmdLine[0],
		Stdin:  dnr,
		Stdout: dnw,
		Stderr: stderrBuf,
	}

	if err := runStartVMCommand(cmd); err != nil {
		return nil, nil, err
	}
	logrus.Debugf("Started qemu pid %d", cmd.Process.Pid)

	readyFunc := func() error {
		return waitForReady(readySocket, cmd.Process.Pid, stderrBuf)
	}

	releaseFunc := func() error {
		if err := cmd.Process.Release(); err != nil {
			return err
		}
		for _, virtiofsdCmd := range q.virtiofsHelpers {
			if err := virtiofsdCmd.command.Process.Release(); err != nil {
				return err
			}
		}
		return nil
	}

	// if this is not the last line in the func, make it a defer
	return releaseFunc, readyFunc, nil
}

func waitForReady(readySocket *define.VMFile, pid int, stdErrBuffer *bytes.Buffer) error {
	defaultBackoff := 500 * time.Millisecond
	maxBackoffs := 6
	conn, err := sockets.DialSocketWithBackoffsAndProcCheck(maxBackoffs, defaultBackoff, readySocket.GetPath(), checkProcessStatus, "qemu", pid, stdErrBuffer)
	if err != nil {
		return err
	}
	defer conn.Close()

	_, err = bufio.NewReader(conn).ReadString('\n')
	return err
}

func (q *QEMUStubber) Exists(_ string) (bool, error) {
	return false, nil
}

func (q *QEMUStubber) VMType() define.VMType {
	return define.QemuVirt
}

func (q *QEMUStubber) PrepareIgnition(_ *vmconfigs.MachineConfig, _ *ignition.IgnitionBuilder) (*ignition.ReadyUnitOpts, error) {
	return nil, nil
}

func (q *QEMUStubber) StopHostNetworking(_ *vmconfigs.MachineConfig, _ define.VMType) error {
	return define.ErrNotImplemented
}

func (q *QEMUStubber) resizeDisk(newSize strongunits.GiB, diskPath *define.VMFile) error {
	// Find the qemu executable
	cfg, err := config.Default()
	if err != nil {
		return err
	}
	resizePath, err := cfg.FindHelperBinary("qemu-img", true)
	if err != nil {
		return err
	}
	resize := exec.Command(resizePath, []string{"resize", diskPath.GetPath(), strconv.Itoa(int(newSize)) + "G"}...)
	resize.Stdout = os.Stdout
	resize.Stderr = os.Stderr
	if err := resize.Run(); err != nil {
		return fmt.Errorf("resizing image: %q", err)
	}

	return nil
}

func (q *QEMUStubber) SetProviderAttrs(mc *vmconfigs.MachineConfig, opts define.SetOptions) error {
	state, err := q.State(mc, false)
	if err != nil {
		return err
	}
	if state != define.Stopped {
		return errors.New("unable to change settings unless vm is stopped")
	}

	if opts.DiskSize != nil {
		if err := q.resizeDisk(*opts.DiskSize, mc.ImagePath); err != nil {
			return err
		}
	}

	if opts.Rootful != nil && mc.HostUser.Rootful != *opts.Rootful {
		if err := mc.SetRootful(*opts.Rootful); err != nil {
			return err
		}
	}

	if opts.USBs != nil {
		usbs, err := define.ParseUSBs(*opts.USBs)
		if err != nil {
			return err
		}
		mc.Resources.USBs = usbs
	}

	// Because QEMU does nothing with these hardware attributes, we can simply return
	return nil
}

func (q *QEMUStubber) StartNetworking(mc *vmconfigs.MachineConfig, cmd *gvproxy.GvproxyCommand) error {
	gvProxySock, err := mc.GVProxySocket()
	if err != nil {
		return err
	}
	socketURL, err := sockets.ToUnixURL(gvProxySock)
	if err != nil {
		return err
	}
	// make sure it does not exist before gvproxy is called
	if err := gvProxySock.Delete(); err != nil {
		logrus.Error(err)
	}
	cmd.AddQemuSocket(socketURL.String())
	return nil
}

func (q *QEMUStubber) RemoveAndCleanMachines(_ *define.MachineDirs) error {
	// nothing to do but remove files
	return nil
}

func (q *QEMUStubber) MountVolumesToVM(_ *vmconfigs.MachineConfig, _ bool) error {
	// virtiofs: mounts are handled by systemd units baked into ignition at init time
	return nil
}

func (q *QEMUStubber) MountType() vmconfigs.VolumeMountType {
	return vmconfigs.VirtIOFS
}

func (q *QEMUStubber) PostStartNetworking(_ *vmconfigs.MachineConfig, _ bool) error {
	return nil
}

func (q *QEMUStubber) UpdateSSHPort(_ *vmconfigs.MachineConfig, _ int) error {
	// managed by gvproxy on this backend, so nothing to do
	return nil
}

func (q *QEMUStubber) GetRosetta(_ *vmconfigs.MachineConfig) (bool, error) {
	return false, nil
}
