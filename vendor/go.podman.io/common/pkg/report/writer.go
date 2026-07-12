package report

import (
	"io"
	"text/tabwriter"
)

// Writer aliases tabwriter.Writer to provide Podman defaults.
type Writer struct {
	*tabwriter.Writer
}

// NewWriter initializes a new report.Writer with given values.
func NewWriter(output io.Writer, minwidth, tabwidth, padding int, padchar byte, flags uint) (*Writer, error) {
	t := tabwriter.NewWriter(output, minwidth, tabwidth, padding, padchar, flags)
	return &Writer{t}, nil
}

// NewWriterDefault initializes a new report.Writer with Podman defaults.
func NewWriterDefault(output io.Writer) (*Writer, error) {
	return NewWriter(output, 12, 2, 2, ' ', 0)
}

// Flush any output left in buffers.
func (w *Writer) Flush() error {
	return w.Writer.Flush()
}
