//go:build !remote && (linux || freebsd)

package libpod

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v5.0.0/libpod/quadlets/install", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

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
}
