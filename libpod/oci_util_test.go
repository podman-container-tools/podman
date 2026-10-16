//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/libpod/define"
)

// TestParseOCIErrors verifies parsing of multiple ociErrors
// from a log file.
func TestParseOCIErrors(t *testing.T) {
	log := []byte(`{"msg":"(00.022100) Error (criu/sk-inet.c:654): Connected TCP socket in image","level":"error","time":"2025-01-01T00:00:00Z"}
{"msg":"(00.022200) Error (criu/cr-restore.c:2522): Restoring FAILED.","level":"error","time":"2025-01-01T00:00:01Z"}
{"msg":"(00.022418) Error (criu/cgroup.c:1970): cg: cgroupd: recv req error: No such file or directory","level":"error","time":"2025-01-01T00:00:02Z"}
`)

	errs, err := parseOCIErrors(log)
	require.NoError(t, err)

	// Verify that all 3 messages are parsed.
	assert.Equal(t, []ociError{
		{Msg: "(00.022100) Error (criu/sk-inet.c:654): Connected TCP socket in image", Level: "error", Time: "2025-01-01T00:00:00Z"},
		{Msg: "(00.022200) Error (criu/cr-restore.c:2522): Restoring FAILED.", Level: "error", Time: "2025-01-01T00:00:01Z"},
		{Msg: "(00.022418) Error (criu/cgroup.c:1970): cg: cgroupd: recv req error: No such file or directory", Level: "error", Time: "2025-01-01T00:00:02Z"},
	}, errs)
}

// TestJoinOCIErrors verifies that multiple OCI error messages
// are combined into a single multiline message.
func TestJoinOCIErrors(t *testing.T) {
	errs := []ociError{
		{Msg: "(00.022100) Error (criu/sk-inet.c:654): Connected TCP socket in image"},
		{Msg: "(00.022200) Error (criu/cr-restore.c:2522): Restoring FAILED."},
		{Msg: "(00.022418) Error (criu/cgroup.c:1970): cg: cgroupd: recv req error: No such file or directory"},
	}

	combined := joinOCIErrors(errs)

	expected := `(00.022100) Error (criu/sk-inet.c:654): Connected TCP socket in image
(00.022200) Error (criu/cr-restore.c:2522): Restoring FAILED.
(00.022418) Error (criu/cgroup.c:1970): cg: cgroupd: recv req error: No such file or directory`

	assert.Equal(t, expected, combined)

	// Empty errors should return empty string.
	assert.Equal(t, "", joinOCIErrors([]ociError{}))
}

// TestGetOCIRuntimeErrorMultiline verifies that multiple OCI errors are
// forwarded as-is as a generic OCI runtime error, while a single error is
// still pattern matched so its specific error type (and exit code) is preserved.
func TestGetOCIRuntimeErrorMultiline(t *testing.T) {
	// Multiple errors (failed CRIU restore): all messages are forwarded and
	// the unrelated "No such file or directory" line must NOT be classified
	// as ErrOCIRuntimeNotFound.
	criuErrs := []ociError{
		{Msg: "(00.022100) Error (criu/sk-inet.c:654): Connected TCP socket in image"},
		{Msg: "(00.022200) Error (criu/cr-restore.c:2522): Restoring FAILED."},
		{Msg: "(00.022418) Error (criu/cgroup.c:1970): cg: cgroupd: recv req error: No such file or directory"},
	}

	err := getOCIRuntimeErrorMultiline("crun", criuErrs)
	require.Error(t, err)

	// All three messages are included in the output.
	assert.Contains(t, err.Error(), "Connected TCP socket in image")
	assert.Contains(t, err.Error(), "Restoring FAILED")
	assert.Contains(t, err.Error(), "No such file or directory")

	// It is a generic OCI runtime error, not misclassified as "not found".
	assert.ErrorIs(t, err, define.ErrOCIRuntime)
	assert.NotErrorIs(t, err, define.ErrOCIRuntimeNotFound)

	// A single "not found" error is still classified so podman run/exec keep
	// their 127 exit code.
	notFoundErr := getOCIRuntimeErrorMultiline("crun", []ociError{
		{Msg: "executable file not found in $PATH: No such file or directory"},
	})
	require.Error(t, notFoundErr)
	assert.Contains(t, notFoundErr.Error(), "executable file not found")
	assert.ErrorIs(t, notFoundErr, define.ErrOCIRuntimeNotFound)
}
