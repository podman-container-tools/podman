//go:build !remote && (linux || freebsd)

package generate

import (
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/libpod"
	"go.podman.io/podman/v6/pkg/specgen"
)

// TestApplyInfraInheritMountOptionsDoNotLeak verifies that mount options from
// one mount do not leak into another when calling applyInfraInherit.
func TestApplyInfraInheritMountOptionsDoNotLeak(t *testing.T) {
	compatibleOptions := &libpod.InfraInherit{
		Mounts: []spec.Mount{
			{Destination: "/mylog", Source: "/a", Type: "bind"},
			{Destination: "/mytmp", Source: "/b", Type: "bind", Options: []string{"ro"}},
		},
	}

	s := &specgen.SpecGenerator{}
	s.Mounts = []spec.Mount{
		{Destination: "/mytmp", Source: "/b", Type: "bind", Options: []string{"ro"}},
		{Destination: "/mylog", Source: "/a", Type: "bind"},
	}

	err := applyInfraInherit(compatibleOptions, s)
	require.NoError(t, err)

	for _, m := range s.Mounts {
		if m.Destination == "/mylog" {
			assert.Empty(t, m.Options,
				"/mylog should have no options; ro from /mytmp leaked")
		}
		if m.Destination == "/mytmp" {
			assert.Equal(t, []string{"ro"}, m.Options,
				"/mytmp should keep its ro option")
		}
	}
}

// TestApplyInfraInheritDoesNotOverwriteSeccomp verifies that applyInfraInherit
// does not overwrite a pre-set SeccompProfilePath when the infra container has
// no seccomp profile (empty string).
func TestApplyInfraInheritDoesNotOverwriteSeccomp(t *testing.T) {
	compatibleOptions := &libpod.InfraInherit{}

	s := &specgen.SpecGenerator{}
	s.SeccompProfilePath = "localhost/seccomp.json"

	err := applyInfraInherit(compatibleOptions, s)
	require.NoError(t, err)

	assert.Equal(t, "localhost/seccomp.json", s.SeccompProfilePath,
		"SeccompProfilePath should not be overwritten by empty infra value")
}
