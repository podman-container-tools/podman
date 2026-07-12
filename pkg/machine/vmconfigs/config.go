package vmconfigs

import (
	"time"

	gvproxy "github.com/containers/gvisor-tap-vsock/pkg/types"
	"go.podman.io/common/pkg/strongunits"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/ignition"
	"go.podman.io/storage/pkg/lockfile"
)

const MachineConfigVersion = 1

type MachineConfig struct {
	// Common stuff
	Created  time.Time
	GvProxy  gvproxy.GvproxyCommand
	HostUser HostUser

	LastUp time.Time

	Mounts []*Mount
	Name   string

	Resources ResourceConfig
	SSH       SSHConfig
	Version   uint

	Swap strongunits.MiB

	ImagePath *define.VMFile // Temporary only until a proper image struct is worked out

	// Provider stuff
	AppleHypervisor   *AppleHVConfig `json:",omitempty"`
	HyperVHypervisor  *HyperVConfig  `json:",omitempty"`
	LibKrunHypervisor *LibKrunConfig `json:",omitempty"`
	QEMUHypervisor    *QEMUConfig    `json:",omitempty"`
	WSLHypervisor     *WSLConfig     `json:",omitempty"`

	lock *lockfile.LockFile

	// configPath can be used for reading, writing, removing
	configPath *define.VMFile

	// used for deriving file, socket, etc locations
	dirs *define.MachineDirs

	// State

	// Starting is defined as "on" but not fully booted
	Starting bool

	Rosetta bool

	Ansible *AnsibleConfig

	ImportNativeCA bool
}

type VMProvider interface { //nolint:interfacebloat
	CreateVM(opts define.CreateVMOpts, mc *MachineConfig, builder *ignition.IgnitionBuilder) error
	PrepareIgnition(mc *MachineConfig, ignBuilder *ignition.IgnitionBuilder) (*ignition.ReadyUnitOpts, error)
	Exists(name string) (bool, error)
	MountType() VolumeMountType
	MountVolumesToVM(mc *MachineConfig, quiet bool) error
	Remove(mc *MachineConfig) ([]string, func() error, error)
	RemoveAndCleanMachines(dirs *define.MachineDirs) error
	SetProviderAttrs(mc *MachineConfig, opts define.SetOptions) error
	StartNetworking(mc *MachineConfig, cmd *gvproxy.GvproxyCommand) error
	PostStartNetworking(mc *MachineConfig, noInfo bool) error
	StartVM(mc *MachineConfig) (func() error, func() error, error)
	State(mc *MachineConfig, bypass bool) (define.Status, error)
	StopVM(mc *MachineConfig, hardStop bool) error
	StopHostNetworking(mc *MachineConfig, vmType define.VMType) error
	VMType() define.VMType
	UserModeNetworkEnabled(mc *MachineConfig) bool
	UseProviderNetworkSetup() bool
	RequireExclusiveActive() bool
	UpdateSSHPort(mc *MachineConfig, port int) error
	GetRosetta(mc *MachineConfig) (bool, error)
}

// HostUser describes the host user
type HostUser struct {
	// Whether this machine should run in a rootful or rootless manner
	Rootful bool
	// UID is the numerical id of the user that called machine
	UID int
	// Whether one of these fields has changed and actions should be taken
	Modified bool `json:"HostUserModified"`
}

type Mount struct {
	OriginalInput string
	ReadOnly      bool
	Source        string
	Tag           string
	Target        string
	Type          string
	VSockNumber   *uint64
}

// ResourceConfig describes physical attributes of the machine
type ResourceConfig struct {
	// CPUs to be assigned to the VM
	CPUs uint64
	// Disk size in gigabytes assigned to the vm
	DiskSize strongunits.GiB
	// Memory in megabytes assigned to the vm
	Memory strongunits.MiB
	// Usbs
	USBs []define.USBConfig
}

// SSHConfig contains remote access information for SSH
type SSHConfig struct {
	// IdentityPath is the fq path to the ssh priv key
	IdentityPath string
	// SSH port for user networking
	Port int
	// RemoteUsername of the vm user
	RemoteUsername string
}

type VMStats struct {
	// Created contains the original created time instead of querying the file mod time
	Created time.Time
	// LastUp contains the last recorded uptime
	LastUp time.Time
}

type AnsibleConfig struct {
	PlaybookPath string
	Contents     string
	User         string
}
