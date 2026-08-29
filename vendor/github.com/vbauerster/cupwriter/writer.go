package cupwriter

import (
	"bytes"
	"io"
	"os"
	"strconv"
)

const (
	// https://github.com/dylanaraps/pure-sh-bible#cursor-movement
	escOpen  = "\x1b["
	cuuAndEd = "A\x1b[J"
)

// New constructs a *Writer which abstracts writing multi lines to a fixed
// position within a terminal.
func New(out io.Writer, forceTTY bool) *Writer {
	bb := make([]byte, 16)
	w := &Writer{
		Buffer:   new(bytes.Buffer),
		out:      out,
		ew:       escWriter(bb[:copy(bb, []byte(escOpen))]),
		forceTTY: forceTTY,
	}
	if f, ok := out.(*os.File); ok {
		w.SetTermFd(int(f.Fd()))
	}
	return w
}

// SetTermFd sets fd if only it stands for an actual terminal handle.
// If it is indeed a terminal handle then next (*Writer).IsTerminal
// call will return true and (*Writer).GetTermSize is safe to call.
// Use case: preserve terminal behaviour while constructing *Writer
// with io.Writer wrapper.
//
//	cw := New(io.MultiWriter(os.Stdout, &someTestBuf), false)
//	cw.IsTerminal() // returns false
//	cw.SetTermFd(int(os.Stdout.Fd()))
//	cw.IsTerminal() // returns true
func (w *Writer) SetTermFd(fd int) {
	if isTerminal(fd) {
		w.fd = fd
		w.terminal = true
	} else {
		w.terminal = false
	}
}

// IsTerminal tells whether underlying io.Writer is terminal aka TTY.
func (w *Writer) IsTerminal() bool {
	return w.terminal
}

// GetTermSize returns width and height of underlying terminal.
// Should be called only if (*Writer).IsTerminal returns true.
func (w *Writer) GetTermSize() (width, height int, err error) {
	return getTermSize(w.fd)
}

type escWriter []byte

func (b escWriter) ansiCuuAndEd(out io.Writer, n int) error {
	// some terminals interpret 'cursor up 0' as 'cursor up 1'
	// therefore ignore n <= 0 case
	if n <= 0 {
		return nil
	}
	b = strconv.AppendInt(b, int64(n), 10)
	_, err := out.Write(append(b, []byte(cuuAndEd)...))
	return err
}
