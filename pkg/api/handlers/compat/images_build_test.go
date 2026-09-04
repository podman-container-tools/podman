//go:build !remote && (linux || freebsd)

package compat

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProcessSecretsDoesNotLeavePlaintextInContextParent covers the host leak in
// https://github.com/containers/podman/issues/29687: processSecrets used to
// os.Rename the secret into the parent of the build context and never delete it.
// For local-API / machine builds that parent is a user directory (e.g. the folder
// containing the context), not a scratch dir.
func TestProcessSecretsDoesNotLeavePlaintextInContextParent(t *testing.T) {
	parent := t.TempDir()
	contextDir := filepath.Join(parent, "ctx")
	require.NoError(t, os.Mkdir(contextDir, 0o755))

	secretName := "podman-build-secret-test"
	secretPath := filepath.Join(contextDir, secretName)
	secretValue := []byte("s3cr3t-value")
	require.NoError(t, os.WriteFile(secretPath, secretValue, 0o600))

	secretsJSON := `["id=tok,src=` + secretName + `"]`
	query := &BuildQuery{Secrets: secretsJSON}
	queryValues := url.Values{"secrets": []string{secretsJSON}}

	secrets, relocated, err := processSecrets(query, contextDir, queryValues)
	require.NoError(t, err)
	require.Len(t, secrets, 1)
	t.Cleanup(func() {
		for _, path := range relocated {
			_ = os.Remove(path)
		}
	})

	// Must leave the context so a later COPY cannot pick the secret up.
	assert.NoFileExists(t, secretPath)

	// Must not park the plaintext copy in the context's parent.
	leftover, err := filepath.Glob(filepath.Join(parent, "podman-build-secret-*"))
	require.NoError(t, err)
	assert.Empty(t, leftover, "secret must not be left in the parent of the build context")

	srcPath := secretSrcPath(t, secrets[0])
	assert.FileExists(t, srcPath)
	assert.Equal(t, []string{srcPath}, relocated)
	got, err := os.ReadFile(srcPath)
	require.NoError(t, err)
	assert.Equal(t, secretValue, got)
}

func secretSrcPath(t *testing.T, secret string) string {
	t.Helper()
	for _, token := range strings.Split(secret, ",") {
		key, val, ok := strings.Cut(token, "=")
		if ok && key == "src" {
			return val
		}
	}
	t.Fatalf("secret %q has no src=", secret)
	return ""
}
