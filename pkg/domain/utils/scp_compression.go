package utils

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"go.podman.io/common/pkg/ssh"
	"go.podman.io/image/v5/pkg/compression"
	"go.podman.io/podman/v6/libpod/define"
	"go.podman.io/podman/v6/pkg/domain/entities"
)

// scpCompressionFormat describes one compression algorithm. A local archive is
// compressed with the c/image package; one on a remote host is compressed by
// running the command line compressor described here.
type scpCompressionFormat struct {
	bin string
	// args overwrite an existing output file, stay quiet, and remove the input
	// once done. gzip removes the input by default, zstd needs --rm.
	args []string
	// ext is the suffix the compressor appends to the file it compresses.
	ext string
	// Bounds are what the command line compressor accepts, applied to the local
	// path too so a level means one thing per algorithm.
	minLevel, maxLevel int
}

// scpCompressionFormats is the single source of truth for the accepted formats.
// An entry has to be detectable by podman load from the stream alone,
// compressible by c/image (which only decompresses bzip2 and xz), and available
// as a command of the same name for the remote case.
var scpCompressionFormats = map[string]scpCompressionFormat{
	"gzip": {bin: "gzip", args: []string{"-f", "-q"}, ext: ".gz", minLevel: 1, maxLevel: 9},
	// zstd goes up to 22, but only with --ultra; 19 is the plain maximum.
	"zstd": {bin: "zstd", args: []string{"-f", "-q", "--rm"}, ext: ".zst", minLevel: 1, maxLevel: 19},
}

// ScpCompressionFormats lists the accepted --compression-format values.
func ScpCompressionFormats() []string {
	return slices.Sorted(maps.Keys(scpCompressionFormats))
}

// scpCompressionFormatByName gives every caller the same rejection wording.
func scpCompressionFormatByName(name string) (scpCompressionFormat, error) {
	format, ok := scpCompressionFormats[name]
	if !ok {
		return scpCompressionFormat{}, fmt.Errorf("unsupported compression format %q, choose from: %s: %w",
			name, strings.Join(ScpCompressionFormats(), ", "), define.ErrInvalidArg)
	}
	return format, nil
}

// ValidateScpCompression checks the format is one podman image scp can apply and
// that the level, if any, is in range. The errors avoid flag names because this
// also runs on the API path.
func ValidateScpCompression(opts entities.ScpCompressionOptions) error {
	if opts.CompressionFormat == "" {
		if opts.CompressionLevel != nil {
			return fmt.Errorf("a compression level requires a compression format: %w", define.ErrInvalidArg)
		}
		return nil
	}

	format, err := scpCompressionFormatByName(opts.CompressionFormat)
	if err != nil {
		return err
	}

	if opts.CompressionLevel != nil && (*opts.CompressionLevel < format.minLevel || *opts.CompressionLevel > format.maxLevel) {
		return fmt.Errorf("compression level %d is out of range for %q, must be between %d and %d: %w",
			*opts.CompressionLevel, opts.CompressionFormat, format.minLevel, format.maxLevel, define.ErrInvalidArg)
	}

	return nil
}

// compressReader returns input compressed with the given format. Compression
// runs in a goroutine feeding a pipe, so the archive is never held in memory in
// full. The caller must close the returned reader.
func compressReader(input io.Reader, opts entities.ScpCompressionOptions) (io.ReadCloser, error) {
	// Not straight to c/image: it also compresses xz and zstd:chunked, which
	// podman image scp does not offer.
	if _, err := scpCompressionFormatByName(opts.CompressionFormat); err != nil {
		return nil, err
	}

	algorithm, err := compression.AlgorithmByName(opts.CompressionFormat)
	if err != nil {
		return nil, err
	}

	reader, writer := io.Pipe()
	compressor, err := compression.CompressStream(writer, algorithm, opts.CompressionLevel)
	if err != nil {
		_ = writer.Close()
		_ = reader.Close()
		return nil, err
	}

	go func() {
		_, err := io.Copy(compressor, input)
		// Closing the compressor flushes the trailer, so its error matters too.
		if closeErr := compressor.Close(); err == nil {
			err = closeErr
		}
		// Always close so a reader blocked in Read() is released.
		_ = writer.CloseWithError(err)
	}()

	return reader, nil
}

// remoteCompressCommand returns the compressor binary, the argv compressing
// remoteFile in place, and the path the compressed archive ends up at.
func remoteCompressCommand(remoteFile string, opts entities.ScpCompressionOptions) (bin string, argv []string, compressedFile string, err error) {
	format, err := scpCompressionFormatByName(opts.CompressionFormat)
	if err != nil {
		return "", nil, "", err
	}

	argv = make([]string, 0, len(format.args)+3)
	argv = append(argv, format.bin)
	argv = append(argv, format.args...)
	if opts.CompressionLevel != nil {
		argv = append(argv, "-"+strconv.Itoa(*opts.CompressionLevel))
	}
	argv = append(argv, remoteFile)

	return format.bin, argv, remoteFile + format.ext, nil
}

// cmdNotFoundStatus is what a POSIX shell exits with for an unknown command.
const cmdNotFoundStatus = 127

// compressRemoteFile compresses remoteFile in place on the host described by
// execOpts and returns the compressed path. The compressor removes remoteFile, so
// only the returned path needs cleaning up; a failure leaves nothing behind.
func compressRemoteFile(run remoteExec, execOpts ssh.ConnectionExecOptions, sshMode ssh.EngineMode, remoteFile string, opts entities.ScpCompressionOptions) (string, error) {
	bin, argv, compressedFile, err := remoteCompressCommand(remoteFile, opts)
	if err != nil {
		return "", err
	}

	compress := execOpts
	compress.Args = argv
	if _, err := run(&compress, sshMode); err != nil {
		// Either path can exist: the input is only removed on success, and a
		// partial output may already have been written.
		removeRemoteFiles(run, execOpts, sshMode, remoteFile, compressedFile)

		// Only 127 means the host lacks the compressor. Reporting anything else
		// that way would mislabel a failure to connect.
		if remoteExitStatus(err) == cmdNotFoundStatus {
			return "", fmt.Errorf("compressing the transfer archive with %q requires the %q command on the remote host: %w",
				opts.CompressionFormat, bin, err)
		}
		return "", fmt.Errorf("compressing %q on the remote host: %w", remoteFile, err)
	}

	return compressedFile, nil
}

// remoteExitStatus returns the status the remote command exited with, or -1 if
// err is not a command that ran to completion. The two ssh engines return
// different error types spelling the accessor differently. Matching the accessor
// rather than the type also keeps this testable: crypto/ssh's status field is
// unexported, so its ExitError cannot be built with a chosen status.
func remoteExitStatus(err error) int {
	var sshExit interface{ ExitStatus() int }
	if errors.As(err, &sshExit) {
		return sshExit.ExitStatus()
	}
	var cmdExit interface{ ExitCode() int }
	if errors.As(err, &cmdExit) {
		return cmdExit.ExitCode()
	}
	return -1
}
