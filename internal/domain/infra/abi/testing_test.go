//go:build !remote && (linux || freebsd)

package abi

import "go.podman.io/podman/v6/internal/domain/entities"

var _ entities.TestingEngine = &TestingEngine{}
