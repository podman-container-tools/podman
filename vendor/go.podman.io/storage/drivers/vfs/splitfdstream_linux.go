//go:build linux

package vfs

import (
	"fmt"
	"io"
	"os"

	graphdriver "go.podman.io/storage/drivers"
	"go.podman.io/storage/internal/driver"
)

// LayerFiles implements driver.SplitDirFDStreamDriver.  VFS stores files
// under their own names, so it has no use for the table of contents.
func (d *Driver) LayerFiles(id string, _ func(key string) (io.ReadCloser, error)) (driver.LayerFiles, error) {
	dir, err := d.Get(id, graphdriver.MountOpts{})
	if err != nil {
		return nil, fmt.Errorf("getting directory of layer %s: %w", id, err)
	}
	dirFile, err := os.Open(dir)
	if err != nil {
		return nil, fmt.Errorf("opening layer directory: %w", err)
	}
	return driver.NewLayerFiles(dirFile, nil), nil
}
