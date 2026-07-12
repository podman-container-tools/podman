//go:build !remote && (linux || freebsd)

package libpod

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractPlayReader(t *testing.T) {
	// Setup temporary directory for testing purposes
	tempDir := t.TempDir()

	t.Run("Content-Type not provided - should return body", func(t *testing.T) {
		req := &http.Request{
			Body: io.NopCloser(strings.NewReader("test body content")),
		}

		reader, err := extractPlayReader(tempDir, req)
		assert.NoError(t, err)

		// Read from the returned reader
		data, err := io.ReadAll(reader)
		assert.NoError(t, err)
		assert.Equal(t, "test body content", string(data))
	})

	t.Run("Supported content types (json/yaml/text) - should return body", func(t *testing.T) {
		supportedTypes := []string{
			"application/json",
			"application/yaml",
			"application/text",
			"application/x-yaml",
		}

		for _, contentType := range supportedTypes {
			req := &http.Request{
				Header: map[string][]string{
					"Content-Type": {contentType},
				},
				Body: io.NopCloser(strings.NewReader("test body content")),
			}

			reader, err := extractPlayReader(tempDir, req)
			assert.NoError(t, err)

			// Read from the returned reader
			data, err := io.ReadAll(reader)
			assert.NoError(t, err)
			assert.Equal(t, "test body content", string(data))
		}
	})

	t.Run("Unsupported content type - should return error", func(t *testing.T) {
		req := &http.Request{
			Header: map[string][]string{
				"Content-Type": {"application/unsupported"},
			},
			Body: io.NopCloser(strings.NewReader("test body content")),
		}

		_, err := extractPlayReader(tempDir, req)
		assert.Error(t, err)
		assert.Equal(t, "Content-Type: application/unsupported is not supported. Should be \"application/x-tar\"", err.Error())
	})
}
