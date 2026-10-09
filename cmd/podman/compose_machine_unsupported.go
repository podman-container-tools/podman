//go:build !(amd64 || arm64)

package main

import (
	"context"
	"errors"
	"net/url"
)

func getMachineConn(_ context.Context, connection string, parsedConnection *url.URL) (string, error) {
	return "", errors.New("podman machine not supported on this architecture")
}
