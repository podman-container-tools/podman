package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

func TestValidateScpCompression(t *testing.T) {
	level := func(l int) *int { return &l }

	tests := []struct {
		name    string
		opts    entities.ScpCompressionOptions
		wantErr string
	}{
		{
			name: "no compression requested",
			opts: entities.ScpCompressionOptions{},
		},
		{
			name: "level without a format",
			opts: entities.ScpCompressionOptions{CompressionLevel: level(9)},
			// A level on its own would be silently ignored, so reject it.
			wantErr: "a compression level requires a compression format",
		},
		{
			name: "unknown format",
			opts: entities.ScpCompressionOptions{CompressionFormat: "lz4"},
			// zstd:chunked and friends are known to c/image but are not
			// whole-stream formats podman load can pick up on its own.
			wantErr: `unsupported compression format "lz4"`,
		},
		{
			name: "zstd:chunked is not accepted",
			opts: entities.ScpCompressionOptions{CompressionFormat: "zstd:chunked"},
			// c/image knows this one, podman image scp deliberately does not.
			wantErr: `unsupported compression format "zstd:chunked"`,
		},
		{
			name: "level above the format's range",
			opts: entities.ScpCompressionOptions{CompressionFormat: "gzip", CompressionLevel: level(10)},
			// zstd would accept 10, gzip stops at 9.
			wantErr: `compression level 10 is out of range for "gzip", must be between 1 and 9`,
		},
		{
			name:    "level below the format's range",
			opts:    entities.ScpCompressionOptions{CompressionFormat: "zstd", CompressionLevel: level(0)},
			wantErr: `compression level 0 is out of range for "zstd", must be between 1 and 19`,
		},
		{
			name: "bzip2 is not accepted",
			opts: entities.ScpCompressionOptions{CompressionFormat: "bzip2"},
			// c/image can only decompress bzip2, so it cannot be produced here.
			wantErr: `unsupported compression format "bzip2"`,
		},
		{
			name: "zstd accepts level 19",
			opts: entities.ScpCompressionOptions{CompressionFormat: "zstd", CompressionLevel: level(19)},
		},
		{
			name: "gzip accepts level 9",
			opts: entities.ScpCompressionOptions{CompressionFormat: "gzip", CompressionLevel: level(9)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScpCompression(tt.opts)
			if tt.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// The list is part of the command's interface: it drives the flag's choices.
func TestScpCompressionFormatsAreUsable(t *testing.T) {
	assert.Equal(t, []string{"gzip", "zstd"}, ScpCompressionFormats())
	for _, format := range ScpCompressionFormats() {
		assert.NoError(t, ValidateScpCompression(entities.ScpCompressionOptions{CompressionFormat: format}))
	}
}

// The API path does not go through flag parsing. Ordering is checked by its
// consequence: ExecuteTransfer creates its temporary file straight after
// validating, so a later check would leave one behind on every rejected request.
func TestExecuteTransferRejectsBadCompressionBeforeDoingAnything(t *testing.T) {
	level := func(l int) *int { return &l }

	tests := []struct {
		name    string
		opts    entities.ScpCompressionOptions
		wantErr string
	}{
		{
			name:    "level without a format",
			opts:    entities.ScpCompressionOptions{CompressionLevel: level(9)},
			wantErr: "a compression level requires a compression format",
		},
		{
			name:    "level out of range",
			opts:    entities.ScpCompressionOptions{CompressionFormat: "gzip", CompressionLevel: level(10)},
			wantErr: `compression level 10 is out of range for "gzip"`,
		},
		{
			name:    "format podman image scp cannot produce",
			opts:    entities.ScpCompressionOptions{CompressionFormat: "bzip2"},
			wantErr: `unsupported compression format "bzip2"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			t.Setenv("TMPDIR", tmp)

			_, err := ExecuteTransfer("alpine", "QA::", entities.ScpExecuteTransferOptions{ScpCompressionOptions: tt.opts})
			assert.ErrorContains(t, err, tt.wantErr)
			assert.ErrorIs(t, err, define.ErrInvalidArg)

			entries, readErr := os.ReadDir(tmp)
			require.NoError(t, readErr)
			assert.Empty(t, entries, "the transfer's temporary file was created before the options were rejected")
		})
	}
}
