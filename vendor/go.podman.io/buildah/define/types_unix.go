//go:build darwin || linux

package define

import (
	"github.com/opencontainers/cgroups/devices/config"
)

// BuildahDevice is a wrapper around devices.Device
// with additional support for renaming a device
// using bind-mount in rootless environments.
type BuildahDevice struct {
	config.Device
	Source      string
	Destination string
}

type ContainerDevices = []BuildahDevice
