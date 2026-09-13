//go:build windows

package machine

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	winio "github.com/Microsoft/go-winio"
	"github.com/stretchr/testify/require"
)

func testNamedPipe(t *testing.T) (string, func() error) {
	t.Helper()

	pipeName := fmt.Sprintf("podman-machine-test-%d", os.Getpid())
	listener, err := winio.ListenPipe(`\\.\pipe\`+pipeName, nil)
	require.NoError(t, err)

	closeConnection := make(chan struct{})
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			<-closeConnection
			_ = conn.Close()
		}
	}()

	return pipeName, func() error {
		close(closeConnection)
		return listener.Close()
	}
}

// A stale proxy is cleaned up only when it owns the named pipe.
func TestCleanupStaleProxy(t *testing.T) {
	t.Run("matching PID is cleaned up", func(t *testing.T) {
		pipeName, closePipe := testNamedPipe(t)

		cleaned := false
		err := cleanupStaleProxy(pipeName, uint32(os.Getpid()), func() error {
			cleaned = true
			return closePipe()
		})

		require.NoError(t, err)
		require.True(t, cleaned)
	})

	t.Run("mismatched PID is not cleaned up", func(t *testing.T) {
		pipeName, closePipe := testNamedPipe(t)
		defer func() { _ = closePipe() }()

		cleaned := false
		err := cleanupStaleProxy(pipeName, uint32(os.Getpid())+1, func() error {
			cleaned = true
			return nil
		})

		require.ErrorContains(t, err, "refusing to terminate the process")
		require.False(t, cleaned)
	})
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
