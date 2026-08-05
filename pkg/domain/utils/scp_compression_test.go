package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.podman.io/common/pkg/ssh"
	"go.podman.io/image/v5/pkg/compression"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
	"go.podman.io/storage/pkg/archive"
	"go.podman.io/storage/pkg/fileutils"
	cryptossh "golang.org/x/crypto/ssh"
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

func TestRemoteCompressCommand(t *testing.T) {
	level := func(l int) *int { return &l }

	tests := []struct {
		name           string
		opts           entities.ScpCompressionOptions
		wantBin        string
		wantArgv       []string
		wantCompressed string
		wantErr        string
	}{
		{
			name:           "gzip without a level",
			opts:           entities.ScpCompressionOptions{CompressionFormat: "gzip"},
			wantBin:        "gzip",
			wantArgv:       []string{"gzip", "-f", "-q", "/tmp/tmp.XXXX"},
			wantCompressed: "/tmp/tmp.XXXX.gz",
		},
		{
			name:           "gzip with a level",
			opts:           entities.ScpCompressionOptions{CompressionFormat: "gzip", CompressionLevel: level(9)},
			wantBin:        "gzip",
			wantArgv:       []string{"gzip", "-f", "-q", "-9", "/tmp/tmp.XXXX"},
			wantCompressed: "/tmp/tmp.XXXX.gz",
		},
		{
			name: "zstd keeps its input unless told otherwise",
			opts: entities.ScpCompressionOptions{CompressionFormat: "zstd"},
			// --rm matters: without it the uncompressed archive is left behind
			// on the remote host next to the compressed one.
			wantBin:        "zstd",
			wantArgv:       []string{"zstd", "-f", "-q", "--rm", "/tmp/tmp.XXXX"},
			wantCompressed: "/tmp/tmp.XXXX.zst",
		},
		{
			name:           "zstd with a level",
			opts:           entities.ScpCompressionOptions{CompressionFormat: "zstd", CompressionLevel: level(19)},
			wantBin:        "zstd",
			wantArgv:       []string{"zstd", "-f", "-q", "--rm", "-19", "/tmp/tmp.XXXX"},
			wantCompressed: "/tmp/tmp.XXXX.zst",
		},
		{
			name:    "unsupported format",
			opts:    entities.ScpCompressionOptions{CompressionFormat: "bzip2"},
			wantErr: `unsupported compression format "bzip2"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin, argv, compressedFile, err := remoteCompressCommand("/tmp/tmp.XXXX", tt.opts)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantBin, bin)
			assert.Equal(t, tt.wantArgv, argv)
			assert.Equal(t, tt.wantCompressed, compressedFile)
			// The path has to be the last argument: the compressors take their
			// flags first and everything after is treated as a file name.
			assert.Equal(t, "/tmp/tmp.XXXX", argv[len(argv)-1])
		})
	}
}

// A wrong suffix makes the scp of the compressed archive fail with "no such file".
func TestRemoteCompressCommandExtensionMatchesCompressor(t *testing.T) {
	extensions := map[string]string{"gzip": ".gz", "zstd": ".zst"}

	for _, format := range ScpCompressionFormats() {
		ext, known := extensions[format]
		require.True(t, known, "no expected extension recorded for %q", format)

		_, _, compressedFile, err := remoteCompressCommand("/tmp/archive", entities.ScpCompressionOptions{CompressionFormat: format})
		require.NoError(t, err)
		assert.Equal(t, "/tmp/archive"+ext, compressedFile)
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

// Run the argv that would be sent to the remote host against the real
// compressors: the only check that the flags do what compressRemoteFile assumes.
func TestRemoteCompressCommandAgainstRealCompressors(t *testing.T) {
	payload := []byte(strings.Repeat("podman image scp compression payload\n", 512))

	for _, format := range ScpCompressionFormats() {
		t.Run(format, func(t *testing.T) {
			opts := entities.ScpCompressionOptions{CompressionFormat: format}
			bin, argv, compressedFile, err := remoteCompressCommand(filepath.Join(t.TempDir(), "archive"), opts)
			require.NoError(t, err)

			if _, err := exec.LookPath(bin); err != nil {
				t.Skipf("%s is not installed", bin)
			}

			archiveFile := argv[len(argv)-1]
			require.NoError(t, os.WriteFile(archiveFile, payload, 0o600))

			out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
			require.NoError(t, err, "%s failed: %s", bin, out)

			compressed, err := os.ReadFile(compressedFile)
			require.NoError(t, err, "%s did not produce %q", bin, compressedFile)
			assert.Less(t, len(compressed), len(payload))

			// The uncompressed archive must be gone: it can be gigabytes, and
			// only the compressed path gets cleaned up afterwards.
			assert.ErrorIs(t, fileutils.Lexists(archiveFile), os.ErrNotExist,
				"%s left the uncompressed archive behind", bin)

			decompressed, err := archive.DecompressStream(bytes.NewReader(compressed))
			require.NoError(t, err)
			defer decompressed.Close()
			roundTripped, err := io.ReadAll(decompressed)
			require.NoError(t, err)
			assert.Equal(t, payload, roundTripped)
		})
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

func TestCompressRemoteFile(t *testing.T) {
	execOpts := ssh.ConnectionExecOptions{Host: "ssh://root@example.test"}
	opts := entities.ScpCompressionOptions{CompressionFormat: "gzip"}

	t.Run("success returns the compressed path and cleans up nothing", func(t *testing.T) {
		remote := &fakeRemote{}
		got, err := compressRemoteFile(remote.exec, execOpts, ssh.GolangMode, "/tmp/tmp.XXXX", opts)
		require.NoError(t, err)
		assert.Equal(t, "/tmp/tmp.XXXX.gz", got)
		assert.Equal(t, [][]string{{"gzip", "-f", "-q", "/tmp/tmp.XXXX"}}, remote.argv)
	})

	t.Run("a missing compressor is named as such", func(t *testing.T) {
		remote := &fakeRemote{errs: []error{exitStatusError{status: cmdNotFoundStatus}}}
		_, err := compressRemoteFile(remote.exec, execOpts, ssh.GolangMode, "/tmp/tmp.XXXX", opts)
		assert.ErrorContains(t, err, `requires the "gzip" command on the remote host`)
	})

	t.Run("a compressor that fails for its own reasons is not blamed on the host", func(t *testing.T) {
		// zstd exits 1 when it cannot write its output.
		remote := &fakeRemote{errs: []error{exitStatusError{status: 1}}}
		_, err := compressRemoteFile(remote.exec, execOpts, ssh.GolangMode, "/tmp/tmp.XXXX", opts)
		assert.ErrorContains(t, err, `compressing "/tmp/tmp.XXXX" on the remote host`)
		assert.NotContains(t, err.Error(), "requires the")
	})

	t.Run("any other failure is not blamed on a missing compressor", func(t *testing.T) {
		remote := &fakeRemote{errs: []error{errors.New("failed to connect: no route to host")}}
		_, err := compressRemoteFile(remote.exec, execOpts, ssh.GolangMode, "/tmp/tmp.XXXX", opts)
		assert.ErrorContains(t, err, `compressing "/tmp/tmp.XXXX" on the remote host`)
		assert.NotContains(t, err.Error(), "requires the")
	})

	t.Run("a failure leaves neither the archive nor a partial output behind", func(t *testing.T) {
		remote := &fakeRemote{errs: []error{errors.New("no space left on device")}}
		_, err := compressRemoteFile(remote.exec, execOpts, ssh.GolangMode, "/tmp/tmp.XXXX", opts)
		require.Error(t, err)
		require.Len(t, remote.argv, 2)
		assert.Equal(t, []string{"rm", "-f", "/tmp/tmp.XXXX", "/tmp/tmp.XXXX.gz"}, remote.argv[1])
	})

	t.Run("an unsupported format never reaches the remote host", func(t *testing.T) {
		remote := &fakeRemote{}
		_, err := compressRemoteFile(remote.exec, execOpts, ssh.GolangMode, "/tmp/tmp.XXXX",
			entities.ScpCompressionOptions{CompressionFormat: "xz"})
		assert.ErrorContains(t, err, `unsupported compression format "xz"`)
		assert.Empty(t, remote.argv)
	})
}

// The paths themselves need two hosts, so without this the flag could stop being
// honoured on one of them and every other test would still pass.
func TestCompressionOptionsReachTheTransfer(t *testing.T) {
	level := 9
	compress := entities.ScpCompressionOptions{CompressionFormat: "gzip", CompressionLevel: &level}
	opts := entities.ScpExecuteTransferOptions{SSHMode: ssh.GolangMode, ScpCompressionOptions: compress}
	source := entities.ScpTransferImageOptions{Image: "alpine", File: "/tmp/podman123"}
	dest := entities.ScpTransferImageOptions{File: "/tmp/podman123"}
	url := &url.URL{Host: "example.test"}

	t.Run("a remote source compresses on the host holding the image", func(t *testing.T) {
		got := saveToRemoteOptions(source, url, "iden", opts, opts.ScpCompressionOptions)
		assert.Equal(t, compress, got.ScpCompressionOptions)
	})

	t.Run("a local source compresses into the stream", func(t *testing.T) {
		got := loadToRemoteOptions(dest, source.File, url, "iden", opts, opts.ScpCompressionOptions)
		assert.Equal(t, compress, got.ScpCompressionOptions)
	})

	t.Run("remote to remote does not compress twice", func(t *testing.T) {
		// Compressing again would produce a doubly wrapped stream podman load
		// cannot read.
		got := loadToRemoteOptions(dest, dest.File, url, "iden", opts, entities.ScpCompressionOptions{})
		assert.Empty(t, got.CompressionFormat)
		assert.Nil(t, got.CompressionLevel)
	})
}

// The fakes below prove the accessor matching works; these prove it still matches
// the types the ssh engines actually return.
var (
	_ interface{ ExitStatus() int } = (*cryptossh.ExitError)(nil)
	_ interface{ ExitCode() int }   = (*exec.ExitError)(nil)
)

// crypto/ssh's ExitError has an unexported status, so it cannot be built here.
type exitStatusError struct{ status int }

func (e exitStatusError) ExitStatus() int { return e.status }
func (e exitStatusError) Error() string {
	return fmt.Sprintf("Process exited with status %d", e.status)
}

// os/exec's ExitError, as the native ssh engine returns for the local ssh binary.
type exitCodeError struct{ code int }

func (e exitCodeError) ExitCode() int { return e.code }
func (e exitCodeError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

// The case that matters most: an error with no status, such as a failure to
// connect, must not be mistaken for a command that ran.
func TestRemoteExitStatus(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "golang engine, command not found",
			err:  exitStatusError{status: cmdNotFoundStatus},
			want: cmdNotFoundStatus,
		},
		{
			name: "native engine, command not found",
			err:  exitCodeError{code: cmdNotFoundStatus},
			want: cmdNotFoundStatus,
		},
		{
			name: "wrapped, as the golang engine returns it alongside remote stderr",
			err:  fmt.Errorf("sh: gzip: command not found: %w", exitStatusError{status: cmdNotFoundStatus}),
			want: cmdNotFoundStatus,
		},
		{
			name: "the command ran and failed for its own reasons",
			err:  exitStatusError{status: 1},
			want: 1,
		},
		{
			name: "never ran: no route to the host",
			err:  errors.New("failed to connect: dial tcp: no route to host"),
			want: -1,
		},
		{
			name: "never ran, wrapped",
			err:  fmt.Errorf("ssh: %w", errors.New("handshake failed")),
			want: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, remoteExitStatus(tt.err))
		})
	}
}

var errRead = errors.New("read failed")

type failingReader struct{}

func (*failingReader) Read([]byte) (int, error) {
	return 0, errRead
}
