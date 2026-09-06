//go:build linux && !remote && seccomp

package generate

import (
	"os"
	"path/filepath"
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/specgen"
)

const testSeccompProfile = `{
	"defaultAction": "SCMP_ACT_ALLOW",
	"syscalls": [
		{
			"names": ["link", "linkat"],
			"action": "SCMP_ACT_ERRNO"
		}
	]
}`

func TestGetSeccompConfigFromFile(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "seccomp.json")
	require.NoError(t, os.WriteFile(profilePath, []byte(testSeccompProfile), 0o600))

	s := specgen.NewSpecGenerator("", false)
	s.SeccompProfilePath = profilePath

	config, err := getSeccompConfig(s, &spec.Spec{}, nil)
	require.NoError(t, err)
	assert.Equal(t, spec.ActAllow, config.DefaultAction)
	require.Len(t, config.Syscalls, 1)
	assert.Equal(t, spec.ActErrno, config.Syscalls[0].Action)
	assert.Equal(t, []string{"link", "linkat"}, config.Syscalls[0].Names)
}

func TestGetSeccompConfigInline(t *testing.T) {
	s := specgen.NewSpecGenerator("", false)
	s.SeccompProfile = testSeccompProfile

	config, err := getSeccompConfig(s, &spec.Spec{}, nil)
	require.NoError(t, err)
	assert.Equal(t, spec.ActAllow, config.DefaultAction)
	require.Len(t, config.Syscalls, 1)
	assert.Equal(t, spec.ActErrno, config.Syscalls[0].Action)
	assert.Equal(t, []string{"link", "linkat"}, config.Syscalls[0].Names)
}

func TestGetSeccompConfigRejectsInvalidInlineProfile(t *testing.T) {
	s := specgen.NewSpecGenerator("", false)
	s.SeccompProfile = "{invalid JSON"

	_, err := getSeccompConfig(s, &spec.Spec{}, nil)
	require.ErrorContains(t, err, "loading inline seccomp profile failed")
}
