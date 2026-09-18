//go:build !remote && (linux || freebsd)

package libpod

import (
	"bytes"
 fix-quadlet-multipart-fd-leak
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"

	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
 main
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

 fix-quadlet-multipart-fd-leak
// newMultipartQuadletRequest builds a request carrying count quadlet files,
// where the file at index i is named unit<i>.container and contains its own
// name so the caller can tell the files apart.
func newMultipartQuadletRequest(t *testing.T, count int) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for i := range count {
		part, err := writer.CreateFormFile("file", quadletPartName(i))
		require.NoError(t, err)
		_, err = part.Write([]byte(quadletPartName(i)))

func createMultipartRequest(t *testing.T, files map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for filename, content := range files {
		part, err := writer.CreateFormFile("file", filename)
		require.NoError(t, err)
		_, err = part.Write([]byte(content))
 main
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

fix-quadlet-multipart-fd-leak
	req := httptest.NewRequest(http.MethodPost, "/v5.0.0/libpod/quadlets/install", body)

	req, err := http.NewRequest(http.MethodPost, "/libpod/quadlets/install", &body)
	require.NoError(t, err)
 main
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

 fix-quadlet-multipart-fd-leak
func quadletPartName(i int) string {
	return fmt.Sprintf("unit%d.container", i)
}

// processMultipartQuadlets deferred closing each uploaded file until the whole
// request had been handled, so it held one descriptor per part for the
// lifetime of the call. Lowering the descriptor limit far below the number of
// parts makes that leak fail the request instead of going unnoticed.
func TestProcessMultipartQuadletsDoesNotLeakDescriptors(t *testing.T) {
	const (
		numFiles = 256
		// Comfortably above what the test binary itself needs, and far
		// below numFiles so one descriptor per part cannot fit.
		fdLimit = 64
	)

	var original syscall.Rlimit
	require.NoError(t, syscall.Getrlimit(syscall.RLIMIT_NOFILE, &original))
	if original.Max < fdLimit {
		t.Skipf("hard descriptor limit %d is below the %d this test needs", original.Max, fdLimit)
	}

	lowered := original
	lowered.Cur = fdLimit
	require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lowered))
	t.Cleanup(func() {
		require.NoError(t, syscall.Setrlimit(syscall.RLIMIT_NOFILE, &original))
	})

	tempDir := t.TempDir()
	req := newMultipartQuadletRequest(t, numFiles)

	filePaths, err := processMultipartQuadlets(tempDir, req)
	require.NoError(t, err)
	require.Len(t, filePaths, numFiles)

	for i, filePath := range filePaths {
		assert.Equal(t, filepath.Join(tempDir, "quadlets", quadletPartName(i)), filePath)

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, quadletPartName(i), string(content))
	}
}

// Parts that carry no filename are skipped, and skipping them must not stop
// the parts around them from being written.
func TestProcessMultipartQuadletsSkipsPartsWithoutFilename(t *testing.T) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "web.container")
	require.NoError(t, err)
	_, err = part.Write([]byte("[Container]"))
	require.NoError(t, err)

	require.NoError(t, writer.WriteField("replace", "true"))

	part, err = writer.CreateFormFile("file", "db.container")
	require.NoError(t, err)
	_, err = part.Write([]byte("[Container]"))
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v5.0.0/libpod/quadlets/install", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	tempDir := t.TempDir()
	filePaths, err := processMultipartQuadlets(tempDir, req)
	require.NoError(t, err)

	quadletDir := filepath.Join(tempDir, "quadlets")
	assert.Equal(t, []string{
		filepath.Join(quadletDir, "web.container"),
		filepath.Join(quadletDir, "db.container"),
	}, filePaths)

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
 main
}
