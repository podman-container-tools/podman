//go:build !remote

package tunnel

import "go.podman.io/podman/v6/internal/domain/entities"

var _ entities.TestingEngine = &TestingEngine{}
