//go:build !remote

package utils

import (
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/stretchr/testify/assert"
	"tags.cncf.io/container-device-interface/pkg/parser"
)

func TestDockerDeviceMappingString(t *testing.T) {
	tests := []struct {
		name string
		dev  container.DeviceMapping
		want string
	}{
		{
			name: "compose style CDI single token",
			dev: container.DeviceMapping{
				PathOnHost:        "nvidia.com/gpu=all",
				PathInContainer:   "",
				CgroupPermissions: "",
			},
			want: "nvidia.com/gpu=all",
		},
		{
			name: "CDI host only with cgroup perms still single token",
			dev: container.DeviceMapping{
				PathOnHost:        "nvidia.com/gpu=all",
				PathInContainer:   "",
				CgroupPermissions: "rwm",
			},
			want: "nvidia.com/gpu=all",
		},
		{
			name: "duplicate CDI host and container path",
			dev: container.DeviceMapping{
				PathOnHost:        "nvidia.com/gpu=all",
				PathInContainer:   "nvidia.com/gpu=all",
				CgroupPermissions: "rwm",
			},
			want: "nvidia.com/gpu=all",
		},
		{
			name: "duplicate CDI host and container empty cgroup",
			dev: container.DeviceMapping{
				PathOnHost:        "nvidia.com/gpu=all",
				PathInContainer:   "nvidia.com/gpu=all",
				CgroupPermissions: "",
			},
			want: "nvidia.com/gpu=all",
		},
		{
			name: "classic bind triple",
			dev: container.DeviceMapping{
				PathOnHost:        "/dev/null",
				PathInContainer:   "/dev/zero",
				CgroupPermissions: "rwm",
			},
			want: "/dev/null:/dev/zero:rwm",
		},
		{
			name: "bind different paths empty cgroup defaults rwm",
			dev: container.DeviceMapping{
				PathOnHost:        "/dev/null",
				PathInContainer:   "/dev/zero",
				CgroupPermissions: "",
			},
			want: "/dev/null:/dev/zero:rwm",
		},
		{
			name: "same bind path non-CDI empty cgroup",
			dev: container.DeviceMapping{
				PathOnHost:        "/dev/null",
				PathInContainer:   "/dev/null",
				CgroupPermissions: "",
			},
			want: "/dev/null",
		},
		{
			name: "same bind path non-CDI explicit cgroup",
			dev: container.DeviceMapping{
				PathOnHost:        "/dev/null",
				PathInContainer:   "/dev/null",
				CgroupPermissions: "r",
			},
			want: "/dev/null:/dev/null:r",
		},
		{
			name: "host path only empty container cgroup",
			dev: container.DeviceMapping{
				PathOnHost:        "/dev/foo",
				PathInContainer:   "",
				CgroupPermissions: "",
			},
			want: "/dev/foo",
		},
		{
			name: "host path only with cgroup uses host::perm form",
			dev: container.DeviceMapping{
				PathOnHost:        "/dev/foo",
				PathInContainer:   "",
				CgroupPermissions: "rwm",
			},
			want: "/dev/foo::rwm",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DockerDeviceMappingString(tt.dev)
			assert.Equal(t, tt.want, got)
			if parser.IsQualifiedName(tt.want) {
				assert.True(t, parser.IsQualifiedName(got))
			}
		})
	}
}
