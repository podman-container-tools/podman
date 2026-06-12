package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecResourceLimits_AllFields(t *testing.T) {
	memLimit := int64(512 * 1024 * 1024)
	limits := &ExecResourceLimits{
		CPUQuota:   50000,
		CPUPeriod:  100000,
		Memory:     &memLimit,
		CPUSetCPUs: "0-3",
	}

	assert.Equal(t, int64(50000), limits.CPUQuota)
	assert.Equal(t, uint64(100000), limits.CPUPeriod)
	assert.NotNil(t, limits.Memory)
	assert.Equal(t, int64(512*1024*1024), *limits.Memory)
	assert.Equal(t, "0-3", limits.CPUSetCPUs)
}

func TestExecResourceLimits_EmptyLimits(t *testing.T) {
	limits := &ExecResourceLimits{}

	assert.Equal(t, int64(0), limits.CPUQuota)
	assert.Equal(t, uint64(0), limits.CPUPeriod)
	assert.Nil(t, limits.Memory)
	assert.Empty(t, limits.CPUSetCPUs)
}

func TestExecResourceLimits_NilMemory(t *testing.T) {
	limits := &ExecResourceLimits{
		CPUQuota:   30000,
		CPUPeriod:  100000,
		Memory:     nil,
		CPUSetCPUs: "",
	}

	assert.Nil(t, limits.Memory)
}

func TestExecResourceLimits_MemoryPointer(t *testing.T) {
	memLimit := int64(1024 * 1024 * 1024)
	limits := &ExecResourceLimits{
		Memory: &memLimit,
	}

	assert.NotNil(t, limits.Memory)
	assert.Equal(t, int64(1024*1024*1024), *limits.Memory)

	// Verify we can modify through pointer
	newLimit := int64(2 * 1024 * 1024 * 1024)
	limits.Memory = &newLimit
	assert.Equal(t, int64(2*1024*1024*1024), *limits.Memory)
}

func TestExecOptions_WithResources(t *testing.T) {
	memLimit := int64(256 * 1024 * 1024)
	opts := ExecOptions{
		Cmd:         []string{"/bin/sh", "-c", "echo test"},
		Interactive: true,
		Tty:         true,
		Resources: &ExecResourceLimits{
			CPUQuota:   25000,
			CPUPeriod:  100000,
			Memory:     &memLimit,
			CPUSetCPUs: "0,1",
		},
	}

	assert.NotNil(t, opts.Resources)
	assert.Equal(t, int64(25000), opts.Resources.CPUQuota)
	assert.NotNil(t, opts.Resources.Memory)
	assert.Equal(t, int64(256*1024*1024), *opts.Resources.Memory)
	assert.Equal(t, "0,1", opts.Resources.CPUSetCPUs)
}

func TestExecOptions_WithoutResources(t *testing.T) {
	opts := ExecOptions{
		Cmd:         []string{"/bin/sh"},
		Interactive: true,
		Resources:   nil,
	}

	assert.Nil(t, opts.Resources)
}

func TestExecResourceLimits_CPUQuotaRanges(t *testing.T) {
	tests := []struct {
		name  string
		quota int64
	}{
		{"1% of one CPU", 1000},
		{"10% of one CPU", 10000},
		{"50% of one CPU", 50000},
		{"100% of one CPU", 100000},
		{"150% (1.5 CPUs)", 150000},
		{"400% (4 CPUs)", 400000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := &ExecResourceLimits{
				CPUQuota: tt.quota,
			}
			assert.Equal(t, tt.quota, limits.CPUQuota)
		})
	}
}

func TestExecResourceLimits_CPUPeriodRanges(t *testing.T) {
	tests := []struct {
		name   string
		period uint64
	}{
		{"1ms minimum", 1000},
		{"10ms", 10000},
		{"100ms default", 100000},
		{"1s maximum", 1000000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := &ExecResourceLimits{
				CPUPeriod: tt.period,
			}
			assert.Equal(t, tt.period, limits.CPUPeriod)
		})
	}
}

func TestExecResourceLimits_MemoryRanges(t *testing.T) {
	tests := []struct {
		name   string
		memory int64
	}{
		{"1MB", 1024 * 1024},
		{"64MB", 64 * 1024 * 1024},
		{"256MB", 256 * 1024 * 1024},
		{"512MB", 512 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"4GB", 4 * 1024 * 1024 * 1024},
		{"16GB", 16 * 1024 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := &ExecResourceLimits{
				Memory: &tt.memory,
			}
			assert.NotNil(t, limits.Memory)
			assert.Equal(t, tt.memory, *limits.Memory)
		})
	}
}

func TestExecResourceLimits_CPUSetFormats(t *testing.T) {
	tests := []struct {
		name   string
		cpuset string
	}{
		{"single CPU", "0"},
		{"list", "0,1,2,3"},
		{"range", "0-7"},
		{"mixed", "0-3,6,8-11"},
		{"sparse", "0,2,4,6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := &ExecResourceLimits{
				CPUSetCPUs: tt.cpuset,
			}
			assert.Equal(t, tt.cpuset, limits.CPUSetCPUs)
		})
	}
}

func TestExecResourceLimits_Combinations(t *testing.T) {
	tests := []struct {
		name       string
		cpuQuota   int64
		cpuPeriod  uint64
		memory     *int64
		cpusetCpus string
	}{
		{
			name:       "CPU quota only",
			cpuQuota:   50000,
			cpuPeriod:  0,
			memory:     nil,
			cpusetCpus: "",
		},
		{
			name:       "Memory only",
			cpuQuota:   0,
			cpuPeriod:  0,
			memory:     func() *int64 { m := int64(512 * 1024 * 1024); return &m }(),
			cpusetCpus: "",
		},
		{
			name:       "CPUSet only",
			cpuQuota:   0,
			cpuPeriod:  0,
			memory:     nil,
			cpusetCpus: "0-3",
		},
		{
			name:       "CPU quota and memory",
			cpuQuota:   25000,
			cpuPeriod:  100000,
			memory:     func() *int64 { m := int64(256 * 1024 * 1024); return &m }(),
			cpusetCpus: "",
		},
		{
			name:       "All limits",
			cpuQuota:   30000,
			cpuPeriod:  100000,
			memory:     func() *int64 { m := int64(1024 * 1024 * 1024); return &m }(),
			cpusetCpus: "0-7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := &ExecResourceLimits{
				CPUQuota:   tt.cpuQuota,
				CPUPeriod:  tt.cpuPeriod,
				Memory:     tt.memory,
				CPUSetCPUs: tt.cpusetCpus,
			}

			assert.Equal(t, tt.cpuQuota, limits.CPUQuota)
			assert.Equal(t, tt.cpuPeriod, limits.CPUPeriod)
			if tt.memory != nil {
				assert.NotNil(t, limits.Memory)
				assert.Equal(t, *tt.memory, *limits.Memory)
			} else {
				assert.Nil(t, limits.Memory)
			}
			assert.Equal(t, tt.cpusetCpus, limits.CPUSetCPUs)
		})
	}
}

func TestExecResourceLimits_MultiCoreCPU(t *testing.T) {
	// Test that we can represent multi-core CPU limits
	limits := &ExecResourceLimits{
		CPUQuota:  400000,  // 4x the period
		CPUPeriod: 100000,  // = 4 cores
	}

	assert.Equal(t, int64(400000), limits.CPUQuota)
	assert.Equal(t, uint64(100000), limits.CPUPeriod)
}

func TestExecResourceLimits_NegativeMemoryNotAllowed(t *testing.T) {
	// Negative memory should be caught at parsing level (units.RAMInBytes)
	// This test documents that the struct itself can hold the value,
	// but validation happens earlier
	negMem := int64(-512)
	limits := &ExecResourceLimits{
		Memory: &negMem,
	}

	// The struct allows it, but CLI validation should prevent it
	assert.NotNil(t, limits.Memory)
	assert.Equal(t, int64(-512), *limits.Memory)
}
