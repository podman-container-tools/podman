//go:build !remote && (linux || freebsd)

package libpod

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
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

	t.Run("dot and dotdot filenames are skipped", func(t *testing.T) {
		tempDir := t.TempDir()
		files := map[string]string{
			"..":             "[Container]\nImage=parent\n",
			".":              "[Container]\nImage=dot\n",
			"app1.container": "[Container]\nImage=valid\n",
		}
		req := createMultipartRequest(t, files)

		paths, err := processMultipartQuadlets(tempDir, req)
		require.NoError(t, err)
		require.Len(t, paths, 1)

		expectedPath := filepath.Join(tempDir, "quadlets", "app1.container")
		assert.Equal(t, expectedPath, paths[0])

		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, "[Container]\nImage=valid\n", string(content))
	})
}

// processMultipartQuadlets used to defer closing every file it wrote until it
// returned, so it held one descriptor per uploaded part for the whole request.
// Lowering the descriptor limit far below the number of parts makes such a
// leak fail the upload with "too many open files" instead of going unnoticed.
func TestProcessMultipartQuadletsDoesNotLeakDescriptors(t *testing.T) {
	const (
		numFiles = 256
		// Comfortably above what the test binary itself holds open, and far
		// below numFiles so that one descriptor per part cannot fit.
		fdLimit = 64
	)

	files := make(map[string]string, numFiles)
	for i := range numFiles {
		filename := fmt.Sprintf("unit%d.container", i)
		files[filename] = "[Container]\nImage=" + filename + "\n"
	}
	req := createMultipartRequest(t, files)

	// Created before the limit is lowered, so that the limit is restored
	// before the directory is cleaned up.
	tempDir := t.TempDir()

	var original syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original))
	if original.Max < fdLimit {
		t.Skipf("hard RLIMIT_NOFILE %d is below the %d this test needs", original.Max, fdLimit)
	}
	lowered := original
	lowered.Cur = fdLimit
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lowered))
	t.Cleanup(func() {
		assert.NoError(t, syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original))
	})

	paths, err := processMultipartQuadlets(tempDir, req)
	require.NoError(t, err)

	expectedPaths := make([]string, 0, numFiles)
	for filename, expectedContent := range files {
		expectedPath := filepath.Join(tempDir, "quadlets", filename)
		expectedPaths = append(expectedPaths, expectedPath)

		content, err := os.ReadFile(expectedPath)
		require.NoError(t, err)
		assert.Equal(t, expectedContent, string(content))
	}
	assert.ElementsMatch(t, expectedPaths, paths)
}
