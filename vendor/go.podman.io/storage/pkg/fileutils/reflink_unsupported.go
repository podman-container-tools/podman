//go:build !linux

package fileutils

import (
	"errors"
	"io"
	"os"
)

// Reflink attempts to reflink (CoW clone) the source to the destination fd.
// Returns an error if the filesystem does not support reflinks.
func Reflink(src, dst *os.File) error {
	return errors.ErrUnsupported
}

// ReflinkOrCopy attempts to reflink the source to the destination fd.
// If reflinking fails or is unsupported, it falls back to io.Copy().
func ReflinkOrCopy(src, dst *os.File) error {
	_, err := io.Copy(dst, src)
	return err
}
