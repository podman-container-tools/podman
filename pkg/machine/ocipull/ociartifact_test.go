package ocipull

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/podman/v6/pkg/machine/define"
)

func Test_extractKindAndCompression(t *testing.T) {
	type args struct {
		name string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "qcow2",
			args: args{name: "foo.qcow2.xz"},
			want: ".qcow2.xz",
		},
		{
			name: "vhdx",
			args: args{name: "foo.vhdx.zip"},
			want: ".vhdx.zip",
		},
		{
			name: "applehv",
			args: args{name: "foo.raw.gz"},
			want: ".raw.gz",
		},
		{
			name: "lots of extensions with type and compression",
			args: args{name: "foo.bar.homer.simpson.qcow2.xz"},
			want: ".qcow2.xz",
		},
		{
			name: "lots of extensions",
			args: args{name: "foo.bar.homer.simpson"},
			want: ".homer.simpson",
		},
		{
			name: "no extensions",
			args: args{name: "foobar"},
			want: "",
		},
		{
			name: "one extension",
			args: args{name: "foobar.zip"},
			want: ".zip",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractKindAndCompression(tt.args.name); got != tt.want {
				t.Errorf("extractKindAndCompression() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPullCreatesParentDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	parentCacheDir := filepath.Join(tmpDir, "containers", "podman", "machine", "wsl", "cache")
	destPath := filepath.Join(parentCacheDir, "24672f841cbf90346f87c9e4f33b2c99f9c54d090a65a46f8f4e6e1be5e469a3")
	destVMFile, err := define.NewMachineFile(destPath, nil)
	require.NoError(t, err)

	_, err = os.Stat(parentCacheDir)
	require.True(t, os.IsNotExist(err), "parent directory must not exist before pull()")

	dummySrcRef, err := layout.ParseReference(filepath.Join(tmpDir, "dummy_source"))
	require.NoError(t, err)

	pullErr := pull(context.Background(), dummySrcRef, destVMFile, &pullOptions{quiet: true})

	parentInfo, statErr := os.Stat(parentCacheDir)
	require.NoError(t, statErr, "pull() must create the missing parent directory")
	require.True(t, parentInfo.IsDir())

	// copy.Image will fail reading dummySrcRef, but execution must reach it rather than failing early in destination layout.ParseReference
	require.Error(t, pullErr)
	require.Contains(t, pullErr.Error(), "pulling source image:")
	require.NotContains(t, pullErr.Error(), destPath)
}
