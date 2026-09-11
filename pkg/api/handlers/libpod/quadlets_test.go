//go:build !remote && (linux || freebsd)

package libpod

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createMultipartRequest(t *testing.T, files map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for filename, content := range files {
		part, err := writer.CreateFormFile("file", filename)
		require.NoError(t, err)
		_, err = part.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req, err := http.NewRequest(http.MethodPost, "/libpod/quadlets/install", &body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestProcessMultipartQuadlets(t *testing.T) {
	t.Run("multiple files extracted correctly", func(t *testing.T) {
		tempDir := t.TempDir()
		files := map[string]string{
			"app1.container": "[Container]\nImage=alpine\n",
			"app2.volume":    "[Volume]\n",
		}
		req := createMultipartRequest(t, files)

		paths, err := processMultipartQuadlets(tempDir, req)
		require.NoError(t, err)
		assert.Len(t, paths, 2)

		for filename, expectedContent := range files {
			expectedPath := filepath.Join(tempDir, "quadlets", filename)
			assert.Contains(t, paths, expectedPath)

			content, err := os.ReadFile(expectedPath)
			require.NoError(t, err)
			assert.Equal(t, expectedContent, string(content))

			info, err := os.Stat(expectedPath)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}
	})

	t.Run("duplicate filename returns error", func(t *testing.T) {
		tempDir := t.TempDir()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)

		// Add two parts with the exact same filename
		part1, err := writer.CreateFormFile("file", "test.container")
		require.NoError(t, err)
		_, err = part1.Write([]byte("first"))
		require.NoError(t, err)

		part2, err := writer.CreateFormFile("file", "test.container")
		require.NoError(t, err)
		_, err = part2.Write([]byte("second"))
		require.NoError(t, err)

		require.NoError(t, writer.Close())

		req, err := http.NewRequest(http.MethodPost, "/libpod/quadlets/install", &body)
		require.NoError(t, err)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		_, err = processMultipartQuadlets(tempDir, req)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create file test.container")
	})

	t.Run("path traversal in filename is sanitized", func(t *testing.T) {
		tempDir := t.TempDir()
		files := map[string]string{
			"../../evil.container": "[Container]\nImage=evil\n",
		}
		req := createMultipartRequest(t, files)

		paths, err := processMultipartQuadlets(tempDir, req)
		require.NoError(t, err)
		require.Len(t, paths, 1)

		expectedPath := filepath.Join(tempDir, "quadlets", "evil.container")
		assert.Equal(t, expectedPath, paths[0])

		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, "[Container]\nImage=evil\n", string(content))
	})
}
