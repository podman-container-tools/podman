// Package config provides native go data types to describe a VM configuration
// (memory, CPUs, bootloader, devices, ...).
// It's used by vfkit which generates a VirtualMachine instance after parsing
// its command line using FromOptions().
// It can also be used by application writers who want to start a VM with
// vfkit. After creating a VirtualMachine instance with the needed devices,
// calling VirtualMachine.Cmd() will return an exec.Cmd which can be used
// to start the virtual machine.
//
// This package does not use Code-Hex/vz directly as it must possible to
// cross-compile code using it.
package config

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"go.podman.io/common/pkg/strongunits"
)

// VirtualMachine is the top-level type. It describes the virtual machine
// configuration (bootloader, devices, ...).
type VirtualMachine struct {
	Vcpus      uint           `json:"vcpus"`
	Memory     strongunits.B  `json:"memoryBytes"`
	Bootloader Bootloader     `json:"bootloader"`
	Devices    []VirtioDevice `json:"devices,omitempty"`
	Timesync   *TimeSync      `json:"timesync,omitempty"`
	Ignition   *Ignition      `json:"ignition,omitempty"`
	Nested     bool           `json:"nested,omitempty"`
}

// TimeSync enables synchronization of the host time to the linux guest after the host was suspended.
// This requires qemu-guest-agent to be running in the guest, and to be listening on a vsock socket
type TimeSync struct {
	VsockPort uint32 `json:"vsockPort"`
}

type Ignition struct {
	ConfigPath string `json:"configPath"`
	SocketPath string `json:"socketPath,omitempty"`
	VsockPort  uint32 `json:"-"`
}

// The VMComponent interface represents a VM element (device, bootloader, ...)
// which can be converted from/to commandline parameters
type VMComponent interface {
	FromOptions([]option) error
	ToCmdLine() ([]string, error)
}

const (
	// the ignition vsock port is hardcoded to 1024 in ignition source code:
	// https://github.com/coreos/ignition/blob/d4ff84b2c28a28ad828b974befe3575563faacdd/internal/providers/applehv/applehv.go#L59-L68
	ignitionVsockPort uint32 = 1024
)

// NewVirtualMachine creates a new VirtualMachine instance. The virtual machine
// will use vcpus virtual CPUs and it will be allocated memoryMiB mibibytes
// (1024*1024 bytes) of RAM. bootloader specifies how the virtual machine will
// be booted (UEFI or with the specified kernel/initrd/commandline)
func NewVirtualMachine(vcpus uint, memoryMiB uint64, bootloader Bootloader) *VirtualMachine {
	return &VirtualMachine{
		Vcpus:      vcpus,
		Memory:     strongunits.MiB(memoryMiB).ToBytes(),
		Bootloader: bootloader,
	}
}

// round value up to the nearest mibibyte multiple
func roundToMiB(value strongunits.StorageUnits) strongunits.MiB {
	mib := uint64(strongunits.MiB(1).ToBytes())
	valueB := strongunits.B(uint64(value.ToBytes()) + mib - 1)
	return strongunits.ToMib(valueB)
}

// ToCmdLine generates a list of arguments for use with the [os/exec] package.
// These arguments will start a virtual machine with the devices/bootloader/...
// described by vm If the virtual machine configuration described by vm is
// invalid, an error will be returned.
func (vm *VirtualMachine) ToCmdLine() ([]string, error) {
	// TODO: missing binary name/path
	args := []string{}

	if vm.Vcpus != 0 {
		args = append(args, "--cpus", strconv.FormatUint(uint64(vm.Vcpus), 10))
	}
	if uint64(vm.Memory.ToBytes()) != 0 {
		args = append(args, "--memory", strconv.FormatUint(uint64(roundToMiB(vm.Memory)), 10))
	}

	if vm.Bootloader == nil {
		return nil, fmt.Errorf("missing bootloader configuration")
	}
	bootloaderArgs, err := vm.Bootloader.ToCmdLine()
	if err != nil {
		return nil, err
	}
	args = append(args, bootloaderArgs...)

	for _, dev := range vm.Devices {
		devArgs, err := dev.ToCmdLine()
		if err != nil {
			return nil, err
		}
		args = append(args, devArgs...)
	}

	if vm.Ignition != nil {
		args = append(args, "--ignition", vm.Ignition.ConfigPath)
	}

	if vm.Nested {
		args = append(args, "--nested")
	}

	return args, nil
}

func (vm *VirtualMachine) extraFiles() []*os.File {
	extraFiles := []*os.File{}
	for _, dev := range vm.Devices {
		virtioNet, ok := dev.(*VirtioNet)
		if !ok {
			continue
		}
		if virtioNet.Socket != nil {
			extraFiles = append(extraFiles, virtioNet.Socket)
		}
	}

	return extraFiles
}

// Cmd creates an exec.Cmd to start vfkit with the configured devices.
// In particular it will set ExtraFiles appropriately when mapping
// a file with a network interface.
func (vm *VirtualMachine) Cmd(vfkitPath string) (*exec.Cmd, error) {
	args, err := vm.ToCmdLine()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(vfkitPath, args...)
	cmd.ExtraFiles = vm.extraFiles()

	return cmd, nil
}

func (vm *VirtualMachine) AddDevicesFromCmdLine(cmdlineOpts []string) error {
	for _, deviceOpts := range cmdlineOpts {
		dev, err := deviceFromCmdLine(deviceOpts)
		if err != nil {
			return err
		}
		vm.Devices = append(vm.Devices, dev)
	}
	return nil
}

func FilterDevices[V VMComponent](vm *VirtualMachine) []V {
	devs := []V{}
	for _, dev := range vm.Devices {
		if dev, isV := dev.(V); isV {
			devs = append(devs, dev)
		}
	}
	return devs
}

func (vm *VirtualMachine) VirtioGPUDevices() []*VirtioGPU {
	return FilterDevices[*VirtioGPU](vm)
}

func (vm *VirtualMachine) VirtioVsockDevices() []*VirtioVsock {
	return FilterDevices[*VirtioVsock](vm)
}

func (vm *VirtualMachine) VirtioInputDevices() []*VirtioInput {
	return FilterDevices[*VirtioInput](vm)
}

func (vm *VirtualMachine) VirtioNetDevices() []*VirtioNet {
	return FilterDevices[*VirtioNet](vm)
}

func (vm *VirtualMachine) NetworkBlockDevice(deviceID string) *NetworkBlockDevice {
	for _, dev := range vm.Devices {
		if nbdDev, isNbdDev := dev.(*NetworkBlockDevice); isNbdDev && nbdDev.DeviceIdentifier == deviceID {
			return nbdDev
		}
	}

	return nil
}

// AddDevice adds a dev to vm. This device can be created with one of the
// VirtioXXXNew methods.
func (vm *VirtualMachine) AddDevice(dev VirtioDevice) error {
	return vm.AddDevices(dev)
}

// AddDevices adds a list of devices to vm.
func (vm *VirtualMachine) AddDevices(dev ...VirtioDevice) error {
	vm.Devices = append(vm.Devices, dev...)
	return nil
}

func (vm *VirtualMachine) AddTimeSyncFromCmdLine(cmdlineOpts string) error {
	if cmdlineOpts == "" {
		return nil
	}
	timesync, err := timesyncFromCmdLine(cmdlineOpts)
	if err != nil {
		return err
	}
	vm.Timesync = timesync

	return nil
}

func (vm *VirtualMachine) TimeSync() *TimeSync {
	return vm.Timesync
}

func IgnitionNew(configPath string, _ string) (*Ignition, error) {
	if configPath == "" {
		return nil, fmt.Errorf("config path cannot be empty")
	}
	return &Ignition{
		ConfigPath: configPath,
		VsockPort:  ignitionVsockPort,
	}, nil
}

func (vm *VirtualMachine) AddIgnitionFileFromCmdLine(cmdlineOpts string) error {
	if cmdlineOpts == "" {
		return nil
	}
	opts := strings.Split(cmdlineOpts, ",")
	if len(opts) != 1 {
		return fmt.Errorf("ignition only accepts one option in command line argument")
	}

	ignition, err := IgnitionNew(opts[0], "")
	if err != nil {
		return err
	}
	vm.Ignition = ignition
	return nil
}

func TimeSyncNew(vsockPort uint) (VMComponent, error) {

	if vsockPort > math.MaxUint32 {
		return nil, fmt.Errorf("invalid vsock port: %d", vsockPort)
	}
	return &TimeSync{
		VsockPort: uint32(vsockPort),
	}, nil
}

func (ts *TimeSync) ToCmdLine() ([]string, error) {
	args := []string{}
	if ts.VsockPort != 0 {
		args = append(args, fmt.Sprintf("vsockPort=%d", ts.VsockPort))
	}
	return []string{"--timesync", strings.Join(args, ",")}, nil
}

func (ts *TimeSync) FromOptions(options []option) error {
	for _, option := range options {
		switch option.key {
		case "vsockPort":
			vsockPort, err := strconv.ParseUint(option.value, 10, 32)
			if err != nil {
				return err
			}
			ts.VsockPort = uint32(vsockPort)
		default:
			return fmt.Errorf("unknown option for timesync parameter: %s", option.key)
		}
	}

	if ts.VsockPort == 0 {
		return fmt.Errorf("missing 'vsockPort' option for timesync parameter")
	}

	return nil
}

func timesyncFromCmdLine(optsStr string) (*TimeSync, error) {
	var timesync TimeSync

	optsStrv := strings.Split(optsStr, ",")
	options := strvToOptions(optsStrv)

	if err := timesync.FromOptions(options); err != nil {
		return nil, err
	}

	return &timesync, nil
}
