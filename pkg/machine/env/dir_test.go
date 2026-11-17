package env_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/podman/v6/pkg/machine/env"
)

func TestWithPodmanPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"adds prefix to a bare name", "machine", "podman-machine"},
		{"leaves an already-prefixed name unchanged", "podman-machine", "podman-machine"},
		{"treats any podman-leading name as already prefixed", "podmanx", "podmanx"},
		{"leaves the bare word podman unchanged", "podman", "podman"},
		{"prefixes a name that merely contains podman", "mypodman", "podman-mypodman"},
		{"prefixes an empty name", "", "podman-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, env.WithPodmanPrefix(tt.input))
		})
	}
}
