package containers

import (
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func TestMemoryParsing_RAMInBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{
			name:  "bytes without unit",
			input: "1024",
			want:  1024,
		},
		{
			name:  "kilobytes lowercase",
			input: "512k",
			want:  512 * 1024,
		},
		{
			name:  "kilobytes uppercase",
			input: "512K",
			want:  512 * 1024,
		},
		{
			name:  "megabytes lowercase",
			input: "256m",
			want:  256 * 1024 * 1024,
		},
		{
			name:  "megabytes uppercase",
			input: "256M",
			want:  256 * 1024 * 1024,
		},
		{
			name:  "megabytes mb suffix",
			input: "512mb",
			want:  512 * 1024 * 1024,
		},
		{
			name:  "gigabytes lowercase",
			input: "2g",
			want:  2 * 1024 * 1024 * 1024,
		},
		{
			name:  "gigabytes uppercase",
			input: "2G",
			want:  2 * 1024 * 1024 * 1024,
		},
		{
			name:  "gigabytes gb suffix",
			input: "4gb",
			want:  4 * 1024 * 1024 * 1024,
		},
		{
			name:  "bytes with B suffix",
			input: "1024b",
			want:  1024,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "invalid suffix",
			input:   "512x",
			wantErr: true,
		},
		{
			name:    "negative bytes",
			input:   "-1024",
			wantErr: true,
		},
		{
			name:    "negative with unit",
			input:   "-512m",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test using units.RAMInBytes (what we actually use now)
			got, err := units.RAMInBytes(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateAndSetResourceLimits_NoneSpecified(t *testing.T) {
	// Reset global variables
	cpuQuota = 0
	cpuPeriod = 0
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	assert.NoError(t, err)
	assert.Nil(t, execOpts.Resources, "Resources should be nil when no limits specified")
}

func TestValidateAndSetResourceLimits_CPUQuotaOnly(t *testing.T) {
	// Set only CPU quota
	cpuQuota = 50000  // 50% of one CPU
	cpuPeriod = 0     // Should use default
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err)
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, int64(50000), execOpts.Resources.CPUQuota)
	assert.Equal(t, uint64(0), execOpts.Resources.CPUPeriod) // Will use default in cgroup setup
	assert.Nil(t, execOpts.Resources.Memory)
	assert.Empty(t, execOpts.Resources.CPUSetCPUs)
}

func TestValidateAndSetResourceLimits_CPUQuotaAndPeriod(t *testing.T) {
	cpuQuota = 25000
	cpuPeriod = 50000
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err)
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, int64(25000), execOpts.Resources.CPUQuota)
	assert.Equal(t, uint64(50000), execOpts.Resources.CPUPeriod)
}

func TestValidateAndSetResourceLimits_MemoryOnly(t *testing.T) {
	cpuQuota = 0
	cpuPeriod = 0
	memory = "512m"
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err)
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, int64(0), execOpts.Resources.CPUQuota)
	require.NotNil(t, execOpts.Resources.Memory)
	assert.Equal(t, int64(512*1024*1024), *execOpts.Resources.Memory)
}

func TestValidateAndSetResourceLimits_AllLimits(t *testing.T) {
	cpuQuota = 30000
	cpuPeriod = 100000
	memory = "1g"
	cpusetCpus = "0-3"

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err)
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, int64(30000), execOpts.Resources.CPUQuota)
	assert.Equal(t, uint64(100000), execOpts.Resources.CPUPeriod)
	require.NotNil(t, execOpts.Resources.Memory)
	assert.Equal(t, int64(1024*1024*1024), *execOpts.Resources.Memory)
	assert.Equal(t, "0-3", execOpts.Resources.CPUSetCPUs)
}

func TestValidateAndSetResourceLimits_InvalidCPUPeriodTooLow(t *testing.T) {
	cpuQuota = 0
	cpuPeriod = 500 // Less than 1000 (1ms)
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cpu-period must be between 1000")
}

func TestValidateAndSetResourceLimits_InvalidCPUPeriodTooHigh(t *testing.T) {
	cpuQuota = 0
	cpuPeriod = 2000000 // Greater than 1000000 (1s)
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cpu-period must be between 1000")
}

func TestValidateAndSetResourceLimits_InvalidCPUQuotaTooLow(t *testing.T) {
	cpuQuota = 500 // Less than 1000 (1ms)
	cpuPeriod = 0
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cpu-quota must be >= 1000")
}

// Multi-core CPU allocation: quota can exceed period
func TestValidateAndSetResourceLimits_MultiCoreCPU_TwoCores(t *testing.T) {
	cpuQuota = 200000  // 2x the period = 2 cores
	cpuPeriod = 100000
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err, "CPU quota can exceed period for multi-core allocation")
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, int64(200000), execOpts.Resources.CPUQuota)
	assert.Equal(t, uint64(100000), execOpts.Resources.CPUPeriod)
}

func TestValidateAndSetResourceLimits_MultiCoreCPU_FourCores(t *testing.T) {
	cpuQuota = 400000  // 4x the period = 4 cores
	cpuPeriod = 100000
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err, "CPU quota can exceed period for multi-core allocation")
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, int64(400000), execOpts.Resources.CPUQuota)
	assert.Equal(t, uint64(100000), execOpts.Resources.CPUPeriod)
}

func TestValidateAndSetResourceLimits_MultiCoreCPU_WithDefaultPeriod(t *testing.T) {
	cpuQuota = 300000  // 3x default period = 3 cores
	cpuPeriod = 0      // Use default 100000
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err, "Multi-core quota should work with default period")
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, int64(300000), execOpts.Resources.CPUQuota)
	assert.Equal(t, uint64(0), execOpts.Resources.CPUPeriod) // 0 means use default in cgroup setup
}

func TestValidateAndSetResourceLimits_InvalidMemory(t *testing.T) {
	cpuQuota = 0
	cpuPeriod = 0
	memory = "invalid"
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid memory")
}

func TestValidateAndSetResourceLimits_CPUSetOnly(t *testing.T) {
	cpuQuota = 0
	cpuPeriod = 0
	memory = ""
	cpusetCpus = "0,2,4"

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err)
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, "0,2,4", execOpts.Resources.CPUSetCPUs)
}

func TestValidateAndSetResourceLimits_CPUSetRange(t *testing.T) {
	cpuQuota = 0
	cpuPeriod = 0
	memory = ""
	cpusetCpus = "0-7"

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err)
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, "0-7", execOpts.Resources.CPUSetCPUs)
}

// Edge case: Maximum valid CPU quota (equal to period)
func TestValidateAndSetResourceLimits_MaxCPUQuota(t *testing.T) {
	cpuQuota = 100000
	cpuPeriod = 100000
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err)
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, int64(100000), execOpts.Resources.CPUQuota)
	assert.Equal(t, uint64(100000), execOpts.Resources.CPUPeriod)
}

// Edge case: Minimum valid CPU quota
func TestValidateAndSetResourceLimits_MinCPUQuota(t *testing.T) {
	cpuQuota = 1000 // Exactly 1ms
	cpuPeriod = 100000
	memory = ""
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err)
	require.NotNil(t, execOpts.Resources)
	assert.Equal(t, int64(1000), execOpts.Resources.CPUQuota)
}

// Edge case: Large memory value
func TestValidateAndSetResourceLimits_LargeMemory(t *testing.T) {
	cpuQuota = 0
	cpuPeriod = 0
	memory = "16g"
	cpusetCpus = ""

	execOpts := &entities.ExecOptions{}
	err := validateAndSetResourceLimits(execOpts)

	require.NoError(t, err)
	require.NotNil(t, execOpts.Resources)
	require.NotNil(t, execOpts.Resources.Memory)
	assert.Equal(t, int64(16*1024*1024*1024), *execOpts.Resources.Memory)
}

func TestValidateAndSetResourceLimits_InvalidCPUSetFormat(t *testing.T) {
	tests := []struct {
		name       string
		cpuset     string
		wantErr    bool
		errContain string
	}{
		{name: "single cpu", cpuset: "0", wantErr: false},
		{name: "cpu list", cpuset: "0,1,2", wantErr: false},
		{name: "cpu range", cpuset: "0-3", wantErr: false},
		{name: "mixed list and range", cpuset: "0-3,7,12-15", wantErr: false},
		{name: "letters only", cpuset: "abc", wantErr: true, errContain: "invalid cpuset-cpus format"},
		{name: "trailing comma", cpuset: "0,1,", wantErr: true, errContain: "invalid cpuset-cpus format"},
		{name: "leading comma", cpuset: ",0,1", wantErr: true, errContain: "invalid cpuset-cpus format"},
		{name: "double comma", cpuset: "0,,1", wantErr: true, errContain: "invalid cpuset-cpus format"},
		{name: "spaces", cpuset: "0 1", wantErr: true, errContain: "invalid cpuset-cpus format"},
		{name: "colon separator", cpuset: "0:1", wantErr: true, errContain: "invalid cpuset-cpus format"},
		{name: "negative number", cpuset: "-1", wantErr: true, errContain: "invalid cpuset-cpus format"},
		{name: "double dash range", cpuset: "0--3", wantErr: true, errContain: "invalid cpuset-cpus format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuQuota = 0
			cpuPeriod = 0
			memory = ""
			cpusetCpus = tt.cpuset

			execOpts := &entities.ExecOptions{}
			err := validateAndSetResourceLimits(execOpts)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContain)
			} else {
				require.NoError(t, err)
				require.NotNil(t, execOpts.Resources)
				assert.Equal(t, tt.cpuset, execOpts.Resources.CPUSetCPUs)
			}
		})
	}
}

// Test cleanup - reset global variables
func TestCleanup(t *testing.T) {
	cpuQuota = 0
	cpuPeriod = 0
	memory = ""
	cpusetCpus = ""
}
