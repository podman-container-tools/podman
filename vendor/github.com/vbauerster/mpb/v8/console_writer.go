package mpb

import "io"

// ConsoleWriter interface.
// mpb package interacts with terminal via ConsoleWriter interface.
// Custom implementation can be provided via WithConsoleWriter ContainerOption.
type ConsoleWriter interface {
	io.Writer
	io.ReaderFrom
	IsTerminal() bool
	GetTermSize() (width, height int, err error)
	Flush(lines int) error
}
