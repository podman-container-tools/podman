package slogcolor

import (
	"log/slog"
	"time"

	"github.com/fatih/color"
)

// DefaultLevelTags are the default level tags.
var DefaultLevelTags = map[slog.Level]string{
	slog.LevelDebug: color.New(color.BgCyan, color.FgHiWhite).Sprint("DEBUG"),
	slog.LevelInfo:  color.New(color.BgGreen, color.FgHiWhite).Sprint("INFO "),
	slog.LevelWarn:  color.New(color.BgYellow, color.FgHiWhite).Sprint("WARN "),
	slog.LevelError: color.New(color.BgRed, color.FgHiWhite).Sprint("ERROR"),
}

// DefaultOptions are the default options.
var DefaultOptions *Options = &Options{
	Level:         slog.LevelInfo,
	TimeFormat:    time.DateTime,
	SrcFileMode:   ShortFile,
	SrcFileLength: 0,
	MsgPrefix:     color.HiWhiteString("| "),
	MsgLength:     0,
	MsgColor:      color.New(),
	NoColor:       false,
	NoTime:        false,
	LevelTags:     DefaultLevelTags,
}

// Options represents the options passed into [NewHandler].
type Options struct {
	// Level reports the minimum level to log.
	// Levels with lower levels are discarded.
	// If nil, the Handler uses [slog.LevelInfo].
	Level slog.Leveler

	// TimeFormat is the time format.
	TimeFormat string

	// SrcFileMode is the source file mode.
	SrcFileMode SourceFileMode

	// SrcFileLength to show fixed length filename to line up the log output, default 0 shows complete filename.
	SrcFileLength int

	// MsgPrefix to show prefix before message, default: white colored "| ".
	MsgPrefix string

	// MsgColor is the color of the message, default to empty.
	MsgColor *color.Color

	// MsgLength to show fixed length message to line up the log output, default 0 shows complete message.
	MsgLength int

	// NoColor disables color, default: false.
	NoColor bool

	// NoTime disables time, default: false.
	NoTime bool

	// LevelTags is level tag for message, default: DefaultLevelStyles
	LevelTags map[slog.Level]string

	// ValueFormatter formats attribute values before they are written.
	// If nil, [slog.Value.String] is used.
	ValueFormatter func(slog.Value) string
}
