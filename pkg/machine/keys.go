//go:build amd64 || arm64

package machine

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.podman.io/storage/pkg/fileutils"
)

var sshCommand = []string{"ssh-keygen", "-N", "", "-t", "ed25519", "-f"}

// CreateSSHKeys makes a priv and pub ssh key for interacting
// the a VM.
func CreateSSHKeys(writeLocation string) (string, error) {
	// If the SSH key already exists, hard fail
	if err := fileutils.Exists(writeLocation); err == nil {
		return "", fmt.Errorf("SSH key already exists: %s", writeLocation)
	}
	if err := os.MkdirAll(filepath.Dir(writeLocation), 0o700); err != nil {
		return "", err
	}
	if err := generatekeys(writeLocation); err != nil {
		return "", err
	}
	b, err := os.ReadFile(writeLocation + ".pub")
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(string(b), "\n"), nil
}

// GetSSHKeys checks to see if there is a ssh key at the provided location.
// If not, we create the priv and pub keys. The ssh key is then returned.
func GetSSHKeys(identityPath string) (string, error) {
	if err := fileutils.Exists(identityPath); err == nil {
		b, err := os.ReadFile(identityPath + ".pub")
		if err != nil {
			return "", err
		}
		return strings.TrimSuffix(string(b), "\n"), nil
	}

	return CreateSSHKeys(identityPath)
}

// generatekeys creates an ed25519 set of keys
func generatekeys(writeLocation string) error {
	args := append(append([]string{}, sshCommand[1:]...), writeLocation)
	cmd := exec.Command(sshCommand[0], args...)
	stdErr := &bytes.Buffer{}
	cmd.Stderr = stdErr

	if err := cmd.Start(); err != nil {
		return err
	}
	waitErr := cmd.Wait()
	if waitErr != nil {
		return fmt.Errorf("failed to generate keys: %s: %w", strings.TrimSpace(stdErr.String()), waitErr)
	}

	return nil
}
