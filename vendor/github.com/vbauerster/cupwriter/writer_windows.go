//go:build windows

package cupwriter

import (
	"bytes"
	"fmt"
	"io"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procSetConsoleCursorPosition   = kernel32.NewProc("SetConsoleCursorPosition")
	procFillConsoleOutputCharacter = kernel32.NewProc("FillConsoleOutputCharacterA")
)

// Writer is a buffered terminal writer, which moves cursor N lines up
// on each flush except the first one, where N is a number of lines of
// a previous flush.
type Writer struct {
	*bytes.Buffer
	out      io.Writer
	ew       escWriter
	fd       int
	lines    int
	terminal bool
	forceTTY bool
}

// Flush flushes the underlying buffer.
// It's caller's responsibility to pass correct number of lines.
func (w *Writer) Flush(lines int) error {
	if w.terminal {
		err := w.clearLines()
		if err != nil {
			return err
		}
		w.lines = lines // save lines for the next clearLines
	}

	_, err := w.WriteTo(w.out)
	if err != nil {
		return err
	}

	if !w.terminal && w.forceTTY {
		return w.ew.ansiCuuAndEd(w, lines)
	}

	return nil
}

func (w *Writer) clearLines() error {
	if w.lines <= 0 {
		return nil
	}

	var info windows.ConsoleScreenBufferInfo
	err := windows.GetConsoleScreenBufferInfo(windows.Handle(w.fd), &info)
	if err != nil {
		return err
	}

	newPosition := info.CursorPosition

	if y := int(newPosition.Y); w.lines > y {
		w.lines = y
	} else {
		y -= w.lines
		newPosition.Y = int16(y)
	}

	// clear lines by writing space character n times starting at newPosition
	// if we don't some artefacts of a previous write may retain
	var r1 uintptr
	var written uint32
	n := uint32(info.Size.X) * uint32(w.lines)
	r1, _, err = procFillConsoleOutputCharacter.Call(
		uintptr(w.fd),
		uintptr(byte(' ')),
		uintptr(n),
		*(*uintptr)(unsafe.Pointer(&newPosition)),
		uintptr(unsafe.Pointer(&written)),
	)
	if r1 == 0 {
		return err
	}
	if written != n {
		return fmt.Errorf("FillConsoleOutputCharacterA: written != n (%d != %d)", written, n)
	}

	// move cursor to newPosition for the next write
	r1, _, err = procSetConsoleCursorPosition.Call(
		uintptr(w.fd),
		uintptr(uint32(uint16(newPosition.Y))<<16|uint32(uint16(newPosition.X))),
	)
	if r1 == 0 {
		return err
	}

	return nil
}

// getTermSize returns the visible dimensions of the given terminal.
// These dimensions don't include any scrollback buffer height.
func getTermSize(fd int) (width, height int, err error) {
	var info windows.ConsoleScreenBufferInfo
	err = windows.GetConsoleScreenBufferInfo(windows.Handle(fd), &info)
	if err != nil {
		return
	}
	// there is need to preserve space for a line ending with '\n'
	// therefore we don't +1 to the width as in reference implementation:
	// https://go.googlesource.com/crypto/+/refs/heads/release-branch.go1.14/ssh/terminal/util_windows.go#75
	// see https://github.com/vbauerster/mpb/issues/66
	return int(info.Window.Right - info.Window.Left), int(info.Window.Bottom - info.Window.Top + 1), nil
}

// isTerminal returns whether the given file descriptor is a terminal.
func isTerminal(fd int) bool {
	var st uint32
	err := windows.GetConsoleMode(windows.Handle(fd), &st)
	return err == nil
}
