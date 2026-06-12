//go:build windows

package hyperv

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/Microsoft/go-winio"
	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"github.com/containers/libhvee/pkg/hypervctl"
	"github.com/containers/libhvee/pkg/kvp/ginsu"
	"github.com/sirupsen/logrus"
	"go.podman.io/common/pkg/strongunits"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/env"
	"go.podman.io/podman/v6/pkg/machine/hyperv/vsock"
	"go.podman.io/podman/v6/pkg/machine/ignition"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/podman/v6/pkg/machine/windows"
	"go.podman.io/podman/v6/pkg/systemd/parser"
)

type HyperVStubber struct {
	vmconfigs.HyperVConfig
}

type permissionChecks struct {
	isElevatedProcess   func() bool
	isHyperVAdminMember func() error
	vsockEntriesExist   func(int) bool
	existingMachinesNum func() (int, error)
}

func (h HyperVStubber) defaultPermissionChecks() permissionChecks {
	return permissionChecks{
		isElevatedProcess:   windows.HasAdminRights,
		isHyperVAdminMember: VerifyHyperVPermissions,
		vsockEntriesExist:   vsock.CheckIfHVSockRegistryEntriesExist,
		existingMachinesNum: h.countMachinesWithToolname,
	}
}

func (h HyperVStubber) UserModeNetworkEnabled(_ *vmconfigs.MachineConfig) bool {
	return true
}

func (h HyperVStubber) UseProviderNetworkSetup() bool {
	return false
}

func (h HyperVStubber) RequireExclusiveActive() bool {
	return true
}

func (h HyperVStubber) CreateVM(_ define.CreateVMOpts, mc *vmconfigs.MachineConfig, builder *ignition.IgnitionBuilder) error {
	var err error
	callbackFuncs := machine.CleanUp()
	defer callbackFuncs.CleanIfErr(&err)
	callbackFuncs.Add(createErrorLogCallback(&err))
	go callbackFuncs.CleanOnSignal()

	hwConfig := hypervctl.HardwareConfig{
		CPUs:     uint16(mc.Resources.CPUs),
		DiskPath: mc.ImagePath.GetPath(),
		DiskSize: uint64(mc.Resources.DiskSize),
		Memory:   uint64(mc.Resources.Memory),
	}

	// Allow creation in these two cases:
	// 1. if the user is Admin
	// 2. if the user has Hyper-V admin rights and a vsock registry entry exists
	// *NEW* machines are those created with the vsock entry having the ToolName field.
	//
	// This is to prevent to prevent an error for trying to create a vsock entry
	// in the Windows registry if the user doesn't have the privileges.
	if err := h.canCreate(len(mc.Mounts)); err != nil {
		// If it returns ErrHypervRegistryInitRequiresElevation and we're not already re-executing,
		// offer to elevate automatically if user is in admin group
		if errors.Is(err, ErrHypervRegistryInitRequiresElevation) &&
			!windows.IsReExecuting() &&
			windows.IsInAdministratorsGroup() {
			message := fmt.Sprintf("%s.\n\n%s", ErrHypervPrepareHostForHyperV.Error(), windows.UACConfirmationPrompt)
			return launchElevate(message)
		}
		return err
	}

	// Once here, we know the user has admin rights but may not be in
	// the Hyper-V Administrators group yet. Add them if necessary so
	// that future commands work without elevation.
	if windows.IsReExecuting() && !IsHyperVAdminsGroupMember() {
		u, err := user.Current()
		if err != nil {
			return err
		}
		logrus.Infof("Adding user %s to the Hyper-V Administrators group", u.Username)
		if err := AddUserToHyperVAdminGroup(u.Username); err != nil {
			return err
		}
	}

	// count number of existing machines, used later to determine if Registry should be cleaned over a failure
	machines, err := h.countMachinesWithToolname()
	if err != nil {
		return err
	}

	// Callback to remove any created vsock entries in the Windows Registry if the creation fails
	removeRegistryEntriesCallBack := func() error {
		// Allow removal only if user is Admin and this is the first machine created.
		// If there are already existing machines, the vsock entries should remain.
		//
		// There is no need to check for admin rights here as this is already a requirement
		// to create the first machine and so it would have failed earlier.
		if machines > 0 {
			return nil
		}

		if err := vsock.RemoveAllHVSockRegistryEntries(); err != nil {
			return fmt.Errorf("unable to remove hvsock registry entries: %q", err)
		}

		return nil
	}
	callbackFuncs.Add(removeRegistryEntriesCallBack)

	// Attempt to load an existing HVSock registry entry for networking.
	// If no existing entry is found, create a new one.
	// Creating a new entry requires administrative rights.
	networkHVSock, err := vsock.LoadHVSockRegistryEntryByPurpose(vsock.Network)
	if err != nil {
		if !windows.HasAdminRights() {
			return ErrHypervRegistryInitRequiresElevation
		}
		networkHVSock, err = vsock.NewHVSockRegistryEntry(vsock.Network, false)
		if err != nil {
			return err
		}
	}

	mc.HyperVHypervisor.NetworkVSock = *networkHVSock

	// Add vsock port numbers to mounts
	err = createShares(mc)
	if err != nil {
		return err
	}

	netUnitFile, err := createNetworkUnit(mc.HyperVHypervisor.NetworkVSock.Port)
	if err != nil {
		return err
	}

	builder.WithUnit(ignition.Unit{
		Contents: ignition.StrToPtr(netUnitFile),
		Enabled:  ignition.BoolToPtr(true),
		Name:     "vsock-network.service",
	})

	builder.WithFile(ignition.File{
		Node: ignition.Node{
			Path: "/etc/NetworkManager/system-connections/vsock0.nmconnection",
		},
		FileEmbedded1: ignition.FileEmbedded1{
			Append: nil,
			Contents: ignition.Resource{
				Source: ignition.EncodeDataURLPtr(hyperVVsockNMConnection),
			},
			Mode: ignition.IntToPtr(0o600),
		},
	})

	vmm := hypervctl.NewVirtualMachineManager()
	err = vmm.NewVirtualMachine(mc.Name, &hwConfig)
	if err != nil {
		return err
	}

	vmRemoveCallback := func() error {
		vm, err := vmm.GetMachine(mc.Name)
		if err != nil {
			return err
		}
		return vm.Remove("")
	}

	callbackFuncs.Add(vmRemoveCallback)
	err = resizeDisk(mc.Resources.DiskSize, mc.ImagePath)
	return err
}

func (h HyperVStubber) Exists(name string) (bool, error) {
	// If the user lacks permissions, WMI will throw an access denied error.
	// We return false to prevent breaking commands like `init`
	// that loop over all providers to verify machine name uniqueness.
	if err := VerifyHyperVPermissions(); err != nil {
		return false, nil //nolint:nilerr
	}

	vmm := hypervctl.NewVirtualMachineManager()
	exists, _, err := vmm.GetMachineExists(name)
	return exists, err
}

func (h HyperVStubber) MountType() vmconfigs.VolumeMountType {
	return vmconfigs.NineP
}

func (h HyperVStubber) MountVolumesToVM(mc *vmconfigs.MachineConfig, _ bool) error {
	var (
		err        error
		executable string
	)
	callbackFuncs := machine.CleanUp()
	defer callbackFuncs.CleanIfErr(&err)
	go callbackFuncs.CleanOnSignal()

	if len(mc.Mounts) == 0 {
		return nil
	}

	var (
		dirs       *define.MachineDirs
		gvproxyPID int
	)
	dirs, err = env.GetMachineDirs(h.VMType())
	if err != nil {
		return err
	}
	// GvProxy PID file path is now derived
	gvproxyPIDFile, err := dirs.RuntimeDir.AppendToNewVMFile("gvproxy.pid", nil)
	if err != nil {
		return err
	}
	gvproxyPID, err = gvproxyPIDFile.ReadPIDFrom()
	if err != nil {
		return err
	}

	executable, err = os.Executable()
	if err != nil {
		return err
	}
	// Start the 9p server in the background
	p9ServerArgs := []string{}
	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		p9ServerArgs = append(p9ServerArgs, "--log-level=debug")
	}
	p9ServerArgs = append(p9ServerArgs, "machine", "server9p")

	for _, mount := range mc.Mounts {
		if mount.VSockNumber == nil {
			return fmt.Errorf("mount %s has no vsock port defined", mount.Source)
		}
		p9ServerArgs = append(p9ServerArgs, "--serve", fmt.Sprintf("%s:%s", mount.Source, winio.VsockServiceID(uint32(*mount.VSockNumber)).String()))
	}
	p9ServerArgs = append(p9ServerArgs, fmt.Sprintf("%d", gvproxyPID))

	logrus.Debugf("Going to start 9p server using command: %s %v", executable, p9ServerArgs)

	fsCmd := exec.Command(executable, p9ServerArgs...)

	if logrus.IsLevelEnabled(logrus.DebugLevel) {
		log, err := logCommandToFile(fsCmd, "podman-machine-server9.log")
		if err != nil {
			return err
		}
		defer log.Close()
	}

	err = fsCmd.Start()
	if err != nil {
		return fmt.Errorf("unable to start 9p server: %v", err)
	}
	logrus.Infof("Started podman 9p server as PID %d", fsCmd.Process.Pid)

	// Note: No callback is needed to stop the 9p server, because it will stop when
	// gvproxy stops

	// Finalize starting shares after we are confident gvproxy is still alive.
	err = startShares(mc)
	return err
}

func (h HyperVStubber) Remove(mc *vmconfigs.MachineConfig) ([]string, func() error, error) {
	// Allow removal in these two cases:
	// 1. if the user is Admin
	// 2. if the user has Hyper-V admin rights and there are 2+ *NEW* machines.
	// *NEW* machines are those created with the vsock entry having the ToolName field.
	//
	// This is to prevent a non-admin user from deleting the last machine
	// which would require removal of vsock entries from the Windows Registry.
	if err := h.canRemove(mc); err != nil {
		// If we get ErrHypervRegistryRemoveRequiresElevation and we're not already re-executing,
		// and the user has admin rights (is in admin group), offer to elevate automatically
		if errors.Is(err, ErrHypervRegistryRemoveRequiresElevation) &&
			!windows.IsReExecuting() &&
			windows.IsInAdministratorsGroup() {
			message := "Removing this Hyper-V machine requires admin rights to clean up the Windows Registry.\n\n" +
				windows.UACConfirmationPrompt
			return nil, nil, launchElevate(message)
		}
		return nil, nil, err
	}

	_, vm, err := GetVMFromMC(mc)
	if err != nil {
		return nil, nil, err
	}

	rmFunc := func() error {
		// Remove ignition registry entries - not a fatal error
		// for vm removal
		// TODO we could improve this by recommending an action be done
		if err := removeIgnitionFromRegistry(mc, vm); err != nil {
			logrus.Errorf("unable to remove ignition registry entries: %q", err)
		}

		// disk path removal is done by generic remove
		if err = vm.Remove(""); err != nil {
			return err
		}

		// remove vsock registry entries
		if isLegacyMachine(mc) {
			removeLegacyHvSockEntries(mc)
		} else {
			if err := h.removeHvSockFromRegistry(); err != nil {
				logrus.Errorf("unable to remove hvsock registry entries: %q", err)
			}
		}

		return nil
	}
	return []string{}, rmFunc, nil
}

// canCreate checks if the user can create an Hyper-V machine.
// It returns `nil` if the user can create it. The conditions are:
//   - the command is running as Administrator (elevated mode)
//     OR
//   - vsock entries in the Windows registry already exist and
//     the user is a member of the Hyper-V Administrators group
//
// The `mounts` parameter is the number of Mounts of the machine. A
// specific Windows registry entry is necessary for every single mount.
//
// It returns `ErrHypervRegistryInitRequiresElevation` if the vsock
// entries in the Windows registry don't exist.
//
// It returns `ErrHypervUserNotInAdminGroup` if the user doesn't
// belong to the Hyper-V administrators group.
func (h HyperVStubber) canCreate(mounts int) error {
	return checkCanCreate(h.defaultPermissionChecks(), mounts)
}

func checkCanCreate(checks permissionChecks, mounts int) error {
	if checks.isElevatedProcess() {
		return nil
	}
	if !checks.vsockEntriesExist(mounts) {
		return ErrHypervRegistryInitRequiresElevation
	}
	return checks.isHyperVAdminMember()
}

// launchElevate attempts to automatically re-run the command as administrator
// This is similar to how WSL handles elevation.
func launchElevate(message string) error {
	if windows.MessageBox(message, "Podman Machine", false) != 1 {
		return errors.New("elevation process cancelled by user. Please rerun the command as administrator")
	}
	err := windows.CreateOrTruncateElevatedOutputFile()
	if err != nil {
		return err
	}

	err = windows.RelaunchElevatedWait()
	if err != nil {
		windows.DumpOutputFile()
		return fmt.Errorf("elevated process failed with error: %w", err)
	}
	return define.ErrRelaunchSucceeded
}

// createErrorLogCallback creates a callback function that logs errors to file when --reexec is detected
func createErrorLogCallback(err *error) func() error {
	return func() error {
		if *err != nil && windows.IsReExecuting() {
			windows.LogErrorToFile(*err)
		}
		return nil
	}
}

func isLegacyMachine(mc *vmconfigs.MachineConfig) bool {
	return mc.HyperVHypervisor != nil && mc.HyperVHypervisor.ReadyVsock.MachineName != ""
}

// canRemove checks if the user can remove an Hyper-V machine.
// It returns `nil` when the user can remove it. The conditions are:
//   - the command is running as Administrator (elevated mode)
//     OR
//   - the machine isn't a legacy one (one that doesn't share the vsock) AND
//     the user is a member of the Hyper-V Administrators group AND
//     it's not the last machine OR the vsock entries have `KeepAfterMachineRemove=true`
//
// It returns `ErrHypervRegistryRemoveRequiresElevation` if the machine is legacy one OR
// if the vsock entry in the Windows registry doesn't have `KeepAfterMachineRemove=true`.
//
// It returns `ErrHypervUserNotInAdminGroup` if the user doesn't
// belogn to the Hyper-V administrators group.
func (h HyperVStubber) canRemove(mc *vmconfigs.MachineConfig) error {
	return checkCanRemove(mc, h.defaultPermissionChecks())
}

func checkCanRemove(mc *vmconfigs.MachineConfig, checks permissionChecks) error {
	if checks.isElevatedProcess() {
		return nil
	}
	if isLegacyMachine(mc) {
		return ErrHypervLegacyMachineRequiresElevation
	}
	if err := checks.isHyperVAdminMember(); err != nil {
		return err
	}
	machines, err := checks.existingMachinesNum()
	if err != nil {
		return err
	}
	if canSkipVSockEntriesRemoval(machines, *mc.HyperVHypervisor) {
		return nil
	}
	return ErrHypervRegistryRemoveRequiresElevation
}

func canSkipVSockEntriesRemoval(machines int, hv vmconfigs.HyperVConfig) bool {
	if machines > 1 {
		return true
	}
	if !hv.NetworkVSock.KeepAfterMachineRemove ||
		!hv.ReadyVsock.KeepAfterMachineRemove {
		return false
	}
	for _, m := range hv.FileserverVSocks {
		if !m.KeepAfterMachineRemove {
			return false
		}
	}
	return true
}

// canStartOrStop checks if the machine can be started or stopped.
// Legacy machines require admin rights to start or stop.
func (h HyperVStubber) canStartOrStop(mc *vmconfigs.MachineConfig) error {
	if windows.HasAdminRights() {
		return nil
	}

	// if machine is legacy (machineName field), require admin rights to start or stop
	if isLegacyMachine(mc) {
		return ErrHypervLegacyMachineRequiresElevation
	}

	if err := VerifyHyperVPermissions(); err != nil {
		return err
	}

	return nil
}

// countMachinesWithToolname counts only machines that have a toolname field with value "podman".
func (h HyperVStubber) countMachinesWithToolname() (int, error) {
	dirs, err := env.GetMachineDirs(h.VMType())
	if err != nil {
		return 0, err
	}
	mcs, err := vmconfigs.LoadMachinesInDir(dirs)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, mc := range mcs {
		if mc.HyperVHypervisor != nil && mc.HyperVHypervisor.ReadyVsock.ToolName == "podman" {
			count++
		}
	}
	return count, nil
}

// removeLegacyHvSockEntries removes any legacy HVSOCK registry entries associated with the machine.
// This is used to clean up entries from older versions of Podman that did not manage the Toolname field.
func removeLegacyHvSockEntries(mc *vmconfigs.MachineConfig) {
	if mc.HyperVHypervisor.NetworkVSock.MachineName != "" {
		// Remove the HVSOCK for networking
		if err := mc.HyperVHypervisor.NetworkVSock.Remove(); err != nil {
			logrus.Errorf("unable to remove registry entry for %s: %q", mc.HyperVHypervisor.NetworkVSock.KeyName, err)
		}
	}

	if mc.HyperVHypervisor.ReadyVsock.MachineName != "" {
		// Remove the HVSOCK for events
		if err := mc.HyperVHypervisor.ReadyVsock.Remove(); err != nil {
			logrus.Errorf("unable to remove registry entry for %s: %q", mc.HyperVHypervisor.ReadyVsock.KeyName, err)
		}
	}

	for _, mount := range mc.Mounts {
		if mount.VSockNumber == nil {
			// nothing to do if the vsock number was never defined
			continue
		}

		vsockReg, err := vsock.LoadHVSockRegistryEntry(*mount.VSockNumber)
		if err != nil {
			logrus.Debugf("Vsock %d for mountpoint %s does not have a valid registry entry, skipping removal", *mount.VSockNumber, mount.Target)
			continue
		}

		if vsockReg.MachineName == "" {
			continue
		}

		if err := vsockReg.Remove(); err != nil {
			logrus.Debugf("unable to remove vsock %d for mountpoint %s: %v", *mount.VSockNumber, mount.Target, err)
		}
	}
}

func (h HyperVStubber) removeHvSockFromRegistry() error {
	// Remove hvsock registry entries only if this is the last machine
	machines, err := h.countMachinesWithToolname()
	if err != nil {
		return err
	}
	if machines > 1 {
		return nil
	}
	return vsock.RemoveAllHVSockRegistryEntries()
}

func (h HyperVStubber) RemoveAndCleanMachines(_ *define.MachineDirs) error {
	return nil
}

func (h HyperVStubber) StartNetworking(mc *vmconfigs.MachineConfig, cmd *gvproxy.GvproxyCommand) error {
	cmd.AddEndpoint(fmt.Sprintf("vsock://%s", mc.HyperVHypervisor.NetworkVSock.KeyName))
	return nil
}

func (h HyperVStubber) StartVM(mc *vmconfigs.MachineConfig) (func() error, func() error, error) {
	if err := h.canStartOrStop(mc); err != nil {
		return nil, nil, err
	}

	_, vm, err := GetVMFromMC(mc)
	if err != nil {
		return nil, nil, err
	}

	callbackFuncs := machine.CleanUp()
	defer callbackFuncs.CleanIfErr(&err)
	callbackFuncs.Add(createErrorLogCallback(&err))
	go callbackFuncs.CleanOnSignal()

	if mc.IsFirstBoot() {
		// this is added because if the machine does not start
		// properly on first boot, the next boot will be considered
		// the first boot again and the addition of the ignition
		// entries will fail because the key/value pairs already exist.
		rmIgnCallbackFunc := func() error {
			return removeIgnitionFromRegistry(mc, vm)
		}
		callbackFuncs.Add(rmIgnCallbackFunc)

		// Add ignition entries to windows registry
		// for first boot only
		if err = readAndSplitIgnition(mc, vm); err != nil {
			return nil, nil, err
		}
	}

	waitReady, listener, err := mc.HyperVHypervisor.ReadyVsock.ListenSetupWait()
	if err != nil {
		return nil, nil, err
	}

	err = vm.Start()
	if err != nil {
		// cleanup the pending listener
		_ = listener.Close()
		return nil, nil, err
	}

	startCallback := func() error {
		return vm.Stop()
	}
	callbackFuncs.Add(startCallback)

	return nil, waitReady, err
}

// State is returns the state as a define.status.  for hyperv, state differs from others because
// state is determined by the VM itself.  normally this can be done with vm.State() and a conversion
// but doing here as well.  this requires a little more interaction with the hypervisor
func (h HyperVStubber) State(mc *vmconfigs.MachineConfig, bypass bool) (define.Status, error) {
	// If the user does not have permissions, WMI will fail with an error anyway.
	if err := VerifyHyperVPermissions(); err != nil {
		if bypass {
			logrus.Warnf("unable to get state for machine %s: %v", mc.Name, err)
			return define.Unknown, nil
		}
		return define.Unknown, fmt.Errorf("unable to get state for machine %s: %w", mc.Name, err)
	}

	_, vm, err := GetVMFromMC(mc)
	if err != nil {
		return define.Unknown, err
	}
	return stateConversion(vm.State())
}

func (h HyperVStubber) StopVM(mc *vmconfigs.MachineConfig, hardStop bool) error {
	if err := h.canStartOrStop(mc); err != nil {
		return err
	}

	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(mc.Name)
	if err != nil {
		return fmt.Errorf("getting virtual machine: %w", err)
	}
	vmState := vm.State()
	if vm.State() == hypervctl.Disabled {
		return nil
	}
	if vmState != hypervctl.Enabled { // more states could be provided as well
		return hypervctl.ErrMachineStateInvalid
	}

	if hardStop {
		return vm.StopWithForce()
	}
	return vm.Stop()
}

// TODO should this be plumbed higher into the code stack?
func (h HyperVStubber) StopHostNetworking(mc *vmconfigs.MachineConfig, vmType define.VMType) error {
	err := machine.StopWinProxy(mc.Name, vmType)
	// in podman 4, this was a "soft" error; keeping behavior as such
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not stop API forwarding service (win-sshproxy.exe): %s\n", err.Error())
	}

	return nil
}

func (h HyperVStubber) VMType() define.VMType {
	return define.HyperVVirt
}

func GetVMFromMC(mc *vmconfigs.MachineConfig) (*hypervctl.VirtualMachineManager, *hypervctl.VirtualMachine, error) {
	vmm := hypervctl.NewVirtualMachineManager()
	vm, err := vmm.GetMachine(mc.Name)
	return vmm, vm, err
}

func stateConversion(s hypervctl.EnabledState) (define.Status, error) {
	switch s {
	case hypervctl.Enabled:
		return define.Running, nil
	case hypervctl.Disabled:
		return define.Stopped, nil
	case hypervctl.Starting:
		return define.Starting, nil
	}
	return define.Unknown, fmt.Errorf("unknown state: %q", s.String())
}

func (h HyperVStubber) SetProviderAttrs(mc *vmconfigs.MachineConfig, opts define.SetOptions) error {
	if err := VerifyHyperVPermissions(); err != nil {
		return err
	}

	var cpuChanged, memoryChanged bool

	_, vm, err := GetVMFromMC(mc)
	if err != nil {
		return err
	}

	if vm.State() != hypervctl.Disabled {
		return errors.New("unable to change settings unless vm is stopped")
	}

	if opts.Rootful != nil && mc.HostUser.Rootful != *opts.Rootful {
		if err := mc.SetRootful(*opts.Rootful); err != nil {
			return err
		}
	}

	if opts.DiskSize != nil {
		if err := resizeDisk(*opts.DiskSize, mc.ImagePath); err != nil {
			return err
		}
	}
	if opts.CPUs != nil {
		cpuChanged = true
	}
	if opts.Memory != nil {
		memoryChanged = true
	}

	if cpuChanged || memoryChanged {
		err := vm.UpdateProcessorMemSettings(func(ps *hypervctl.ProcessorSettings) {
			if cpuChanged {
				ps.VirtualQuantity = *opts.CPUs
			}
		}, func(ms *hypervctl.MemorySettings) {
			if memoryChanged {
				ms.DynamicMemoryEnabled = false
				mem := uint64(*opts.Memory)
				ms.VirtualQuantity = mem
				ms.Limit = mem
				ms.Reservation = mem
			}
		})
		if err != nil {
			return fmt.Errorf("setting CPU and Memory for VM: %w", err)
		}
	}

	if opts.USBs != nil {
		return fmt.Errorf("changing USBs not supported for hyperv machines")
	}

	return nil
}

func (h HyperVStubber) PrepareIgnition(mc *vmconfigs.MachineConfig, _ *ignition.IgnitionBuilder) (*ignition.ReadyUnitOpts, error) {
	// HyperV is different because it has to know some ignition details before creating the VM.  It cannot
	// simply be derived. So we create the HyperVConfig here.
	mc.HyperVHypervisor = new(vmconfigs.HyperVConfig)
	var ignOpts ignition.ReadyUnitOpts

	// Attempt to load an existing HVSock registry entry for events.
	// If no existing entry is found, create a new one.
	// Creating a new entry requires administrative rights.
	readySock, err := vsock.LoadHVSockRegistryEntryByPurpose(vsock.Events)
	if err != nil {
		if !windows.HasAdminRights() {
			if !windows.IsReExecuting() && windows.IsInAdministratorsGroup() {
				message := fmt.Sprintf("%s.\n\n%s", ErrHypervPrepareHostForHyperV.Error(), windows.UACConfirmationPrompt)
				return nil, launchElevate(message)
			}
			return nil, ErrHypervRegistryInitRequiresElevation
		}
		readySock, err = vsock.NewHVSockRegistryEntry(vsock.Events, false)
		if err != nil {
			return nil, err
		}
	}

	// TODO Stopped here ... fails bc mc.Hypervisor is nil ... this can be nil checked prior and created
	// however the same will have to be done in create
	mc.HyperVHypervisor.ReadyVsock = *readySock
	ignOpts.Port = readySock.Port
	return &ignOpts, nil
}

func (h HyperVStubber) PostStartNetworking(_ *vmconfigs.MachineConfig, _ bool) error {
	return nil
}

func (h HyperVStubber) UpdateSSHPort(_ *vmconfigs.MachineConfig, _ int) error {
	// managed by gvproxy on this backend, so nothing to do
	return nil
}

func resizeDisk(newSize strongunits.GiB, imagePath *define.VMFile) error {
	resize := exec.Command("powershell", "-command", fmt.Sprintf("Resize-VHD \"$ENV:IMAGE_PATH\" %d", newSize.ToBytes()))
	logrus.Debug(resize.Args)
	resize.Stdout = os.Stdout
	resize.Stderr = os.Stderr
	resize.Env = append(os.Environ(), "IMAGE_PATH="+imagePath.GetPath())
	if err := resize.Run(); err != nil {
		return fmt.Errorf("resizing image: %q", err)
	}
	return nil
}

func getIgnitionReader(mc *vmconfigs.MachineConfig) (*bytes.Reader, error) {
	ignFile, err := mc.IgnitionFile()
	if err != nil {
		return nil, err
	}
	ign, err := ignFile.Read()
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(ign), nil
}

// readAndSplitIgnition reads the ignition file and splits it into key:value pairs
func readAndSplitIgnition(mc *vmconfigs.MachineConfig, vm *hypervctl.VirtualMachine) error {
	reader, err := getIgnitionReader(mc)
	if err != nil {
		return err
	}
	return vm.SplitAndAddIgnition("ignition.config.", reader)
}

func removeIgnitionFromRegistry(mc *vmconfigs.MachineConfig, vm *hypervctl.VirtualMachine) error {
	// because the vm is down at this point, we cannot query hyperv for these key value pairs.
	// therefore we recalculate the number of parts ignition file consists of and
	// remove all ignition key/value pairs the same way they were added
	reader, err := getIgnitionReader(mc)
	if err != nil {
		return err
	}
	parts, err := ginsu.Dice(reader)
	if err != nil {
		return err
	}
	for idx := range parts {
		key := fmt.Sprintf("ignition.config.%d", idx)
		if err := vm.RemoveKeyValuePairNoWait(key); err != nil {
			return err
		}
	}
	return nil
}

func logCommandToFile(c *exec.Cmd, filename string) (*os.File, error) {
	dir, err := env.GetDataDir(define.HyperVVirt)
	if err != nil {
		return nil, fmt.Errorf("obtain machine dir: %w", err)
	}
	path := filepath.Join(dir, filename)
	logrus.Infof("Going to log to %s", path)
	log, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create log file: %w", err)
	}

	c.Stdout = log
	c.Stderr = log

	return log, nil
}

const hyperVVsockNMConnection = `
[connection]
id=vsock0
type=tun
interface-name=vsock0

[tun]
mode=2

[802-3-ethernet]
cloned-mac-address=5A:94:EF:E4:0C:EE

[ipv4]
method=auto

[proxy]
`

func createNetworkUnit(netPort uint64) (string, error) {
	netUnit := parser.NewUnitFile()
	netUnit.Add("Unit", "Description", "vsock_network")
	netUnit.Add("Unit", "After", "NetworkManager.service")
	netUnit.Add("Service", "ExecStart", fmt.Sprintf("/usr/libexec/podman/gvforwarder -preexisting -iface vsock0 -url vsock://2:%d/connect", netPort))
	netUnit.Add("Service", "ExecStartPost", "/usr/bin/nmcli c up vsock0")
	netUnit.Add("Install", "WantedBy", "multi-user.target")
	return netUnit.ToString()
}

func (h HyperVStubber) GetRosetta(_ *vmconfigs.MachineConfig) (bool, error) {
	return false, nil
}
