package mpb

import (
	"io"

	"github.com/mattn/go-runewidth"
	"github.com/vbauerster/mpb/v8/decor"
	"github.com/vbauerster/mpb/v8/internal"
)

const (
	iLbound = iota
	iRefiller
	iFiller
	iPadding
	iRbound
	iLen
)

var barStyleComposer = BarStyleComposer{
	style:     [iLen]string{"[", "+", "=", "-", "]"},
	tipFrames: []string{">"},
}

type component struct {
	width int
	bytes []byte
}

type barSection struct {
	meta  func(string) string
	bytes []byte
}

type barSections [iLen + 1]barSection

type barFiller struct {
	components [iLen]component
	metas      [iLen + 1]func(string) string
	flushOp    func(barSections, io.Writer) error
	tip        struct {
		onComplete bool
		count      uint
		frames     []component
	}
}

// BarStyleComposer is a builder which provides methods to build custom BarFiller.
// Call BarStyle to construct a new one.
type BarStyleComposer struct {
	style         [iLen]string
	metas         [iLen + 1]func(string) string
	tipFrames     []string
	tipOnComplete bool
	rev           bool
}

// BarStyle constructs default BarStyleComposer which implements
// BarFillerBuilder interface.
func BarStyle() BarStyleComposer {
	return barStyleComposer
}

func (s BarStyleComposer) Lbound(bound string) BarStyleComposer {
	s.style[iLbound] = bound
	return s
}

func (s BarStyleComposer) LboundMeta(fn func(string) string) BarStyleComposer {
	s.metas[iLbound] = fn
	return s
}

func (s BarStyleComposer) Rbound(bound string) BarStyleComposer {
	s.style[iRbound] = bound
	return s
}

func (s BarStyleComposer) RboundMeta(fn func(string) string) BarStyleComposer {
	s.metas[iRbound] = fn
	return s
}

func (s BarStyleComposer) Filler(filler string) BarStyleComposer {
	s.style[iFiller] = filler
	return s
}

func (s BarStyleComposer) FillerMeta(fn func(string) string) BarStyleComposer {
	s.metas[iFiller] = fn
	return s
}

func (s BarStyleComposer) Refiller(refiller string) BarStyleComposer {
	s.style[iRefiller] = refiller
	return s
}

func (s BarStyleComposer) RefillerMeta(fn func(string) string) BarStyleComposer {
	s.metas[iRefiller] = fn
	return s
}

func (s BarStyleComposer) Padding(padding string) BarStyleComposer {
	s.style[iPadding] = padding
	return s
}

func (s BarStyleComposer) PaddingMeta(fn func(string) string) BarStyleComposer {
	s.metas[iPadding] = fn
	return s
}

func (s BarStyleComposer) Tip(frames ...string) BarStyleComposer {
	if len(frames) != 0 {
		s.tipFrames = frames
	}
	return s
}

func (s BarStyleComposer) TipMeta(fn func(string) string) BarStyleComposer {
	s.metas[iLen] = fn
	return s
}

func (s BarStyleComposer) TipOnComplete() BarStyleComposer {
	s.tipOnComplete = true
	return s
}

func (s BarStyleComposer) Reverse() BarStyleComposer {
	s.rev = true
	return s
}

func (s BarStyleComposer) ToBuilder() BarFillerBuilder {
	return s
}

func (s BarStyleComposer) Build() BarFiller {
	bf := &barFiller{metas: s.metas}
	bf.components[iLbound] = component{
		width: runewidth.StringWidth(s.style[iLbound]),
		bytes: []byte(s.style[iLbound]),
	}
	bf.components[iRbound] = component{
		width: runewidth.StringWidth(s.style[iRbound]),
		bytes: []byte(s.style[iRbound]),
	}
	bf.components[iFiller] = component{
		width: runewidth.StringWidth(s.style[iFiller]),
		bytes: []byte(s.style[iFiller]),
	}
	bf.components[iRefiller] = component{
		width: runewidth.StringWidth(s.style[iRefiller]),
		bytes: []byte(s.style[iRefiller]),
	}
	bf.components[iPadding] = component{
		width: runewidth.StringWidth(s.style[iPadding]),
		bytes: []byte(s.style[iPadding]),
	}
	bf.tip.onComplete = s.tipOnComplete
	bf.tip.frames = make([]component, 0, len(s.tipFrames))
	for _, t := range s.tipFrames {
		bf.tip.frames = append(bf.tip.frames, component{
			width: runewidth.StringWidth(t),
			bytes: []byte(t),
		})
	}
	if s.rev {
		bf.flushOp = barSections.flushRev
	} else {
		bf.flushOp = barSections.flush
	}
	return bf
}

func (s *barFiller) Fill(w io.Writer, stat decor.Statistics) error {
	width := internal.CheckRequestedWidth(stat.RequestedWidth, stat.AvailableWidth)
	// don't count brackets as progress
	width -= (s.components[iLbound].width + s.components[iRbound].width)
	if width < 0 {
		return nil
	}

	var tip component
	var refilling, filling, padding []byte
	var fillCount int
	curWidth := int(internal.PercentageRound(stat.Total, stat.Current, int64(width)))

	if curWidth != 0 {
		if !stat.Completed || s.tip.onComplete {
			tip = s.tip.frames[s.tip.count%uint(len(s.tip.frames))]
			s.tip.count++
			fillCount += tip.width
		}
		switch refWidth := 0; {
		case stat.Refill != 0:
			refWidth = int(internal.PercentageRound(stat.Total, stat.Refill, int64(width)))
			curWidth -= refWidth
			refWidth += curWidth
			fallthrough
		default:
			for w := s.components[iFiller].width; curWidth-fillCount >= w; fillCount += w {
				filling = append(filling, s.components[iFiller].bytes...)
			}
			for w := s.components[iRefiller].width; refWidth-fillCount >= w; fillCount += w {
				refilling = append(refilling, s.components[iRefiller].bytes...)
			}
		}
	}

	for w := s.components[iPadding].width; width-fillCount >= w; fillCount += w {
		padding = append(padding, s.components[iPadding].bytes...)
	}

	for w := 1; width-fillCount >= w; fillCount += w {
		padding = append(padding, "…"...)
	}

	return s.flushOp(barSections{
		{s.metas[iLbound], s.components[iLbound].bytes},
		{s.metas[iRefiller], refilling},
		{s.metas[iFiller], filling},
		{s.metas[iLen], tip.bytes},
		{s.metas[iPadding], padding},
		{s.metas[iRbound], s.components[iRbound].bytes},
	}, w)
}

func (s barSection) flush(w io.Writer) (err error) {
	if s.meta != nil {
		_, err = io.WriteString(w, s.meta(string(s.bytes)))
	} else {
		_, err = w.Write(s.bytes)
	}
	return err
}

func (bb barSections) flush(w io.Writer) error {
	for _, s := range bb {
		err := s.flush(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bb barSections) flushRev(w io.Writer) error {
	bb[0], bb[len(bb)-1] = bb[len(bb)-1], bb[0]
	for i := len(bb) - 1; i >= 0; i-- {
		err := bb[i].flush(w)
		if err != nil {
			return err
		}
	}
	return nil
}
