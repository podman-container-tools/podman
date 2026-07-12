//go:build !remote

package libpod

import (
	"testing"

	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
)

func TestGenerateUserPasswdEntry(t *testing.T) {
	c := Container{
		config: &ContainerConfig{
			Spec: &spec.Spec{},
			ContainerSecurityConfig: ContainerSecurityConfig{
				User: "123456:456789",
			},
		},
		state: &ContainerState{
			Mountpoint: "/does/not/exist/tmp/",
		},
	}
	user, err := c.generateUserPasswdEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, user, "123456:*:123456:456789:container user:/:/bin/sh\n")

	c.config.User = "567890"
	user, err = c.generateUserPasswdEntry(0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, user, "567890:*:567890:0:container user:/:/bin/sh\n")
}

func TestGenerateUserGroupEntry(t *testing.T) {
	c := Container{
		config: &ContainerConfig{
			Spec: &spec.Spec{},
			ContainerSecurityConfig: ContainerSecurityConfig{
				User: "123456:456789",
			},
		},
		state: &ContainerState{
			Mountpoint: "/does/not/exist/tmp/",
		},
	}
	group, err := c.generateUserGroupEntry(-1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, group, "456789:x:456789:123456\n")

	c.config.User = "567890"
	group, err = c.generateUserGroupEntry(-1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, group, "0:x:0:567890\n")
}

func TestParseCgroupPath(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    string
		wantErr bool
	}{
		{
			name: "simple cgroup v2",
			data: "0::/user.slice/user-1000.slice/session-1.scope\n",
			want: "/user.slice/user-1000.slice/session-1.scope",
		},
		{
			name: "named cgroup ignored",
			data: "1:name=systemd:/user.slice\n0::/longer/path\n",
			want: "/longer/path",
		},
		{
			name: "longest path wins",
			data: "0::/short\n0::/much/longer/path\n",
			want: "/much/longer/path",
		},
		{
			name: "trailing newline",
			data: "0::/some/path\n",
			want: "/some/path",
		},
		{
			name:    "empty input",
			data:    "",
			wantErr: true,
		},
		{
			name:    "only newlines",
			data:    "\n\n",
			wantErr: true,
		},
		{
			name: "colon in cgroup path from dbus",
			data: "0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-dbus\\x2d:1.2\\x2dorg.gnome.Console.slice/abc123\n",
			want: "/user.slice/user-1000.slice/user@1000.service/app.slice/app-dbus\\x2d:1.2\\x2dorg.gnome.Console.slice/abc123",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCgroupPath([]byte(tt.data))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
