package utils

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

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
