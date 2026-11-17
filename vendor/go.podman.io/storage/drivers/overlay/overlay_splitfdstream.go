//go:build linux

package overlay

import (
	"errors"
	"fmt"
	"io"
	"os"

	"go.podman.io/storage/internal/driver"
	"go.podman.io/storage/pkg/chunked/dump"
	"go.podman.io/storage/pkg/fileutils"
	"golang.org/x/sys/unix"
)

// LayerFiles implements driver.SplitDirFDStreamDriver.
func (d *Driver) LayerFiles(id string, bigData func(key string) (io.ReadCloser, error)) (driver.LayerFiles, error) {
	dir := d.dir(id)
	if err := fileutils.Exists(dir); err != nil {
		return nil, fmt.Errorf("layer %s does not exist: %w", id, err)
	}

	// Only a layer applied for composefs is stored in the flat layout, with
	// its files under names its table of contents maps them to; any other
	// layer, including one pulled partially without composefs, keeps its
	// files under their own names.
	var flatNames map[string]string
	if err := fileutils.Exists(d.getComposefsData(id)); err != nil {
		if !errors.Is(err, unix.ENOENT) {
			return nil, err
		}
	} else {
		flatNames, err = dump.LayerFlatFileNames(bigData)
		if err != nil {
			return nil, fmt.Errorf("reading the table of contents of layer %s: %w", id, err)
		}
	}

	diffPath, err := d.getDiffPath(id)
	if err != nil {
		return nil, fmt.Errorf("getting diff path of layer %s: %w", id, err)
	}
	diffDir, err := os.Open(diffPath)
	if err != nil {
		return nil, fmt.Errorf("opening diff directory: %w", err)
	}
	return driver.NewLayerFiles(diffDir, flatNames), nil
}
