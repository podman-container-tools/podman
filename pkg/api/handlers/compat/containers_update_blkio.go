//go:build !remote && (linux || freebsd)

package compat

import (
	"errors"
	"fmt"

	"github.com/moby/moby/api/types/blkiodev"
	"github.com/moby/moby/api/types/container"
	spec "github.com/opencontainers/runtime-spec/specs-go"
)

func updateBlockIO(options *container.UpdateConfig, resolve func(string) (int64, int64, error)) (*spec.LinuxBlockIO, error) {
	update := new(spec.LinuxBlockIO)
	changed := false
	if options.BlkioWeight != 0 {
		update.Weight = &options.BlkioWeight
		changed = true
	}
	if options.BlkioWeightDevice != nil {
		update.WeightDevice = make([]spec.LinuxWeightDevice, 0, len(options.BlkioWeightDevice))
		for _, device := range options.BlkioWeightDevice {
			if device == nil || device.Weight < 10 || device.Weight > 1000 {
				return nil, errors.New("BlkioWeightDevice entries must have a weight between 10 and 1000")
			}
			major, minor, err := resolve(device.Path)
			if err != nil {
				return nil, fmt.Errorf("BlkioWeightDevice: %w", err)
			}
			update.WeightDevice = append(update.WeightDevice, spec.LinuxWeightDevice{
				LinuxBlockIODevice: spec.LinuxBlockIODevice{Major: major, Minor: minor},
				Weight:             &device.Weight,
			})
		}
		changed = true
	}
	for _, list := range []struct {
		name        string
		devices     []*blkiodev.ThrottleDevice
		destination *[]spec.LinuxThrottleDevice
	}{
		{"BlkioDeviceReadBps", options.BlkioDeviceReadBps, &update.ThrottleReadBpsDevice},
		{"BlkioDeviceWriteBps", options.BlkioDeviceWriteBps, &update.ThrottleWriteBpsDevice},
		{"BlkioDeviceReadIOps", options.BlkioDeviceReadIOps, &update.ThrottleReadIOPSDevice},
		{"BlkioDeviceWriteIOps", options.BlkioDeviceWriteIOps, &update.ThrottleWriteIOPSDevice},
	} {
		if list.devices == nil {
			continue
		}
		devices := make([]spec.LinuxThrottleDevice, 0, len(list.devices))
		for _, device := range list.devices {
			if device == nil || device.Rate == 0 {
				return nil, fmt.Errorf("%s entries must have a positive rate", list.name)
			}
			major, minor, err := resolve(device.Path)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", list.name, err)
			}
			devices = append(devices, spec.LinuxThrottleDevice{
				LinuxBlockIODevice: spec.LinuxBlockIODevice{Major: major, Minor: minor},
				Rate:               device.Rate,
			})
		}
		*list.destination = devices
		changed = true
	}
	if !changed {
		return nil, nil
	}
	return update, nil
}
