//go:build !remote

package libpod

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestSafeMountSubPathAfterReplacement(t *testing.T) {
	parent := t.TempDir()
	volumeRoot := filepath.Join(parent, "volume")
	subPath := filepath.Join(volumeRoot, "sub")
	outside := filepath.Join(parent, "outside")
	require.NoError(t, os.MkdirAll(subPath, 0o755))
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subPath, "probe"), []byte("inside"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "probe"), []byte("outside"), 0o644))

	container := &Container{state: &ContainerState{RunDir: t.TempDir()}}
	mount, err := container.safeMountSubPath(volumeRoot, "sub")
	if errors.Is(err, unix.EPERM) {
		t.Skip("bind mounts are not permitted in this test environment")
	}
	require.NoError(t, err)
	defer mount.Close()
	root := mount.mountPoint

	require.NoError(t, os.Rename(subPath, filepath.Join(volumeRoot, "moved")))
	require.NoError(t, os.Symlink(outside, subPath))

	content, err := os.ReadFile(filepath.Join(root, "probe"))
	require.NoError(t, err)
	require.Equal(t, "inside", string(content))

	require.NoError(t, os.WriteFile(filepath.Join(root, "written"), []byte("x"), 0o644))
	content, err = os.ReadFile(filepath.Join(volumeRoot, "moved", "written"))
	require.NoError(t, err)
	require.Equal(t, "x", string(content))
	_, err = os.Stat(filepath.Join(outside, "written"))
	require.ErrorIs(t, err, os.ErrNotExist)
}
