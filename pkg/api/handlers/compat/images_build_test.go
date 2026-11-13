//go:build !remote && (linux || freebsd)

package compat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func secretQuery(t *testing.T, name string) (*BuildQuery, url.Values) {
	t.Helper()
	encoded, err := json.Marshal([]string{"id=secret,src=" + name})
	require.NoError(t, err)
	return &BuildQuery{Secrets: string(encoded)}, url.Values{"secrets": {string(encoded)}}
}

func TestProcessSecretsUsesScratchDirectory(t *testing.T) {
	for _, inBody := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy context", true: "request body"}[inBody], func(t *testing.T) {
			fixture := t.TempDir()
			contextDir := filepath.Join(fixture, "context")
			require.NoError(t, os.Mkdir(contextDir, 0o700))
			secretDir := filepath.Join(t.TempDir(), "secrets")
			sourceDir := contextDir
			if inBody {
				sourceDir = secretDir
			}
			require.NoError(t, os.MkdirAll(sourceDir, 0o700))
			require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "secret"), []byte("value"), 0o600))
			query, values := secretQuery(t, "secret")
			secrets, err := processSecrets(query, sourceDir, secretDir, values)
			require.NoError(t, err)
			assert.Equal(t, []string{"id=secret,src=" + filepath.Join(secretDir, "secret")}, secrets)
			contents, err := os.ReadFile(filepath.Join(secretDir, "secret"))
			require.NoError(t, err)
			assert.Equal(t, "value", string(contents))
			entries, err := os.ReadDir(contextDir)
			require.NoError(t, err)
			assert.Empty(t, entries)
			entries, err = os.ReadDir(fixture)
			require.NoError(t, err)
			assert.Len(t, entries, 1, "no secret may remain in the context parent")
		})
	}
}

func TestProcessSecretsAcrossFilesystems(t *testing.T) {
	anchorDir, err := os.MkdirTemp("/dev/shm", "podman-build-secret-test-") //nolint:usetesting // The destination must use a different filesystem.
	if err != nil {
		t.Skipf("cannot create a directory on /dev/shm: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(anchorDir) })
	sourceDir := t.TempDir()
	sourceInfo, err := os.Stat(sourceDir)
	require.NoError(t, err)
	destinationInfo, err := os.Stat(anchorDir)
	require.NoError(t, err)
	if sourceInfo.Sys().(*syscall.Stat_t).Dev == destinationInfo.Sys().(*syscall.Stat_t).Dev {
		t.Skip("source and destination use the same filesystem")
	}
	require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "secret"), []byte("value"), 0o600))
	query, values := secretQuery(t, "secret")
	secretDir := filepath.Join(anchorDir, "secrets")
	_, err = processSecrets(query, sourceDir, secretDir, values)
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(sourceDir, "secret"))
	contents, err := os.ReadFile(filepath.Join(secretDir, "secret"))
	require.NoError(t, err)
	assert.Equal(t, "value", string(contents))
}

func TestProcessSecretsRejectsUnsafeSources(t *testing.T) {
	for _, name := range []string{"../outside", "/absolute", `..\outside`, "", ".", ".."} {
		t.Run(name, func(t *testing.T) {
			query, values := secretQuery(t, name)
			_, err := processSecrets(query, t.TempDir(), filepath.Join(t.TempDir(), "secrets"), values)
			require.ErrorContains(t, err, "invalid secret filename")
		})
	}
	sourceDir := t.TempDir()
	require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(sourceDir, "secret")))
	query, values := secretQuery(t, "secret")
	_, err := processSecrets(query, sourceDir, filepath.Join(t.TempDir(), "secrets"), values)
	require.ErrorContains(t, err, "must be a regular file")
}

func TestLocalBuildSecretsCleanupOnError(t *testing.T) {
	fixture := t.TempDir()
	contextDir := filepath.Join(fixture, "context")
	require.NoError(t, os.Mkdir(contextDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(contextDir, "Containerfile"), []byte("FROM scratch\n"), 0o600))
	scratchParent := t.TempDir()
	t.Setenv("TMPDIR", scratchParent)
	t.Setenv("PODMAN_RETAIN_BUILD_ARTIFACT", "false")
	body := strings.NewReader("\x1f\x8b\x08\xffinvalid gzip header")
	req := httptest.NewRequest(http.MethodPost, "/v6.3.0/libpod/local/build?localcontextdir="+url.QueryEscape(contextDir), body)
	req.Header.Set("Content-Type", "application/x-tar")
	recorder := httptest.NewRecorder()
	LocalBuildImage(recorder, req)
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "reading build secrets")
	entries, err := os.ReadDir(scratchParent)
	require.NoError(t, err)
	assert.Empty(t, entries, "failed requests must remove the scratch directory")
	entries, err = os.ReadDir(fixture)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	entries, err = os.ReadDir(contextDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}
