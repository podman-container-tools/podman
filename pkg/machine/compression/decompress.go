package compression

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"go.podman.io/podman/v6/pkg/machine/define"
	"go.podman.io/podman/v6/utils"
	"go.podman.io/storage/pkg/archive"
)

const (
	decompressedFileFlag = os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	macOs                = "darwin"
	progressBarPrefix    = "Extracting compressed file"
	zipExt               = ".zip"
	magicNumberMaxBytes  = 10
)

type decompressor interface {
	compressedFileSize() int64
	compressedFileReader() (io.ReadCloser, error)
	decompress(w io.WriteSeeker, r io.Reader) error
	close()
}

func Decompress(compressedVMFile *define.VMFile, decompressedFilePath string) error {
	compressedFilePath := compressedVMFile.GetPath()
	compressedFileMagicNum, err := compressedVMFile.ReadMagicNumber(magicNumberMaxBytes)
	if err != nil {
		return err
	}

	var d decompressor
	if d, err = newDecompressor(compressedFilePath, compressedFileMagicNum); err != nil {
		return err
	}

	return runDecompression(d, decompressedFilePath)
}

func newDecompressor(compressedFilePath string, compressedFileMagicNum []byte) (decompressor, error) {
	compressionType := archive.DetectCompression(compressedFileMagicNum)
	hasZipSuffix := strings.HasSuffix(compressedFilePath, zipExt)

	switch {
	// Zip files are not guaranteed to have a magic number at the beginning
	// of the file, so we need to use the file name to detect them.
	case compressionType == archive.Uncompressed && hasZipSuffix:
		return newZipDecompressor(compressedFilePath)
	case compressionType == archive.Uncompressed:
		return newUncompressedDecompressor(compressedFilePath)
	default:
		return newGenericDecompressor(compressedFilePath)
	}
}

func runDecompression(d decompressor, decompressedFilePath string) (retErr error) {
	compressedFileReader, err := d.compressedFileReader()
	if err != nil {
		return err
	}
	defer d.close()

	filesize := d.compressedFileSize()
	// bar.ProxyReader() returns nil when the bar is finished.
	// When the size is set to 0 the bar will finfish immediately, but this happens
	// in a different goroutine so it is race and only happens sometimes.
	// In general if the input is an empty file then we do not have to display a
	// progress bar at all as there is nothing to copy/extract really
	// https://github.com/containers/podman/issues/23281
	if filesize > 0 {
		initMsg := progressBarPrefix + ": " + filepath.Base(decompressedFilePath)
		finalMsg := initMsg + ": done"

		p, bar := utils.ProgressBar(initMsg, filesize, finalMsg)
		// Wait for bars to complete and then shut down the bars container
		defer p.Wait()

		compressedFileReader = bar.ProxyReader(compressedFileReader)
		// Interrupts the bar goroutine. It's important that
		// bar.Abort(false) is called before p.Wait(), otherwise
		// can hang.
		defer bar.Abort(false)
	}

	var decompressedFileWriter *os.File

	if decompressedFileWriter, err = os.OpenFile(decompressedFilePath, decompressedFileFlag, 0o600); err != nil {
		logrus.Errorf("Unable to open destination file %s for writing: %q", decompressedFilePath, err)
		return err
	}
	defer func() {
		if err := decompressedFileWriter.Close(); err != nil {
			logrus.Warnf("Unable to close destination file %s: %q", decompressedFilePath, err)
			if retErr == nil {
				retErr = err
			}
		}
	}()

	if err = d.decompress(decompressedFileWriter, compressedFileReader); err != nil {
		logrus.Errorf("Error extracting compressed file: %q", err)
		return err
	}

	return nil
}
