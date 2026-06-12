package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecConfig_CgroupPath(t *testing.T) {
	execConfig := &ExecConfig{
		Command:    []string{"/bin/sh", "-c", "echo test"},
		Terminal:   true,
		CgroupPath: "../exec-abc123",
	}

	assert.Equal(t, "../exec-abc123", execConfig.CgroupPath)
}

func TestExecConfig_CgroupPathEmpty(t *testing.T) {
	execConfig := &ExecConfig{
		Command:    []string{"/bin/sh"},
		CgroupPath: "",
	}

	assert.Empty(t, execConfig.CgroupPath)
}

func TestExecConfig_CgroupPathJSON(t *testing.T) {
	execConfig := &ExecConfig{
		Command:    []string{"/bin/sh", "-c", "test"},
		Terminal:   true,
		User:       "root",
		WorkDir:    "/tmp",
		CgroupPath: "../exec-123",
	}

	// Marshal to JSON
	data, err := json.Marshal(execConfig)
	require.NoError(t, err)

	// Verify JSON contains cgroupPath
	assert.Contains(t, string(data), "cgroupPath")
	assert.Contains(t, string(data), "../exec-123")

	// Unmarshal back
	var decoded ExecConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, execConfig.CgroupPath, decoded.CgroupPath)
	assert.Equal(t, execConfig.Command, decoded.Command)
	assert.Equal(t, execConfig.Terminal, decoded.Terminal)
	assert.Equal(t, execConfig.User, decoded.User)
	assert.Equal(t, execConfig.WorkDir, decoded.WorkDir)
}

func TestExecConfig_CgroupPathJSONOmitEmpty(t *testing.T) {
	execConfig := &ExecConfig{
		Command:    []string{"/bin/sh"},
		Terminal:   false,
		CgroupPath: "",
	}

	data, err := json.Marshal(execConfig)
	require.NoError(t, err)

	// When CgroupPath is empty, it should be omitted from JSON
	assert.NotContains(t, string(data), "cgroupPath")
}

func TestExecConfig_CgroupPathPreservesOtherFields(t *testing.T) {
	detachKeys := "ctrl-p,ctrl-q"
	env := map[string]string{
		"PATH": "/usr/bin:/bin",
		"HOME": "/root",
	}

	execConfig := &ExecConfig{
		Command:      []string{"/bin/bash", "-c", "sleep 60"},
		Terminal:     true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		DetachKeys:   &detachKeys,
		Environment:  env,
		Privileged:   true,
		User:         "postgres",
		WorkDir:      "/var/lib/postgresql",
		PreserveFDs:  3,
		CgroupPath:   "../exec-xyz",
	}

	assert.Equal(t, []string{"/bin/bash", "-c", "sleep 60"}, execConfig.Command)
	assert.True(t, execConfig.Terminal)
	assert.True(t, execConfig.AttachStdin)
	assert.True(t, execConfig.AttachStdout)
	assert.True(t, execConfig.AttachStderr)
	assert.Equal(t, "ctrl-p,ctrl-q", *execConfig.DetachKeys)
	assert.Equal(t, env, execConfig.Environment)
	assert.True(t, execConfig.Privileged)
	assert.Equal(t, "postgres", execConfig.User)
	assert.Equal(t, "/var/lib/postgresql", execConfig.WorkDir)
	assert.Equal(t, uint(3), execConfig.PreserveFDs)
	assert.Equal(t, "../exec-xyz", execConfig.CgroupPath)
}

func TestExecConfig_CgroupPathWithExitCommand(t *testing.T) {
	exitCmd := []string{"/usr/bin/podman", "--root", "/var/lib/containers/storage"}

	execConfig := &ExecConfig{
		Command:          []string{"/bin/sh"},
		ExitCommand:      exitCmd,
		ExitCommandDelay: 5,
		CgroupPath:       "../exec-abc",
	}

	assert.Equal(t, exitCmd, execConfig.ExitCommand)
	assert.Equal(t, uint(5), execConfig.ExitCommandDelay)
	assert.Equal(t, "../exec-abc", execConfig.CgroupPath)
}

func TestExecConfig_CgroupPathVariations(t *testing.T) {
	tests := []struct {
		name       string
		cgroupPath string
	}{
		{
			name:       "relative sibling path",
			cgroupPath: "../exec-123",
		},
		{
			name:       "relative sibling with nanosecond ID",
			cgroupPath: "../exec-1718193847283947123",
		},
		{
			name:       "relative sibling short ID",
			cgroupPath: "../exec-456",
		},
		{
			name:       "relative sibling def456",
			cgroupPath: "../exec-def456",
		},
		{
			name:       "relative sibling simple",
			cgroupPath: "../exec-789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			execConfig := &ExecConfig{
				Command:    []string{"/bin/sh"},
				CgroupPath: tt.cgroupPath,
			}

			assert.Equal(t, tt.cgroupPath, execConfig.CgroupPath)

			// Verify JSON round-trip
			data, err := json.Marshal(execConfig)
			require.NoError(t, err)

			var decoded ExecConfig
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.cgroupPath, decoded.CgroupPath)
		})
	}
}

func TestExecConfig_CgroupPathBackwardCompatibility(t *testing.T) {
	// Simulate old JSON without cgroupPath field
	oldJSON := `{
		"command": ["/bin/sh", "-c", "echo test"],
		"terminal": true,
		"user": "root"
	}`

	var execConfig ExecConfig
	err := json.Unmarshal([]byte(oldJSON), &execConfig)
	require.NoError(t, err)

	// CgroupPath should be empty (omitempty)
	assert.Empty(t, execConfig.CgroupPath)
	assert.Equal(t, []string{"/bin/sh", "-c", "echo test"}, execConfig.Command)
	assert.True(t, execConfig.Terminal)
	assert.Equal(t, "root", execConfig.User)
}

func TestExecConfig_CgroupPathForwardCompatibility(t *testing.T) {
	// New JSON with cgroupPath field
	newJSON := `{
		"command": ["/bin/sh"],
		"terminal": false,
		"cgroupPath": "../exec-new"
	}`

	var execConfig ExecConfig
	err := json.Unmarshal([]byte(newJSON), &execConfig)
	require.NoError(t, err)

	assert.Equal(t, "../exec-new", execConfig.CgroupPath)
	assert.Equal(t, []string{"/bin/sh"}, execConfig.Command)
	assert.False(t, execConfig.Terminal)
}

func TestExecConfig_CgroupPathNotAffectOtherJSONFields(t *testing.T) {
	execConfig := &ExecConfig{
		Command:      []string{"/usr/bin/stress-ng", "--cpu", "1"},
		Terminal:     true, // Changed to true so it appears in JSON
		AttachStdout: true,
		AttachStderr: true,
		Environment: map[string]string{
			"STRESS_OPTS": "--timeout 60s",
		},
		CgroupPath: "../exec-stress",
	}

	data, err := json.Marshal(execConfig)
	require.NoError(t, err)

	// Verify all non-empty fields are in JSON
	jsonStr := string(data)
	assert.Contains(t, jsonStr, "command")
	assert.Contains(t, jsonStr, "terminal")
	assert.Contains(t, jsonStr, "attachStdout")
	assert.Contains(t, jsonStr, "attachStderr")
	assert.Contains(t, jsonStr, "environment")
	assert.Contains(t, jsonStr, "cgroupPath")
	assert.Contains(t, jsonStr, "../exec-stress")
}
