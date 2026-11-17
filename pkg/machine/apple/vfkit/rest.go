//go:build darwin

package vfkit

import (
	"errors"
	"fmt"

	"go.podman.io/podman/v6/pkg/machine/define"
)

type Endpoint string

// VZMachineState is what the restful service in vfkit will return
type VZMachineState string

const (
	// Values that the machine can be in
	// "VirtualMachineStateStoppedVirtualMachineStateRunningVirtualMachineStatePausedVirtualMachineStateErrorVirtualMachineStateStartingVirtualMachineStatePausingVirtualMachineStateResumingVirtualMachineStateStopping"
	VZMachineStateStopped  VZMachineState = "VirtualMachineStateStopped"
	VZMachineStateRunning  VZMachineState = "VirtualMachineStateRunning"
	VZMachineStatePaused   VZMachineState = "VirtualMachineStatePaused"
	VZMachineStateError    VZMachineState = "VirtualMachineStateError"
	VZMachineStateStarting VZMachineState = "VirtualMachineStateStarting"
	VZMachineStatePausing  VZMachineState = "VirtualMachineStatePausing"
	VZMachineStateResuming VZMachineState = "VirtualMachineStateResuming"
	VZMachineStateStopping VZMachineState = "VirtualMachineStateStopping"
)

func ToMachineStatus(val string) (define.Status, error) {
	switch val {
	case string(VZMachineStateRunning), string(VZMachineStatePausing), string(VZMachineStateResuming), string(VZMachineStateStopping), string(VZMachineStatePaused):
		return define.Running, nil
	case string(VZMachineStateStopped):
		return define.Stopped, nil
	case string(VZMachineStateStarting):
		return define.Starting, nil
	case string(VZMachineStateError):
		return "", errors.New("machine is in error state")
	}
	return "", fmt.Errorf("unknown machine state: %s", val)
}
