package mpb

import (
	"io"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v8/decor"
	"github.com/vbauerster/mpb/v8/internal"
)

const (
	positionLeft = 1 + iota
	positionRight
)

var spinnerStyleComposer = SpinnerStyleComposer{
	frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
}

type spinnerFiller struct {
	frames   []string
	count    uint
	meta     func(string) string
	position func(string, int) string
}

// SpinnerStyleComposer is a builder which provides methods to build custom BarFiller.
// Call SpinnerStyle to construct a new one.
type SpinnerStyleComposer struct {
	position uint
	frames   []string
	meta     func(string) string
}

// SpinnerStyle constructs default SpinnerStyleComposer which implements
// BarFillerBuilder interface.
func SpinnerStyle(frames ...string) SpinnerStyleComposer {
	if len(frames) == 0 {
		return spinnerStyleComposer
	}
	return SpinnerStyleComposer{frames: frames}
}

func (s SpinnerStyleComposer) PositionLeft() SpinnerStyleComposer {
	s.position = positionLeft
	return s
}

func (s SpinnerStyleComposer) PositionRight() SpinnerStyleComposer {
	s.position = positionRight
	return s
}

func (s SpinnerStyleComposer) Meta(fn func(string) string) SpinnerStyleComposer {
	s.meta = fn
	return s
}

func (s SpinnerStyleComposer) ToBuilder() BarFillerBuilder {
	return s
}

func (s SpinnerStyleComposer) Build() BarFiller {
	sf := &spinnerFiller{frames: s.frames}
	switch s.position {
	case positionLeft:
		sf.position = func(frame string, padWidth int) string {
			return frame + strings.Repeat(" ", padWidth)
		}
	case positionRight:
		sf.position = func(frame string, padWidth int) string {
			return strings.Repeat(" ", padWidth) + frame
		}
	default:
		sf.position = func(frame string, padWidth int) string {
			return strings.Repeat(" ", padWidth/2) + frame + strings.Repeat(" ", padWidth/2+padWidth%2)
		}
	}
	if s.meta != nil {
		sf.meta = s.meta
	} else {
		sf.meta = func(s string) string { return s }
	}
	return sf
}

func (s *spinnerFiller) Fill(w io.Writer, stat decor.Statistics) error {
	width := internal.CheckRequestedWidth(stat.RequestedWidth, stat.AvailableWidth)
	frame := s.frames[s.count%uint(len(s.frames))]
	frameWidth := runewidth.StringWidth(frame)
	s.count++

	if width < frameWidth {
		return nil
	}

	_, err := io.WriteString(w, s.position(s.meta(frame), width-frameWidth))
	return err
}
