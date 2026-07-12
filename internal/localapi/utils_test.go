//go:build linux || darwin || freebsd

package localapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func TestIsPathAvailableOnMachine_Unix(t *testing.T) {
	tests := []struct {
		name           string
		mounts         []*vmconfigs.Mount
		vmType         define.VMType
		localPath      string
		wantRemotePath string
		wantFound      bool
	}{
		{
			name: "QEMU - Path within mount",
			mounts: []*vmconfigs.Mount{
				{
					Source: "/home/test",
					Target: "/home/user",
				},
			},
			vmType:         define.QemuVirt,
			localPath:      "/home/test/project/file.txt",
			wantRemotePath: "/home/user/project/file.txt",
			wantFound:      true,
		},
		{
			name: "QEMU - Path exactly at mount point",
			mounts: []*vmconfigs.Mount{
				{
					Source: "/home/test",
					Target: "/mnt/host",
				},
			},
			vmType:         define.QemuVirt,
			localPath:      "/home/test",
			wantRemotePath: "/mnt/host",
			wantFound:      true,
		},
		{
			name: "QEMU - Path not in any mount",
			mounts: []*vmconfigs.Mount{
				{
					Source: "/home/test",
					Target: "/mnt/test",
				},
			},
			vmType:    define.QemuVirt,
			localPath: "/home/another/file.txt",
			wantFound: false,
		},
		{
			name: "QEMU - Multiple mounts, path in second",
			mounts: []*vmconfigs.Mount{
				{
					Source: "/home/first",
					Target: "/mnt/first",
				},
				{
					Source: "/home/second",
					Target: "/mnt/second",
				},
			},
			vmType:         define.QemuVirt,
			localPath:      "/home/second/data",
			wantRemotePath: "/mnt/second/data",
			wantFound:      true,
		},
		{
			name: "QEMU - Nested subdirectory in mount",
			mounts: []*vmconfigs.Mount{
				{
					Source: "/home/test",
					Target: "/home/user",
				},
			},
			vmType:         define.QemuVirt,
			localPath:      "/home/test/deep/nested/path/file.go",
			wantRemotePath: "/home/user/deep/nested/path/file.go",
			wantFound:      true,
		},
		{
			name:      "QEMU - Empty mounts list",
			mounts:    []*vmconfigs.Mount{},
			vmType:    define.QemuVirt,
			localPath: "/home/test/file.txt",
			wantFound: false,
		},
		{
			name: "LibKrun - Path within mount",
			mounts: []*vmconfigs.Mount{
				{
					Source: "/Users/developer/projects",
					Target: "/mnt/projects",
				},
			},
			vmType:         define.LibKrun,
			localPath:      "/Users/developer/projects/myapp/src/main.go",
			wantRemotePath: "/mnt/projects/myapp/src/main.go",
			wantFound:      true,
		},
		{
			name: "AppleHV - Path within mount",
			mounts: []*vmconfigs.Mount{
				{
					Source: "/Users/developer",
					Target: "/home/developer",
				},
			},
			vmType:         define.AppleHvVirt,
			localPath:      "/Users/developer/Documents/code",
			wantRemotePath: "/home/developer/Documents/code",
			wantFound:      true,
		},
		{
			name: "Multiple mounts - first match wins",
			mounts: []*vmconfigs.Mount{
				{
					Source: "/home",
					Target: "/mnt/home",
				},
				{
					Source: "/home/user",
					Target: "/mnt/user",
				},
			},
			vmType:         define.QemuVirt,
			localPath:      "/home/user/file.txt",
			wantRemotePath: "/mnt/home/user/file.txt",
			wantFound:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, found := IsPathAvailableOnMachine(tt.mounts, tt.vmType, tt.localPath)

			assert.Equal(t, tt.wantFound, found, "found flag mismatch")

			if tt.wantFound {
				require.NotNil(t, result, "result should not be nil when found is true")
				assert.Equal(t, tt.wantRemotePath, result.RemotePath, "RemotePath mismatch")
			} else {
				assert.Nil(t, result, "result should be nil when found is false")
			}
		})
	}
}
