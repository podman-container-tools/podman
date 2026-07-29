//go:build !windows

package cupwriter

import (
	"bytes"
	"io"

	"golang.org/x/sys/unix"
)

// Writer is a buffered terminal writer, which moves cursor N lines up
// on each flush except the first one, where N is a number of lines of
// a previous flush.
type Writer struct {
	*bytes.Buffer
	out      io.Writer
	ew       escWriter
	fd       int
	terminal bool
	forceTTY bool
}

// Flush flushes the underlying buffer.
// It's caller's responsibility to pass correct number of lines.
func (w *Writer) Flush(lines int) error {
	_, err := w.WriteTo(w.out)
	if err != nil {
		return err
	}

	if w.terminal || w.forceTTY {
		return w.ew.ansiCuuAndEd(w, lines)
	}

	return nil
}

// getTermSize returns the dimensions of the given terminal.
func getTermSize(fd int) (width, height int, err error) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return
	}
	return int(ws.Col), int(ws.Row), nil
}

// isTerminal returns whether the given file descriptor is a terminal.
func isTerminal(fd int) bool {
	_, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	return err == nil
}
