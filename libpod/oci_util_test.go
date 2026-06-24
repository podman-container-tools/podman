//go:build !remote && (linux || freebsd)

package libpod

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestCheckOCIErrorsForTCPEstablished tests matching
// of "Connected TCP socket in image" errors in a log file.
func TestCheckOCIErrorsForTCPEstablished(t *testing.T) {
	withTCP := []ociError{
		{Msg: "(00.022100) Error (criu/sk-inet.c:654): Connected TCP socket in image", Level: "error"},
		{Msg: "(00.022200) Error (criu/cr-restore.c:2522): Restoring FAILED.", Level: "error"},
		{Msg: "(00.022418) Error (criu/cgroup.c:1970): cg: cgroupd: recv req error: No such file or directory", Level: "error"},
	}

	err := checkOCIErrorsForTCPEstablished(withTCP)

	// Verify that the log message is matched and expected error converted.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--tcp-established")

	withoutTCP := []ociError{
		{Msg: "some other OCI runtime error", Level: "error"},
	}

	// Verify that different message is not matched.
	err = checkOCIErrorsForTCPEstablished(withoutTCP)
	assert.NoError(t, err)
}
