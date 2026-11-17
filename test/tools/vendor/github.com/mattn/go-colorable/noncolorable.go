package colorable

import (
	"bytes"
	"io"
)

// NonColorable holds writer but removes escape sequence.
type NonColorable struct {
	out io.Writer
}

// NewNonColorable returns new instance of Writer which removes escape sequence from Writer.
func NewNonColorable(w io.Writer) io.Writer {
	return &NonColorable{out: w}
}

// Write writes data on console
func (w *NonColorable) Write(data []byte) (n int, err error) {
	for offset := 0; offset < len(data); {
		escape := bytes.IndexByte(data[offset:], 0x1b)
		if escape < 0 {
			w.out.Write(data[offset:])
			break
		}
		if escape > 0 {
			if n, err := w.out.Write(data[offset : offset+escape]); err != nil || n != escape {
				break
			}
		}
		offset += escape + 1
		if offset == len(data) {
			break
		}
		c2 := data[offset]
		offset++
		if c2 != 0x5b {
			continue
		}

		for offset < len(data) {
			c := data[offset]
			offset++
			if ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') || c == '@' {
				break
			}
		}
	}

	return len(data), nil
}
