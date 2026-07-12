//go:build windows

package hypervctl

import (
	"time"
)

type vmState uint16

const (
	// Changes the state to 'Running'.
	start vmState = 2
	// Stops the job temporarily. The intention is to subsequently restart the job with 'Start'. It might be possible to
	// enter the 'Service' state while suspended. (This is job-specific.)
	suspend vmState = 3 //nolint: unused
	// Stops the job cleanly, saves data, preserves the state, and shuts down all underlying processes'
	// in an orderly manner.
	terminate vmState = 4 //nolint: unused
	//Terminates the job immediately with no requirement to save data or preserve the state.
	kill vmState = 5 //nolint: unused
)

type EnabledState uint16

const (
	// Unknown The state of the element could not be determined.
	Unknown EnabledState = 0
	// Other No description
	Other EnabledState = 1
	// Enabled The element is running.
	Enabled EnabledState = 2
	// Disabled The element is turned off.
	Disabled EnabledState = 3
	// ShuttingDown Shutting_Down The element is in the process of going to a Disabled state.
	ShuttingDown EnabledState = 4
	// NotApplicable The element does not support being enabled or disabled.
	NotApplicable EnabledState = 5
	// EnabledButOffline The element might be completing commands, and it will drop any new requests.
	EnabledButOffline EnabledState = 6
	// InTest The element is in a test state.
	InTest EnabledState = 7
	// Deferred The element might be completing commands, but it will queue any new requests.
	Deferred EnabledState = 8
	// Quiesce The element is enabled but in a restricted mode. The behavior of the element is similar to the Enabled state
	//(2), but it processes only a restricted set of commands. All other requests are queued.
	Quiesce EnabledState = 9
	// Starting The element is in the process of going to an Enabled state (2). New requests are queued.
	Starting EnabledState = 10
)

func (es EnabledState) String() string {
	switch es {
	case Unknown:
		return "unknown"
	case Other:
		return "other"
	case Enabled:
		return "enabled"
	case Disabled:
		return "disabled"
	case ShuttingDown:
		return "shutting down"
	case NotApplicable:
		return "not applicable"
	case EnabledButOffline:
		return "enabled but offline"
	case InTest:
		return "in test"
	case Deferred:
		return "deferred"
	case Quiesce:
		return "quiesce"
	}
	return "starting"
}

func (es EnabledState) equal(s uint16) bool {
	return es == EnabledState(s)
}

// HyperVConfig describes physical attributes of the machine
type HyperVConfig struct {
	// Hardware configuration for HyperV
	Hardware HardwareConfig
	// state and descriptive info
	Status Statuses
}

type HardwareConfig struct {
	// CPUs to be assigned to the VM
	CPUs uint16
	// Diskpath is fully qualified location of the
	// bootable disk image
	DiskPath string
	// Disk size in gigabytes assigned to the vm
	DiskSize uint64
	// Memory in megabytes assigned to the vm
	Memory uint64
	// Network is bool to add a Network Connection to the
	// default network switch in Microsoft HyperV
	Network bool
	// DVDDiskPath is the path to the disk image
	// that will be used as a DVD drive in the VM (e.g. for cloud-init)
	DVDDiskPath string
}

type Statuses struct {
	// time vm created
	Created time.Time
	// last time vm ran
	LastUp time.Time
	// is vm running
	Running bool
	// is vm starting up
	Starting bool
	// vm state
	State EnabledState
}
