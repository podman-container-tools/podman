//go:build (amd64 || arm64) && !windows

package main

import (
	"errors"

	"go.podman.io/podman/v6/pkg/machine/define"
)

func extractConnectionString(podmanSocket *define.VMFile, _ *define.VMFile) (string, error) {
	if podmanSocket == nil {
		return "", errors.New("socket of machine is not set")
	}
	return "unix://" + podmanSocket.Path, nil
}
