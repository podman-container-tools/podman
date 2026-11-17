//go:build !remote && (linux || freebsd)

package libpod

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/storage"
)

func Test_generateName(t *testing.T) {
	state, _ := getEmptySqliteState(t)

	r := &Runtime{
		state: state,
	}

	// Test that (*Runtime).generateName returns different names
	// if called twice.
	n1, _ := r.generateName()
	n2, _ := r.generateName()
	assert.NotEqual(t, n1, n2)
}

func TestGraphDriver(t *testing.T) {
	t.Run("store not initialized", func(t *testing.T) {
		r := &Runtime{}
		assert.Empty(t, r.GraphDriver())
	})

	// storage.conf does not have to set a driver: c/storage then selects
	// one when the store is opened, and the configured driver name stays
	// empty.  GraphDriver() has to return the driver that is really in use.
	t.Run("no driver configured", func(t *testing.T) {
		tmpDir := t.TempDir()
		opts := storage.StoreOptions{
			GraphRoot: filepath.Join(tmpDir, "root"),
			RunRoot:   filepath.Join(tmpDir, "runroot"),
			// Do not name a driver, but let c/storage select one
			// that works without privileges.
			GraphDriverPriority: []string{"vfs"},
		}
		store, err := storage.GetStore(opts)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = store.Shutdown(true)
		})
		r := &Runtime{store: store, storageConfig: opts}

		require.Empty(t, r.StorageConfig().GraphDriverName, "no driver is configured")
		driver := r.GraphDriver()
		assert.NotEmpty(t, driver)
		assert.Equal(t, store.GraphDriverName(), driver)

		// The per-image data directory is named after the driver in use.
		// This is where the API service stores uploaded manifest artifacts.
		image, err := store.CreateImage("", nil, "", "", nil)
		require.NoError(t, err)
		require.NoError(t, store.SetImageBigData(image.ID, "key", []byte("data"), nil))
		assert.DirExists(t, filepath.Join(r.GraphRoot(), driver+"-images", image.ID))
		assert.NoDirExists(t, filepath.Join(r.GraphRoot(), r.StorageConfig().GraphDriverName+"-images", image.ID))
	})

	t.Run("driver configured", func(t *testing.T) {
		tmpDir := t.TempDir()
		opts := storage.StoreOptions{
			GraphRoot:       filepath.Join(tmpDir, "root"),
			RunRoot:         filepath.Join(tmpDir, "runroot"),
			GraphDriverName: "vfs",
		}
		store, err := storage.GetStore(opts)
		require.NoError(t, err)
		t.Cleanup(func() {
			_, _ = store.Shutdown(true)
		})
		r := &Runtime{store: store, storageConfig: opts}

		assert.Equal(t, "vfs", r.GraphDriver())
	})
}
