package compression

import (
	"archive/zip"
	"errors"
	"io"

	"github.com/sirupsen/logrus"
)

type zipDecompressor struct {
	genericDecompressor
	zipReader  *zip.ReadCloser
	fileReader io.ReadCloser
}

func newZipDecompressor(compressedFilePath string) (*zipDecompressor, error) {
	d, err := newGenericDecompressor(compressedFilePath)
	if err != nil {
		return nil, err
	}
	return &zipDecompressor{*d, nil, nil}, err
}

// This is the only compressor that doesn't return the compressed file
// stream (zip.OpenReader() provides access to the decompressed file).
// As a result the progress bar shows the decompressed file stream
// but the final size is the compressed file size.
func (d *zipDecompressor) compressedFileReader() (io.ReadCloser, error) {
	zipReader, err := zip.OpenReader(d.compressedFilePath)
	if err != nil {
		return nil, err
	}
	d.zipReader = zipReader
	if len(zipReader.File) != 1 {
		return nil, errors.New("machine image files should consist of a single compressed file")
	}
	z, err := zipReader.File[0].Open()
	if err != nil {
		return nil, err
	}
	d.fileReader = z
	return z, nil
}

func (*zipDecompressor) decompress(w io.WriteSeeker, r io.Reader) error {
	_, err := io.Copy(w, r)
	return err
}

func (d *zipDecompressor) close() {
	if err := d.zipReader.Close(); err != nil {
		logrus.Errorf("Unable to close zip file: %q", err)
	}
	if err := d.fileReader.Close(); err != nil {
		logrus.Errorf("Unable to close zip file: %q", err)
	}
}
