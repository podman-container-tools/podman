//go:build !remote

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/pkg/config"
)

// newCgroupTestContainer builds the bare minimum container the conmon cgroup
// helpers need.  They only look at the config and at the runtime config, so
// none of the usual container plumbing has to be set up.
func newCgroupTestContainer(cfg *ContainerConfig) *Container {
	return &Container{
		config: cfg,
		state:  &ContainerState{},
		runtime: &Runtime{
			config: &config.Config{},
		},
	}
}

func TestMustCreateConmonCgroup(t *testing.T) {
	tests := []struct {
		name string
		cfg  *ContainerConfig
		want bool
	}{
		{
			name: "default",
			cfg:  &ContainerConfig{},
			want: true,
		},
		{
			name: "no cgroups",
			cfg:  &ContainerConfig{ContainerMiscConfig: ContainerMiscConfig{NoCgroups: true}},
			want: false,
		},
		{
			name: "cgroups disabled",
			cfg:  &ContainerConfig{ContainerMiscConfig: ContainerMiscConfig{CgroupsMode: "disabled"}},
			want: false,
		},
		{
			name: "no conmon cgroup",
			cfg:  &ContainerConfig{ContainerMiscConfig: ContainerMiscConfig{CgroupsMode: "no-conmon"}},
			want: false,
		},
		{
			name: "split cgroups",
			cfg:  &ContainerConfig{ContainerMiscConfig: ContainerMiscConfig{CgroupsMode: cgroupSplit}},
			want: false,
		},
		{
			name: "enabled cgroups",
			cfg:  &ContainerConfig{ContainerMiscConfig: ContainerMiscConfig{CgroupsMode: "enabled"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCgroupTestContainer(tt.cfg)
			assert.Equal(t, tt.want, c.mustCreateConmonCgroup())
		})
	}
}

func TestMustCreateConmonCgroupUnderSystemd(t *testing.T) {
	// $INVOCATION_ID is set by systemd when running as a service, the unit
	// already owns the cgroup then.
	t.Setenv("INVOCATION_ID", "1234")

	c := newCgroupTestContainer(&ContainerConfig{})
	assert.False(t, c.mustCreateConmonCgroup(), "local podman runs inside the unit cgroup")

	// A remote client is not the process systemd started, so the check does
	// not apply to it.
	c.runtime.config.Engine.RemoteURI = "unix:///run/podman/podman.sock"
	assert.True(t, c.mustCreateConmonCgroup(), "remote podman still needs the cgroup")
}

func TestConmonScopeUnit(t *testing.T) {
	const id = "0123456789abcdef"

	tests := []struct {
		name         string
		cgroupParent string
		wantSlice    string
	}{
		{
			name:         "systemd slice",
			cgroupParent: "user.slice/user-1000.slice/user@1000.service/user.slice",
			wantSlice:    "user.slice",
		},
		{
			name:         "machine slice",
			cgroupParent: "machine.slice",
			wantSlice:    "machine.slice",
		},
		{
			name:         "cgroupfs path",
			cgroupParent: "/libpod_parent",
			wantSlice:    "/libpod_parent",
		},
		{
			name:         "empty",
			cgroupParent: "",
			wantSlice:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCgroupTestContainer(&ContainerConfig{ID: id})
			c.config.CgroupParent = tt.cgroupParent

			slice, unitName := c.conmonScopeUnit()
			assert.Equal(t, tt.wantSlice, slice, "check slice")
			assert.Equal(t, "libpod-conmon-"+id+".scope", unitName, "check unit name")
		})
	}
}

func TestConmonCgroupfsPath(t *testing.T) {
	c := newCgroupTestContainer(&ContainerConfig{})
	c.config.CgroupParent = "/libpod_parent"
	assert.Equal(t, "/libpod_parent/conmon", c.conmonCgroupfsPath())
}

func TestMoveToConmonCgroupIsANoop(t *testing.T) {
	// Without a cgroup to create there is nothing to do, and in particular
	// nothing that could fail in a test environment.
	for _, mode := range []string{"disabled", "no-conmon", cgroupSplit} {
		t.Run(mode, func(t *testing.T) {
			c := newCgroupTestContainer(&ContainerConfig{ContainerMiscConfig: ContainerMiscConfig{CgroupsMode: mode}})
			require.NoError(t, c.moveToConmonCgroup(1))
		})
	}
}
