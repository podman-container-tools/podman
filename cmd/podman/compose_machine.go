//go:build amd64 || arm64

package main

import (
	"net/url"

	"go.podman.io/podman/v6/internal/localapi"
)

func getMachineConn(connectionURI string, parsedConnection *url.URL) (string, error) {
	mc, machineProvider, err := localapi.FindMachineByPort(connectionURI, parsedConnection)
	if err != nil {
		return "", err
	}

	podmanSocket, podmanPipe, err := mc.ConnectionInfo(machineProvider.VMType())
	if err != nil {
		return "", err
	}
	return extractConnectionString(podmanSocket, podmanPipe)
}
