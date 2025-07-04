//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	manifest "go.podman.io/image/v5/manifest"
	"go.podman.io/podman/v6/libpod/define"
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

func TestHealthCheckStatusChanged(t *testing.T) {
	checks := []struct {
		name     string
		previous define.HealthCheckResults
		current  define.HealthCheckResults
		want     bool
	}{
		{
			name:     "first result after start",
			previous: define.HealthCheckResults{Status: define.HealthCheckStarting},
			current:  define.HealthCheckResults{Status: define.HealthCheckStarting, Log: []define.HealthCheckLog{{ExitCode: 1}}},
			want:     true,
		},
		{
			name:     "repeated starting result",
			previous: define.HealthCheckResults{Status: define.HealthCheckStarting, Log: []define.HealthCheckLog{{ExitCode: 1}}},
			current:  define.HealthCheckResults{Status: define.HealthCheckStarting, Log: []define.HealthCheckLog{{ExitCode: 1}, {ExitCode: 1}}},
			want:     false,
		},
		{
			name:     "starting to unhealthy",
			previous: define.HealthCheckResults{Status: define.HealthCheckStarting, Log: []define.HealthCheckLog{{ExitCode: 1}}},
			current:  define.HealthCheckResults{Status: define.HealthCheckUnhealthy, Log: []define.HealthCheckLog{{ExitCode: 1}, {ExitCode: 1}}},
			want:     true,
		},
		{
			name:     "repeated unhealthy result",
			previous: define.HealthCheckResults{Status: define.HealthCheckUnhealthy, Log: []define.HealthCheckLog{{ExitCode: 1}}},
			current:  define.HealthCheckResults{Status: define.HealthCheckUnhealthy, Log: []define.HealthCheckLog{{ExitCode: 1}, {ExitCode: 1}}},
			want:     false,
		},
		{
			name:     "unhealthy to healthy",
			previous: define.HealthCheckResults{Status: define.HealthCheckUnhealthy, Log: []define.HealthCheckLog{{ExitCode: 1}}},
			current:  define.HealthCheckResults{Status: define.HealthCheckHealthy, Log: []define.HealthCheckLog{{ExitCode: 1}, {ExitCode: 0}}},
			want:     true,
		},
		{
			name:     "first result after reset",
			previous: define.HealthCheckResults{Status: define.HealthCheckReset},
			current:  define.HealthCheckResults{Status: define.HealthCheckStarting, Log: []define.HealthCheckLog{{ExitCode: 1}}},
			want:     true,
		},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			assert.Equal(t, check.want, healthCheckStatusChanged(check.previous, check.current))
		})
	}
}
