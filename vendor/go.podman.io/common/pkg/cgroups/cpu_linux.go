//go:build linux

package cgroups

import (
	"strconv"

	"github.com/opencontainers/cgroups"
)

// cpuStat fills a metrics structure with usage stats for the cpu controller.
func cpuStat(ctr *CgroupControl, m *cgroups.Stats) error {
	cpu := cgroups.CpuStats{}
	values, err := readCgroup2MapFile(ctr, "cpu.stat")
	if err != nil {
		return err
	}
	if val, found := values["usage_usec"]; found {
		cpu.CpuUsage.TotalUsage, err = strconv.ParseUint(cleanString(val[0]), 10, 64)
		if err != nil {
			return err
		}
		cpu.CpuUsage.TotalUsage *= 1000
	}
	if val, found := values["system_usec"]; found {
		cpu.CpuUsage.UsageInKernelmode, err = strconv.ParseUint(cleanString(val[0]), 10, 64)
		if err != nil {
			return err
		}
		cpu.CpuUsage.UsageInKernelmode *= 1000
	}
	m.CpuStats = cpu
	return nil
}
