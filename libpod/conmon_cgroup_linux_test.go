//go:build !remote

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.podman.io/common/libnetwork/pasta"
	"go.podman.io/common/pkg/config"
)

// newCgroupTestContainer builds the bare minimum a Container needs for the
// conmon cgroup helpers.  Note that mustCreateConmonCgroup() dereferences the
// runtime, so it cannot be left nil.
func newCgroupTestContainer(cfg *ContainerConfig) *Container {
	return &Container{
		config:  cfg,
		state:   &ContainerState{},
		runtime: &Runtime{config: &config.Config{}},
	}
}

func TestMustCreateConmonCgroup(t *testing.T) {
	// $INVOCATION_ID is only looked at when it is set, and the test binary
	// may well run as a systemd service itself.
	t.Setenv("INVOCATION_ID", "")

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
			name: "enabled",
			cfg:  &ContainerConfig{ContainerMiscConfig: ContainerMiscConfig{CgroupsMode: "enabled"}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCgroupTestContainer(tt.cfg)
			assert.Equal(t, tt.want, c.mustCreateConmonCgroup(), "check whether podman owns the conmon cgroup")
		})
	}
}

func TestMustCreateConmonCgroupUnderSystemd(t *testing.T) {
	t.Setenv("INVOCATION_ID", "1234")

	// Running as a systemd service, systemd owns the cgroup, not podman.
	c := newCgroupTestContainer(&ContainerConfig{})
	assert.False(t, c.mustCreateConmonCgroup(), "local podman runs inside the unit cgroup")

	// ... unless this is only the server side of a remote podman, which does
	// not share the client's unit.
	c.runtime.config.Engine.RemoteURI = "unix:///run/podman/podman.sock"
	assert.True(t, c.mustCreateConmonCgroup(), "remote podman still needs its own cgroup")
}

func TestConmonScopeUnit(t *testing.T) {
	const id = "0123456789abcdef"

	tests := []struct {
		name      string
		parent    string
		wantSlice string
	}{
		{
			name:      "no parent",
			parent:    "",
			wantSlice: "",
		},
		{
			name:      "plain slice",
			parent:    "user.slice",
			wantSlice: "user.slice",
		},
		{
			name:      "nested slice keeps only the leaf",
			parent:    "user.slice/user-1000.slice/user@1000.service/user.slice",
			wantSlice: "user.slice",
		},
		{
			name:      "cgroupfs path is passed through",
			parent:    "/libpod_parent",
			wantSlice: "/libpod_parent",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCgroupTestContainer(&ContainerConfig{
				ContainerMiscConfig: ContainerMiscConfig{CgroupParent: tt.parent},
			})
			c.config.ID = id

			slice, unit := c.conmonScopeUnit()
			assert.Equal(t, tt.wantSlice, slice, "check slice")
			assert.Equal(t, "libpod-conmon-"+id+".scope", unit, "check unit name")
		})
	}
}

func TestConmonCgroupfsPath(t *testing.T) {
	c := newCgroupTestContainer(&ContainerConfig{
		ContainerMiscConfig: ContainerMiscConfig{CgroupParent: "/libpod_parent"},
	})
	assert.Equal(t, "/libpod_parent/conmon", c.conmonCgroupfsPath(), "check cgroup path")
}

func TestNetHelperPids(t *testing.T) {
	tests := []struct {
		name            string
		pastaResult     *pasta.SetupResult
		rootlessPortPid int
		want            []int
	}{
		{
			name: "no helpers",
			want: nil,
		},
		{
			name:        "pasta only",
			pastaResult: &pasta.SetupResult{Pid: 42},
			want:        []int{42},
		},
		{
			name:            "rootlessport only",
			rootlessPortPid: 43,
			want:            []int{43},
		},
		{
			name:            "both",
			pastaResult:     &pasta.SetupResult{Pid: 42},
			rootlessPortPid: 43,
			want:            []int{42, 43},
		},
		{
			// The state of a container created by an older podman has no pid.
			name:            "unknown pasta pid is skipped",
			pastaResult:     &pasta.SetupResult{},
			rootlessPortPid: 43,
			want:            []int{43},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCgroupTestContainer(&ContainerConfig{})
			c.pastaResult = tt.pastaResult
			c.rootlessPortPid = tt.rootlessPortPid
			assert.Equal(t, tt.want, c.netHelperPids(), "check network helper pids")
		})
	}
}

func TestMoveToConmonCgroupIsANoop(t *testing.T) {
	// Neither of these may touch cgroups at all, so they are safe to run
	// unprivileged: the point is that they return before trying.
	t.Run("no pids", func(t *testing.T) {
		c := newCgroupTestContainer(&ContainerConfig{})
		created, err := c.moveToConmonCgroup(true)
		assert.False(t, created, "expect no cgroup to have been created")
		assert.NoError(t, err, "expect no error with nothing to move")
	})

	t.Run("cgroups disabled", func(t *testing.T) {
		c := newCgroupTestContainer(&ContainerConfig{
			ContainerMiscConfig: ContainerMiscConfig{CgroupsMode: "disabled"},
		})
		created, err := c.moveToConmonCgroup(true, 1)
		assert.False(t, created, "expect no cgroup to have been created")
		assert.NoError(t, err, "expect no error when podman does not own the cgroup")
	})
}
