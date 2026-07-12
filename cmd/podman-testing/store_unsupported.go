//go:build !(linux || freebsd) || remote

package main

import (
	"go.podman.io/podman/v6/pkg/domain/entities"
)

var engineMode = entities.TunnelMode

func storeBefore() error {
	return nil
}

func storeAfter() error {
	return nil
}

func testingEngineBefore(_ *entities.PodmanConfig) (err error) {
	return nil
}
