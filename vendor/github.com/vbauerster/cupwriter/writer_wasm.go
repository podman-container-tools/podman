//go:build js || wasip1

package cupwriter

import (
	"bytes"
	"io"
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

	if w.forceTTY {
		return w.ew.ansiCuuAndEd(w, lines)
	}

	return nil
}

// getTermSize reports that terminal dimensions are unavailable on WebAssembly.
func getTermSize(fd int) (width, height int, err error) {
	return 0, 0, nil
}

// isTerminal reports that WebAssembly file descriptors are not terminals.
func isTerminal(fd int) bool {
	return false
}
