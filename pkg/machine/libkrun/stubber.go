//go:build darwin && arm64

package libkrun

import (
	"fmt"
	"strconv"

	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	vfConfig "github.com/crc-org/vfkit/pkg/config"
	"go.podman.io/podman/v6/pkg/machine"
	"go.podman.io/podman/v6/pkg/machine/apple"
	"go.podman.io/podman/v6/pkg/machine/apple/vfkit"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/ignition"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
	"go.podman.io/podman/v6/utils"
)

const (
	krunkitBinary = "krunkit"
	localhostURI  = "http://localhost"
)

type LibKrunStubber struct {
	vmconfigs.AppleHVConfig
}

func (l *LibKrunStubber) CreateVM(opts define.CreateVMOpts, mc *vmconfigs.MachineConfig, builder *ignition.IgnitionBuilder) error {
	mc.LibKrunHypervisor = new(vmconfigs.LibKrunConfig)
	mc.LibKrunHypervisor.KRun = vfkit.Helper{}

	bl := vfConfig.NewEFIBootloader(fmt.Sprintf("%s/efi-bl-%s", opts.Dirs.DataDir.GetPath(), opts.Name), true)
	mc.LibKrunHypervisor.KRun.VirtualMachine = vfConfig.NewVirtualMachine(uint(mc.Resources.CPUs), uint64(mc.Resources.Memory), bl)

	randPort, err := utils.GetRandomPort()
	if err != nil {
		return err
	}
	mc.LibKrunHypervisor.KRun.Endpoint = localhostURI + ":" + strconv.Itoa(randPort)

	virtiofsMounts := make([]machine.VirtIoFs, 0, len(mc.Mounts))
	for _, mnt := range mc.Mounts {
		virtiofsMounts = append(virtiofsMounts, machine.MountToVirtIOFs(mnt))
	}

	// Populate the ignition file with virtiofs stuff
	virtIOIgnitionMounts, err := machine.GenerateSystemDFilesForVirtiofsMounts(virtiofsMounts)
	if err != nil {
		return err
	}
	builder.WithUnit(virtIOIgnitionMounts...)

	return apple.ResizeDisk(mc, mc.Resources.DiskSize)
}

func (l *LibKrunStubber) PrepareIgnition(_ *vmconfigs.MachineConfig, _ *ignition.IgnitionBuilder) (*ignition.ReadyUnitOpts, error) {
	return nil, nil
}

func (l *LibKrunStubber) Exists(_ string) (bool, error) {
	// not applicable for libkrun (same as applehv)
	return false, nil
}

func (l *LibKrunStubber) MountType() vmconfigs.VolumeMountType {
	return vmconfigs.VirtIOFS
}

func (l *LibKrunStubber) MountVolumesToVM(_ *vmconfigs.MachineConfig, _ bool) error {
	return nil
}

func (l *LibKrunStubber) Remove(_ *vmconfigs.MachineConfig) ([]string, func() error, error) {
	return []string{}, func() error { return nil }, nil
}

func (l *LibKrunStubber) RemoveAndCleanMachines(_ *define.MachineDirs) error {
	return nil
}

func (l *LibKrunStubber) SetProviderAttrs(mc *vmconfigs.MachineConfig, opts define.SetOptions) error {
	state, err := l.State(mc, false)
	if err != nil {
		return err
	}
	return apple.SetProviderAttrs(mc, opts, state)
}

func (l *LibKrunStubber) StartNetworking(mc *vmconfigs.MachineConfig, cmd *gvproxy.GvproxyCommand) error {
	return apple.StartGenericNetworking(mc, cmd)
}

func (l *LibKrunStubber) PostStartNetworking(_ *vmconfigs.MachineConfig, _ bool) error {
	return nil
}

func (l *LibKrunStubber) StartVM(mc *vmconfigs.MachineConfig) (func() error, func() error, error) {
	bl := mc.LibKrunHypervisor.KRun.VirtualMachine.Bootloader
	if bl == nil {
		return nil, nil, fmt.Errorf("unable to determine boot loader for this machine")
	}
	return apple.StartGenericAppleVM(mc, krunkitBinary, bl, mc.LibKrunHypervisor.KRun.Endpoint)
}

func (l *LibKrunStubber) State(mc *vmconfigs.MachineConfig, _ bool) (define.Status, error) {
	return mc.LibKrunHypervisor.KRun.State()
}

func (l *LibKrunStubber) StopVM(mc *vmconfigs.MachineConfig, hardStop bool) error {
	return mc.LibKrunHypervisor.KRun.Stop(hardStop, true)
}

func (l *LibKrunStubber) StopHostNetworking(_ *vmconfigs.MachineConfig, _ define.VMType) error {
	return nil
}

func (l *LibKrunStubber) VMType() define.VMType {
	return define.LibKrun
}

func (l *LibKrunStubber) UserModeNetworkEnabled(_ *vmconfigs.MachineConfig) bool {
	return true
}

func (l *LibKrunStubber) UseProviderNetworkSetup() bool {
	return false
}

func (l *LibKrunStubber) RequireExclusiveActive() bool {
	return true
}

func (l *LibKrunStubber) UpdateSSHPort(_ *vmconfigs.MachineConfig, _ int) error {
	return nil
}

func (l *LibKrunStubber) GetRosetta(_ *vmconfigs.MachineConfig) (bool, error) {
	return false, nil
}
