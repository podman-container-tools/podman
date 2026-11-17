//go:build linux

package cgroups

import (
	"path/filepath"

	"github.com/opencontainers/cgroups"
)

// memoryStat fills a metrics structure with usage stats for the memory controller.
func memoryStat(ctr *CgroupControl, m *cgroups.Stats) error {
	var err error
	memUsage := cgroups.MemoryStats{}

	var memoryRoot string
	var limitFilename string

	memoryRoot = filepath.Join(cgroupRoot, ctr.config.Path)
	limitFilename = "memory.max"

	// Read memory.current
	current, err := readFileAsUint64(filepath.Join(memoryRoot, "memory.current"))
	if err != nil {
		return err
	}

	// Read inactive_file from memory.stat
	inactiveFile, err := readFileByKeyAsUint64(filepath.Join(memoryRoot, "memory.stat"), "inactive_file")
	if err != nil {
		return err
	}

	// Docker calculation: memory.current - memory.stat['inactive_file']
	memUsage.Usage.Usage = 0
	if inactiveFile < current {
		memUsage.Usage.Usage = current - inactiveFile
	}

	memUsage.Usage.Limit, err = readFileAsUint64(filepath.Join(memoryRoot, limitFilename))
	if err != nil {
		return err
	}

	m.MemoryStats = memUsage
	return nil
}
