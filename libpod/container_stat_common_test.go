//go:build !remote && (linux || freebsd)

package libpod

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageMountLinkTarget(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "dir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "plain"), []byte("plain"), 0o644))
	require.NoError(t, os.Symlink("plain", filepath.Join(root, "relative")))
	require.NoError(t, os.Symlink("../plain", filepath.Join(root, "dir", "relative")))
	require.NoError(t, os.Symlink("/etc/os-release", filepath.Join(root, "absolute")))
	require.NoError(t, os.Symlink("dir", filepath.Join(root, "directory-link")))
	require.NoError(t, os.Symlink("missing", filepath.Join(root, "dangling")))

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "mount root", path: "/data"},
		{name: "regular file", path: "/data/plain"},
		{name: "missing path", path: "/data/missing/file"},
		{name: "relative link", path: "/data/relative", want: "/data/plain"},
		{name: "nested relative link", path: "/data/dir/relative", want: "/data/plain"},
		{name: "absolute link", path: "/data/absolute", want: "/etc/os-release"},
		{name: "link in parent", path: "/data/directory-link/child", want: "/data/dir/child"},
		{name: "dangling link", path: "/data/dangling", want: "/data/missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, err := imageMountLinkTarget(root, "/data", tt.path)
			require.NoError(t, err)
			require.Equal(t, tt.want, target)
		})
	}
}
