//go:build windows

package machine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	winio "github.com/Microsoft/go-winio"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/machine/define"
)

func testNamedPipe(t *testing.T) (string, func() error) {
	t.Helper()

	pipeName := fmt.Sprintf("podman-machine-test-%d", os.Getpid())
	listener, err := winio.ListenPipe(`\\.\pipe\`+pipeName, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	return pipeName, listener.Close
}

// A stale proxy is cleaned up when its state file has already supplied the PID.
func TestCleanupStaleProxy(t *testing.T) {
	pipeName, closePipe := testNamedPipe(t)
	cleaned := false
	err := cleanupStaleProxy(pipeName, uint32(os.Getpid()), func() error {
		cleaned = true
		return closePipe()
	})

	require.NoError(t, err)
	require.True(t, cleaned)
}

func TestCleanupStaleGVProxyFailsWithoutPIDFile(t *testing.T) {
	pipeName, closePipe := testNamedPipe(t)
	defer func() { _ = closePipe() }()

	pidFile := define.VMFile{Path: filepath.Join(t.TempDir(), "missing.pid")}
	err := CleanupStaleGVProxy(pipeName, pidFile)
	require.ErrorContains(t, err, "reading gvproxy PID file")
}

// CreateNewItemWithPowerShell creates a new item using PowerShell.
// It's an helper to easily create junctions on Windows (as well as other file types).
// It constructs a PowerShell command to create a new item at the specified path with the given item type.
// If a target is provided, it includes it in the command.
//
// Parameters:
//   - t: The testing.T instance.
//   - path: The path where the new item will be created.
//   - itemType: The type of the item to be created (e.g., "File", "SymbolicLink", "Junction").
//   - target: The target for the new item, if applicable.
func CreateNewItemWithPowerShell(t *testing.T, path string, itemType string, target string) {
	var pwshCmd, pwshPath string
	// Look for Powershell 7 first as it allow Symlink creation for non-admins too
	pwshPath, err := exec.LookPath("pwsh.exe")
	if err != nil {
		// Use Powershell 5 that is always present
		pwshPath = "powershell.exe"
	}
	pwshCmd = "New-Item -Path " + path + " -ItemType " + itemType
	if target != "" {
		pwshCmd += " -Target " + target
	}
	cmd := exec.Command(pwshPath, "-Command", pwshCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	require.NoError(t, err)
}
