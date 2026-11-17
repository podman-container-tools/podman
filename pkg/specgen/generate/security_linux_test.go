//go:build !remote

package generate

import (
	"testing"

	"github.com/opencontainers/runtime-tools/generate"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/pkg/specgen"
)

func TestSetupCapabilities(t *testing.T) {
	rtc, err := config.Default()
	require.NoError(t, err)

	tests := []struct {
		name     string
		capAdd   []string
		labels   map[string]string
		expected []string
	}{
		{
			name:     "no capabilities configured",
			labels:   nil,
			expected: rtc.Containers.DefaultCapabilities.Get(),
		},
		{
			name:     "image label only",
			labels:   map[string]string{"io.containers.capabilities": "setuid,setgid"},
			expected: []string{"CAP_SETGID", "CAP_SETUID"},
		},
		{
			name:     "image label with capability added by the user",
			capAdd:   []string{"sys_admin"},
			labels:   map[string]string{"io.containers.capabilities": "setuid,setgid"},
			expected: []string{"CAP_SETGID", "CAP_SETUID", "CAP_SYS_ADMIN"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := &specgen.SpecGenerator{}
			s.CapAdd = test.capAdd
			s.Labels = test.labels
			g, err := generate.New("linux")
			require.NoError(t, err)

			require.NoError(t, setupCapabilities(s, &g, rtc))
			assert.Equal(t, test.expected, g.Config.Process.Capabilities.Bounding)
		})
	}
}
