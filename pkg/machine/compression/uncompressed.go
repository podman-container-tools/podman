package compression

import (
	"io"
)

type uncompressedDecompressor struct {
	genericDecompressor
}

func newUncompressedDecompressor(compressedFilePath string) (*uncompressedDecompressor, error) {
	d, err := newGenericDecompressor(compressedFilePath)
	if err != nil {
		return nil, err
	}
	return &uncompressedDecompressor{*d}, err
}

func (d *uncompressedDecompressor) decompress(w io.WriteSeeker, r io.Reader) error {
	return d.sparseOptimizedCopy(w, r)
}
