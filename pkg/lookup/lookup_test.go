package lookup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetContainerGroups(t *testing.T) {
	// The group file shipped in the container's rootfs.
	containerMount := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(containerMount, "etc"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(containerMount, "etc", "group"), []byte("rootfs:x:100:\n"), 0o644))

	// The group file Podman generates and bind mounts onto /etc/group. It is a path on the
	// host, outside of the container mount point.
	bindMounted := filepath.Join(t.TempDir(), "group")
	require.NoError(t, os.WriteFile(bindMounted, []byte("bindmounted:x:200:\n"), 0o644))

	// Without an override, groups are looked up in the container's rootfs.
	gids, err := GetContainerGroups([]string{"rootfs"}, containerMount, nil)
	require.NoError(t, err)
	assert.Equal(t, []uint32{100}, gids)

	// An override is an already resolved host path and must be used as such, the same way
	// GetUserGroupInfo uses it. Joining it onto the container mount point would produce a
	// path which does not exist, and the lookup would fail.
	gids, err = GetContainerGroups([]string{"bindmounted"}, containerMount, &Overrides{ContainerEtcGroupPath: bindMounted})
	require.NoError(t, err)
	assert.Equal(t, []uint32{200}, gids)

	// The override replaces the rootfs group file, it does not add to it.
	_, err = GetContainerGroups([]string{"rootfs"}, containerMount, &Overrides{ContainerEtcGroupPath: bindMounted})
	assert.Error(t, err)
}
