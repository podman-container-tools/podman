//go:build !remote && (linux || freebsd)

package libpod

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/pkg/config"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/libpod/lock"
)

// Regression test for issue #27858.
func TestVolumeMountPointReturnsCorrectPath(t *testing.T) {
	t.Parallel()

	runtimeConfig := &config.Config{
		Engine: config.EngineConfig{
			VolumePlugins: map[string]string{},
		},
	}
	runtime := &Runtime{config: runtimeConfig}

	testCases := []struct {
		name           string
		driver         string
		configMount    string
		stateMount     string
		expectedResult string
	}{
		{
			name:           "local driver uses config mountpoint",
			driver:         define.VolumeDriverLocal,
			configMount:    "/var/lib/containers/storage/volumes/testvol/_data",
			stateMount:     "",
			expectedResult: "/var/lib/containers/storage/volumes/testvol/_data",
		},
		{
			name:           "empty driver uses config mountpoint",
			driver:         "",
			configMount:    "/var/lib/containers/storage/volumes/testvol/_data",
			stateMount:     "",
			expectedResult: "/var/lib/containers/storage/volumes/testvol/_data",
		},
		{
			name:           "plugin driver uses state mountpoint",
			driver:         "volume-fs",
			configMount:    "",
			stateMount:     "/run/containers/storage/volumes/plugin-vol",
			expectedResult: "/run/containers/storage/volumes/plugin-vol",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			vol := &Volume{
				config: &VolumeConfig{
					Name:       "test-volume",
					Driver:     tc.driver,
					MountPoint: tc.configMount,
				},
				state: &VolumeState{
					MountPoint: tc.stateMount,
				},
				runtime: runtime,
			}

			result := vol.mountPoint()
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

// Regression test for issue #27858: plugin volumes must use state.MountPoint.
func TestVolumeMountPointPluginConsistency(t *testing.T) {
	t.Parallel()

	runtimeConfig := &config.Config{
		Engine: config.EngineConfig{
			VolumePlugins: map[string]string{},
		},
	}
	runtime := &Runtime{config: runtimeConfig}

	vol := &Volume{
		config: &VolumeConfig{
			Name:       "plugin-volume",
			Driver:     "volume-fs",
			MountPoint: "",
		},
		state: &VolumeState{
			MountPoint: "/run/containers/storage/volumes/plugin-vol",
		},
		runtime: runtime,
	}

	assert.True(t, vol.UsesVolumeDriver())
	result := vol.mountPoint()
	assert.NotEmpty(t, result)
	assert.Equal(t, vol.state.MountPoint, result)
}

func TestVolumeLocalDriverDoesNotUseVolumeDriver(t *testing.T) {
	t.Parallel()

	runtimeConfig := &config.Config{
		Engine: config.EngineConfig{
			VolumePlugins: map[string]string{},
		},
	}
	runtime := &Runtime{config: runtimeConfig}

	vol := &Volume{
		config: &VolumeConfig{
			Name:       "local-volume",
			Driver:     define.VolumeDriverLocal,
			MountPoint: "/var/lib/containers/storage/volumes/local-vol/_data",
		},
		state: &VolumeState{
			MountPoint: "",
		},
		runtime: runtime,
	}

	assert.False(t, vol.UsesVolumeDriver())
	result := vol.mountPoint()
	assert.Equal(t, vol.config.MountPoint, result)
}

type dummyReadCloser struct {
	closed bool
}

func (d *dummyReadCloser) Read(_ []byte) (n int, err error) {
	return 0, io.EOF
}

func (d *dummyReadCloser) Close() error {
	d.closed = true
	return nil
}

func TestVolumeExportReadCloserDelaysUnmount(t *testing.T) {
	t.Parallel()

	lockManager, err := lock.NewInMemoryManager(16)
	require.NoError(t, err)
	volLock, err := lockManager.AllocateLock()
	require.NoError(t, err)

	dummy := &dummyReadCloser{}
	vol := &Volume{
		config: &VolumeConfig{
			Name:    "export-test-vol",
			Driver:  define.VolumeDriverLocal,
			Options: map[string]string{"type": "tmpfs"},
		},
		state: &VolumeState{
			MountCount: 1,
		},
		lock: volLock,
	}

	wrapper := &volumeExportReadCloser{
		ReadCloser: dummy,
		vol:        vol,
	}

	assert.Equal(t, uint(1), vol.state.MountCount)
	assert.False(t, dummy.closed)

	closeErr := wrapper.Close()
	assert.Error(t, closeErr)
	assert.True(t, dummy.closed)
	assert.Equal(t, uint(0), vol.state.MountCount)
}
