//go:build !remote && systemd

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHCUnitName(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		isStartup bool
		bare      bool
		expected  string
	}{
		{
			name:      "uses libpod-healthcheck prefix",
			id:        "a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef",
			isStartup: false,
			bare:      false,
			expected:  `^libpod-healthcheck-a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef-[0-9a-f]+$`,
		},
		{
			name:      "adds startup suffix",
			id:        "a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef",
			isStartup: true,
			bare:      false,
			expected:  `^libpod-healthcheck-a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef-startup-[0-9a-f]+$`,
		},
		{
			name:      "omits random suffix when bare is true",
			id:        "a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef",
			isStartup: false,
			bare:      true,
			expected:  `^libpod-healthcheck-a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef$`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctr := &Container{
				config: &ContainerConfig{},
			}

			ctr.config.ID = tt.id

			assert.Regexp(t, tt.expected, ctr.hcUnitName(tt.isStartup, tt.bare))
		})
	}
}
