//go:build windows

package localapi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/pkg/machine/vmconfigs"
)

func TestIsPathAvailableOnMachine_Windows(t *testing.T) {
	tests := []struct {
		name           string
		mounts         []*vmconfigs.Mount
		vmType         define.VMType
		localPath      string
		wantRemotePath string
		wantFound      bool
	}{
		{
			name:           "WSL - Windows C drive path",
			mounts:         nil, // WSL doesn't use mounts
			vmType:         define.WSLVirt,
			localPath:      "C:\\Users\\test\\project",
			wantRemotePath: "/mnt/c/Users/test/project",
			wantFound:      true,
		},
		{
			name:           "WSL - Windows D drive path",
			mounts:         nil,
			vmType:         define.WSLVirt,
			localPath:      "D:\\data\\workspace",
			wantRemotePath: "/mnt/d/data/workspace",
			wantFound:      true,
		},
		{
			name: "HyperV - Path within mount",
			mounts: []*vmconfigs.Mount{
				{
					Source: "C:\\Users\\test",
					Target: "/home/user",
				},
			},
			vmType:         define.HyperVVirt,
			localPath:      "C:\\Users\\test\\project\\file.txt",
			wantRemotePath: "/home/user/project/file.txt",
			wantFound:      true,
		},
		{
			name: "HyperV - D drive path",
			mounts: []*vmconfigs.Mount{
				{
					Source: "D:\\Users\\test",
					Target: "/home/user",
				},
			},
			vmType:         define.HyperVVirt,
			localPath:      "D:\\Users\\test\\project\\file.txt",
			wantRemotePath: "/home/user/project/file.txt",
			wantFound:      true,
		},
		{
			name: "HyperV - Path exactly at mount point",
			mounts: []*vmconfigs.Mount{
				{
					Source: "C:\\Users\\test",
					Target: "/mnt/host",
				},
			},
			vmType:         define.HyperVVirt,
			localPath:      "C:\\Users\\test",
			wantRemotePath: "/mnt/host",
			wantFound:      true,
		},
		{
			name: "HyperV - Path not in any mount",
			mounts: []*vmconfigs.Mount{
				{
					Source: "C:\\Users\\test",
					Target: "/mnt/test",
				},
			},
			vmType:    define.HyperVVirt,
			localPath: "C:\\Users\\another\\file.txt",
			wantFound: false,
		},
		{
			name: "HyperV - Path in different drive",
			mounts: []*vmconfigs.Mount{
				{
					Source: "C:\\Users\\test",
					Target: "/mnt/test",
				},
			},
			vmType:    define.HyperVVirt,
			localPath: "D:\\Users\\test\\file.txt",
			wantFound: false,
		},
		{
			name: "HyperV - Multiple mounts, path in second",
			mounts: []*vmconfigs.Mount{
				{
					Source: "C:\\Users\\first",
					Target: "/mnt/first",
				},
				{
					Source: "C:\\Users\\second",
					Target: "/mnt/second",
				},
			},
			vmType:         define.HyperVVirt,
			localPath:      "C:\\Users\\second\\data",
			wantRemotePath: "/mnt/second/data",
			wantFound:      true,
		},
		{
			name: "HyperV - Nested subdirectory in mount",
			mounts: []*vmconfigs.Mount{
				{
					Source: "C:\\Users\\test",
					Target: "/home/user",
				},
			},
			vmType:         define.HyperVVirt,
			localPath:      "C:\\Users\\test\\deep\\nested\\path\\file.go",
			wantRemotePath: "/home/user/deep/nested/path/file.go",
			wantFound:      true,
		},
		{
			name:      "HyperV - Empty mounts list",
			mounts:    []*vmconfigs.Mount{},
			vmType:    define.HyperVVirt,
			localPath: "C:\\Users\\test\\file.txt",
			wantFound: false,
		},
		{
			name: "HyperV - Path with mixed case drive letter",
			mounts: []*vmconfigs.Mount{
				{
					Source: "C:\\Projects",
					Target: "/home/projects",
				},
			},
			vmType:         define.HyperVVirt,
			localPath:      "c:\\Projects\\myapp",
			wantRemotePath: "/home/projects/myapp",
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
