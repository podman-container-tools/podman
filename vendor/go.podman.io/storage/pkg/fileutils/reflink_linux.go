package fileutils

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// Reflink attempts to reflink (CoW clone) the source to the destination fd.
// Returns an error if the filesystem does not support reflinks.
func Reflink(src, dst *os.File) error {
	return unix.IoctlFileClone(int(dst.Fd()), int(src.Fd()))
}

// ReflinkOrCopy attempts to reflink the source to the destination fd.
// If reflinking fails or is unsupported, it falls back to io.Copy().
func ReflinkOrCopy(src, dst *os.File) error {
	if err := Reflink(src, dst); err == nil {
		return nil
	}

	_, err := io.Copy(dst, src)
	return err
}
