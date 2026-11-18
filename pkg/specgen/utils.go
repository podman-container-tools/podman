//go:build !linux

package specgen

import "errors"

// BlockDeviceNumbers is only supported on Linux.
func BlockDeviceNumbers(_ string) (int64, int64, error) {
	return 0, 0, errors.New("block device I/O limits are only supported on Linux")
}

// FinishThrottleDevices cannot be called on non-linux OS' due to importing unix functions
func FinishThrottleDevices(_ *SpecGenerator) error {
	return nil
}

// WeightDevices cannot be called on non-linux OS' due to importing unix functions
func WeightDevices(_ *SpecGenerator) error {
	return nil
}
