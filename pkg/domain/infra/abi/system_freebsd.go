//go:build !remote

package abi

import (
	"context"
)

// Default path for system runtime state
const defaultRunPath = "/var/run"

// SetupRootless in a NOP for freebsd as it only configures the rootless userns on linux.
func (ic *ContainerEngine) SetupRootless(_ context.Context, _ bool, _ string) error {
	return nil
}
