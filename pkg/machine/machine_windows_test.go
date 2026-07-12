//go:build windows

package machine

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

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
