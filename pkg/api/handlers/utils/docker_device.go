//go:build !remote

package utils

import (
	"fmt"

	"github.com/moby/moby/api/types/container"
	"tags.cncf.io/container-device-interface/pkg/parser"
)

// DockerDeviceMappingString maps Moby DeviceMapping to a podman --device string.
func DockerDeviceMappingString(dev container.DeviceMapping) string {
	// Inverse of docker/cli parseLinuxDevice (https://github.com/docker/cli/blob/master/cli/command/container/opts.go#L1007-L1039),
	// building the podman --device string that pkg/specgen/generate.ParseDevice expects.

	host := dev.PathOnHost
	ctr := dev.PathInContainer
	perm := dev.CgroupPermissions

	switch {
	case host != "" && parser.IsQualifiedName(host) && (ctr == "" || ctr == host):
		return host
	case host != "" && host == ctr && perm == "":
		return host
	case ctr == "" && perm == "":
		return host
	default:
		if perm == "" {
			perm = "rwm"
		}
		return fmt.Sprintf("%s:%s:%s", host, ctr, perm)
	}
}
