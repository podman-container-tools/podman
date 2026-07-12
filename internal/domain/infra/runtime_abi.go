//go:build !remote && (linux || freebsd)

package infra

import (
	"context"
	"fmt"

	ientities "go.podman.io/podman/v6/internal/domain/entities"
	"go.podman.io/podman/v6/internal/domain/infra/tunnel"
	"go.podman.io/podman/v6/pkg/bindings"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// NewTestingEngine factory provides a libpod runtime for testing-specific operations
func NewTestingEngine(facts *entities.PodmanConfig) (ientities.TestingEngine, error) {
	switch facts.EngineMode {
	case entities.ABIMode:
		r, err := NewLibpodTestingRuntime(facts.FlagSet, facts)
		return r, err
	case entities.TunnelMode:
		ctx, err := bindings.NewConnectionWithOptions(context.Background(), bindings.Options{
			URI:         facts.URI,
			Identity:    facts.Identity,
			TLSCertFile: facts.TLSCertFile,
			TLSKeyFile:  facts.TLSKeyFile,
			TLSCAFile:   facts.TLSCAFile,
			Machine:     facts.MachineMode,
		})
		return &tunnel.TestingEngine{ClientCtx: ctx}, err
	}
	return nil, fmt.Errorf("runtime mode '%v' is not supported", facts.EngineMode)
}
