package utils

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/pkg/ssh"
	"go.podman.io/image/v5/pkg/compression"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/storage/pkg/archive"
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

// The feature rests on podman load recognising the compression unaided, and the
// two archive formats reach that through different detectors.
func TestCompressReaderIsDetectedByBothLoadPaths(t *testing.T) {
	payload := []byte(strings.Repeat("podman image scp compression payload\n", 512))

	for _, format := range ScpCompressionFormats() {
		t.Run(format, func(t *testing.T) {
			reader, err := compressReader(bytes.NewReader(payload), entities.ScpCompressionOptions{CompressionFormat: format})
			require.NoError(t, err)
			defer reader.Close()
			compressed, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.Less(t, len(compressed), len(payload), "compressed output should be smaller than the input")

			// docker-archive: c/image tarfile.Reader uses AutoDecompress.
			viaCImage, isCompressed, err := compression.AutoDecompress(bytes.NewReader(compressed))
			require.NoError(t, err)
			require.True(t, isCompressed)
			defer viaCImage.Close()
			roundTripped, err := io.ReadAll(viaCImage)
			require.NoError(t, err)
			assert.Equal(t, payload, roundTripped)

			// oci-archive: c/storage archive.Untar uses DecompressStream.
			viaCStorage, err := archive.DecompressStream(bytes.NewReader(compressed))
			require.NoError(t, err)
			defer viaCStorage.Close()
			roundTripped, err = io.ReadAll(viaCStorage)
			require.NoError(t, err)
			assert.Equal(t, payload, roundTripped)
		})
	}
}

func TestCompressReaderWithLevel(t *testing.T) {
	payload := []byte(strings.Repeat("podman image scp compression payload\n", 512))

	level := 1
	reader, err := compressReader(bytes.NewReader(payload), entities.ScpCompressionOptions{
		CompressionFormat: "gzip",
		CompressionLevel:  &level,
	})
	require.NoError(t, err)
	defer reader.Close()

	compressed, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Less(t, len(compressed), len(payload))
}

// c/image can compress xz and zstd:chunked, so these catch compressReader going
// straight to it. "lz4" would not: c/image does not know it either.
func TestCompressReaderRejectsFormatsOutsideTheTable(t *testing.T) {
	for _, format := range []string{"xz", "zstd:chunked", "lz4"} {
		t.Run(format, func(t *testing.T) {
			_, err := compressReader(bytes.NewReader(nil), entities.ScpCompressionOptions{CompressionFormat: format})
			assert.ErrorContains(t, err, "unsupported compression format")
			assert.ErrorIs(t, err, define.ErrInvalidArg)
		})
	}
}

func TestCompressReaderPropagatesReadError(t *testing.T) {
	reader, err := compressReader(&failingReader{}, entities.ScpCompressionOptions{CompressionFormat: "gzip"})
	require.NoError(t, err)
	defer reader.Close()

	_, err = io.ReadAll(reader)
	assert.ErrorContains(t, err, "read failed")
}

// The path itself needs two hosts, so without this the flag could stop being
// honoured and every other test would still pass.
func TestCompressionOptionsReachTheTransfer(t *testing.T) {
	level := 9
	compress := entities.ScpCompressionOptions{CompressionFormat: "gzip", CompressionLevel: &level}
	opts := entities.ScpExecuteTransferOptions{SSHMode: ssh.GolangMode, ScpCompressionOptions: compress}
	dest := entities.ScpTransferImageOptions{File: "/tmp/podman123"}
	url := &url.URL{Host: "example.test"}

	t.Run("a local source compresses into the stream", func(t *testing.T) {
		got := loadToRemoteOptions(dest, "/tmp/podman123", url, "iden", opts, opts.ScpCompressionOptions)
		assert.Equal(t, compress, got.ScpCompressionOptions)
	})
}

var errRead = errors.New("read failed")

type failingReader struct{}

func (*failingReader) Read([]byte) (int, error) {
	return 0, errRead
}
