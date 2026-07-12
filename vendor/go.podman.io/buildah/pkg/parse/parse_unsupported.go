//go:build !linux && !darwin

package parse //nolint:revive,nolintlint

import (
	"errors"

	"go.podman.io/buildah/define"
)

func getDefaultProcessLimits() []string {
	return []string{}
}

func DeviceFromPath(device string) (define.ContainerDevices, error) {
	return nil, errors.New("devices not supported")
}
