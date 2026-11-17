//go:build linux

package cgroups

import (
	"path/filepath"

	"github.com/opencontainers/cgroups"
)

// pidsStat fills a metrics structure with usage stats for the pids controller.
func pidsStat(ctr *CgroupControl, m *cgroups.Stats) error {
	if ctr.config.Path == "" {
		// nothing we can do to retrieve the pids.current path
		return nil
	}

	PIDRoot := filepath.Join(cgroupRoot, ctr.config.Path)

	current, err := readFileAsUint64(filepath.Join(PIDRoot, "pids.current"))
	if err != nil {
		return err
	}

	m.PidsStats.Current = current
	return nil
}
