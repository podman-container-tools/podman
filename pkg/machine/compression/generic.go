package compression

import (
	"io"
	"io/fs"
	"os"
	"runtime"

	"github.com/sirupsen/logrus"
	"go.podman.io/image/v5/pkg/compression"
)

type genericDecompressor struct {
	compressedFilePath string
	compressedFile     *os.File
	compressedFileInfo os.FileInfo
}

func newGenericDecompressor(compressedFilePath string) (*genericDecompressor, error) {
	d := &genericDecompressor{}
	d.compressedFilePath = compressedFilePath
	stat, err := os.Stat(d.compressedFilePath)
	if err != nil {
		return nil, err
	}
	d.compressedFileInfo = stat
	return d, nil
}

func (d *genericDecompressor) compressedFileSize() int64 {
	return d.compressedFileInfo.Size()
}

func (d *genericDecompressor) compressedFileMode() fs.FileMode {
	return d.compressedFileInfo.Mode()
}

func (d *genericDecompressor) compressedFileReader() (io.ReadCloser, error) {
	compressedFile, err := os.Open(d.compressedFilePath)
	if err != nil {
		return nil, err
	}
	d.compressedFile = compressedFile
	return compressedFile, nil
}

func (d *genericDecompressor) decompress(w io.WriteSeeker, r io.Reader) error {
	decompressedFileReader, _, err := compression.AutoDecompress(r)
	if err != nil {
		return err
	}
	defer func() {
		if err := decompressedFileReader.Close(); err != nil {
			logrus.Errorf("Unable to close gz file: %q", err)
		}
	}()

	// Use sparse-optimized copy for macOS as applehv,
	// macOS native hypervisor, uses large raw VM disk
	// files mostly empty (~2GB of content ~8GB empty).
	if runtime.GOOS == macOs {
		err = d.sparseOptimizedCopy(w, decompressedFileReader)
	} else {
		_, err = io.Copy(w, decompressedFileReader)
	}

	return err
}

func (d *genericDecompressor) close() {
	if err := d.compressedFile.Close(); err != nil {
		logrus.Errorf("Unable to close compressed file: %q", err)
	}
}

func (d *genericDecompressor) sparseOptimizedCopy(w io.WriteSeeker, r io.Reader) error {
	var err error
	sparseWriter := NewSparseWriter(w)
	defer func() {
		e := sparseWriter.Close()
		if e != nil && err == nil {
			err = e
		}
	}()
	_, err = io.Copy(sparseWriter, r)
	return err
}
