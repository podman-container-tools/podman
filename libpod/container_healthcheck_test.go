//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	manifest "go.podman.io/image/v5/manifest"
)

func TestHasHealthCheckCases(t *testing.T) {
	ctr := &Container{config: &ContainerConfig{}}

	// nil HealthCheckConfig -> false
	ctr.config.HealthCheckConfig = nil
	assert.False(t, ctr.HasHealthCheck(), "nil HealthCheckConfig should not be considered a healthcheck")

	// Test == nil -> false
	ctr.config.HealthCheckConfig = &manifest.Schema2HealthConfig{Test: nil}
	assert.False(t, ctr.HasHealthCheck(), "nil Test slice should not be considered a healthcheck")

	// empty slice -> false
	ctr.config.HealthCheckConfig = &manifest.Schema2HealthConfig{Test: []string{}}
	assert.False(t, ctr.HasHealthCheck(), "empty Test slice should not be considered a healthcheck")

	// NONE sentinel -> false (case-insensitive)
	ctr.config.HealthCheckConfig = &manifest.Schema2HealthConfig{Test: []string{"NONE"}}
	assert.False(t, ctr.HasHealthCheck(), "[\"NONE\"] sentinel should not be considered a healthcheck")
	ctr.config.HealthCheckConfig = &manifest.Schema2HealthConfig{Test: []string{"none"}}
	assert.False(t, ctr.HasHealthCheck(), "[\"none\"] sentinel should not be considered a healthcheck")

	// valid CMD form -> true
	ctr.config.HealthCheckConfig = &manifest.Schema2HealthConfig{Test: []string{"CMD-SHELL", "echo hi"}}
	assert.True(t, ctr.HasHealthCheck(), "non-empty Test with command should be considered a healthcheck")
}
