package slogcolor

type SourceFileMode int

const (
	// Nop does nothing.
	Nop SourceFileMode = iota

	// ShortFile produces only the filename (for example main.go:39).
	ShortFile

	// LongFile produces the full file path (for example /home/user/go/src/myapp/main.go:39).
	LongFile
)
