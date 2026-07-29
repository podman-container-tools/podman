package runewidth

import (
	"os"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
)

//go:generate go run script/generate.go

var (
	// EastAsianWidth will be set true if the current locale is CJK
	EastAsianWidth bool

	// StrictEmojiNeutral should be set false if handle broken fonts
	StrictEmojiNeutral bool = true

	// ZeroWidthJoiner is flag to set to use UTR#51 ZWJ.
	//
	// Deprecated: ZWJ sequences are always handled through Unicode
	// grapheme cluster segmentation now, so this flag has no effect.
	// It is kept only for compatibility with code written against
	// v0.0.9 and earlier.
	ZeroWidthJoiner bool

	// DefaultCondition is a condition in current locale
	DefaultCondition = &Condition{
		EastAsianWidth:     false,
		StrictEmojiNeutral: true,
	}
)

var (
	zerowidth       table // combining + nonprint merged for faster zero-width lookup
	widewidth       table // ambiguous + doublewidth merged for EA path
	eastAsianWidth  widthTable
	eastAsianWidth0 [0x300]byte
	strictWidthLUT  [2][0x110000]byte
)

func init() {
	zerowidth = mergeIntervals(combining, nonprint)
	widewidth = mergeIntervals(ambiguous, doublewidth)
	eastAsianWidth = makeWidthTable(zerowidth, widewidth)
	for r := range eastAsianWidth0 {
		eastAsianWidth0[r] = byte(runeWidthEastAsianNoCache(rune(r), true))
	}
	initStrictWidthLUT()
	handleEnv()
}

func mergeIntervals(t1, t2 table) table {
	merged := make(table, 0, len(t1)+len(t2))
	i, j := 0, 0
	for i < len(t1) && j < len(t2) {
		if t1[i].first <= t2[j].first {
			merged = append(merged, t1[i])
			i++
		} else {
			merged = append(merged, t2[j])
			j++
		}
	}
	merged = append(merged, t1[i:]...)
	merged = append(merged, t2[j:]...)
	if len(merged) == 0 {
		return merged
	}
	result := merged[:1]
	for _, iv := range merged[1:] {
		last := &result[len(result)-1]
		if iv.first <= last.last+1 {
			if iv.last > last.last {
				last.last = iv.last
			}
		} else {
			result = append(result, iv)
		}
	}
	return result
}

func handleEnv() {
	env := os.Getenv("RUNEWIDTH_EASTASIAN")
	if env == "" {
		EastAsianWidth = IsEastAsian()
	} else {
		EastAsianWidth = env == "1"
	}
	// update DefaultCondition
	if DefaultCondition.EastAsianWidth != EastAsianWidth {
		DefaultCondition.EastAsianWidth = EastAsianWidth
		if len(DefaultCondition.combinedLut) > 0 {
			DefaultCondition.combinedLut = DefaultCondition.combinedLut[:0]
			CreateLUT()
		}
	}
}

type interval struct {
	first rune
	last  rune
}

type table []interval

type widthInterval struct {
	first rune
	last  rune
	width byte
}

type widthTable []widthInterval

func inTable(r rune, t table) bool {
	if r < t[0].first {
		return false
	}
	if r > t[len(t)-1].last {
		return false
	}

	bot := 0
	top := len(t) - 1
	for top >= bot {
		mid := (bot + top) >> 1

		switch {
		case t[mid].last < r:
			bot = mid + 1
		case t[mid].first > r:
			top = mid - 1
		default:
			return true
		}
	}

	return false
}

func makeWidthTable(zero, two table) widthTable {
	wt := make(widthTable, 0, len(zero)+len(two))
	zi := 0
	for _, iv := range two {
		start := iv.first
		for zi < len(zero) && zero[zi].last < start {
			zi++
		}
		for i := zi; i < len(zero) && zero[i].first <= iv.last; i++ {
			if start < zero[i].first {
				wt = append(wt, widthInterval{start, zero[i].first - 1, 2})
			}
			if start <= zero[i].last {
				start = zero[i].last + 1
			}
			if start > iv.last {
				break
			}
		}
		if start <= iv.last {
			wt = append(wt, widthInterval{start, iv.last, 2})
		}
	}
	for _, iv := range zero {
		wt = append(wt, widthInterval{iv.first, iv.last, 0})
	}
	sort.Slice(wt, func(i, j int) bool {
		return wt[i].first < wt[j].first
	})
	return wt
}

func inWidthTable(r rune, t widthTable) (int, bool) {
	if r < t[0].first {
		return 0, false
	}
	if r > t[len(t)-1].last {
		return 0, false
	}

	bot := 0
	top := len(t) - 1
	for top >= bot {
		mid := (bot + top) >> 1

		switch {
		case t[mid].last < r:
			bot = mid + 1
		case t[mid].first > r:
			top = mid - 1
		default:
			return int(t[mid].width), true
		}
	}

	return 0, false
}

func runeWidthNoLUT(r rune, eastAsian, strictEmojiNeutral bool) int {
	if !eastAsian {
		if r < 0x20 {
			return 0
		}
		if (r >= 0x7F && r <= 0x9F) || r == 0xAD { // nonprint
			return 0
		}
		if r < 0x300 {
			return 1
		}
		switch {
		case inTable(r, zerowidth):
			return 0
		case inTable(r, doublewidth):
			return 2
		default:
			return 1
		}
	}

	if r < 0x300 {
		return int(eastAsianWidth0[r])
	}
	if w, ok := inWidthTable(r, eastAsianWidth); ok {
		return w
	}
	if !strictEmojiNeutral && inTable(r, emoji) {
		return 2
	}
	return 1
}

func runeWidthEastAsianNoCache(r rune, strictEmojiNeutral bool) int {
	if w, ok := inWidthTable(r, eastAsianWidth); ok {
		return w
	}
	if !strictEmojiNeutral && inTable(r, emoji) {
		return 2
	}
	return 1
}

func initStrictWidthLUT() {
	for i := range strictWidthLUT[0] {
		r := rune(i)
		strictWidthLUT[0][i] = byte(runeWidthNoLUT(r, false, true))
		strictWidthLUT[1][i] = byte(runeWidthNoLUT(r, true, true))
	}
}

var private = table{
	{0x00E000, 0x00F8FF}, {0x0F0000, 0x0FFFFD}, {0x100000, 0x10FFFD},
}

var nonprint = table{
	{0x0000, 0x001F}, {0x007F, 0x009F}, {0x00AD, 0x00AD},
	{0x070F, 0x070F}, {0x180B, 0x180E}, {0x200B, 0x200F},
	{0x2028, 0x202E}, {0x206A, 0x206F}, {0xD800, 0xDFFF},
	{0xFEFF, 0xFEFF}, {0xFFF9, 0xFFFB}, {0xFFFE, 0xFFFF},
}

// Condition have flag EastAsianWidth whether the current locale is CJK or not.
type Condition struct {
	combinedLut        []byte
	EastAsianWidth     bool
	StrictEmojiNeutral bool

	// Deprecated: ZWJ sequences are always handled through Unicode
	// grapheme cluster segmentation now, so this flag has no effect.
	// It is kept only for compatibility with code written against
	// v0.0.9 and earlier.
	ZeroWidthJoiner bool
}

// NewCondition return new instance of Condition which is current locale.
func NewCondition() *Condition {
	return &Condition{
		EastAsianWidth:     EastAsianWidth,
		StrictEmojiNeutral: StrictEmojiNeutral,
		ZeroWidthJoiner:    ZeroWidthJoiner,
	}
}

// RuneWidth returns the number of cells in r.
// See http://www.unicode.org/reports/tr11/
func (c *Condition) RuneWidth(r rune) int {
	if r < 0 || r > 0x10FFFF {
		return 0
	}
	if len(c.combinedLut) > 0 {
		return int(c.combinedLut[r>>1]>>(uint(r&1)*4)) & 3
	}
	if c.StrictEmojiNeutral {
		if c.EastAsianWidth {
			return int(strictWidthLUT[1][r])
		}
		return int(strictWidthLUT[0][r])
	}
	return runeWidthNoLUT(r, c.EastAsianWidth, c.StrictEmojiNeutral)
}

// CreateLUT will create an in-memory lookup table of 557056 bytes for faster operation.
// This should not be called concurrently with other operations on c.
// If options in c is changed, CreateLUT should be called again.
func (c *Condition) CreateLUT() {
	const max = 0x110000
	lut := c.combinedLut
	if len(c.combinedLut) != 0 {
		// Remove so we don't use it.
		c.combinedLut = nil
	} else {
		lut = make([]byte, max/2)
	}
	for i := range lut {
		i32 := int32(i * 2)
		x0 := c.RuneWidth(i32)
		x1 := c.RuneWidth(i32 + 1)
		lut[i] = uint8(x0) | uint8(x1)<<4
	}
	c.combinedLut = lut
}

// graphemeWidth returns the width of a single grapheme cluster: the sum of
// the widths of its runes, capped at 2 cells. The cap keeps multi-rune
// sequences that render as a single glyph (ZWJ emoji, flags, Hangul jamo)
// from being counted wider than the two cells terminals give them.
func (c *Condition) graphemeWidth(cluster string) int {
	width := 0
	for _, r := range cluster {
		width += c.RuneWidth(r)
	}
	if width > 2 {
		width = 2
	}
	return width
}

// StringWidth return width as you can see
func (c *Condition) StringWidth(s string) (width int) {
	if len(s) == 1 {
		b := s[0]
		if b < 0x20 || b == 0x7F {
			return 0
		}
		return 1
	}
	if len(s) > 0 && len(s) <= utf8.UTFMax {
		r, size := utf8.DecodeRuneInString(s)
		if size == len(s) {
			return c.RuneWidth(r)
		}
	}
	// ASCII fast path: no grapheme clustering needed for pure ASCII
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 0x80 {
			goto graphemes
		}
		if b >= 0x20 && b != 0x7F {
			width++
		}
	}
	return

graphemes:
	width = 0
	g := graphemes.FromString(s)
	for g.Next() {
		width += c.graphemeWidth(g.Value())
	}
	return
}

// Truncate return string truncated with w cells
func (c *Condition) Truncate(s string, w int, tail string) string {
	if c.StringWidth(s) <= w {
		return s
	}
	w -= c.StringWidth(tail)
	var width int
	pos := len(s)
	g := graphemes.FromString(s)
	for g.Next() {
		chWidth := c.graphemeWidth(g.Value())
		if width+chWidth > w {
			pos = g.Start()
			break
		}
		width += chWidth
	}
	return s[:pos] + tail
}

// TruncateLeft cuts w cells from the beginning of the `s`.
func (c *Condition) TruncateLeft(s string, w int, prefix string) string {
	if c.StringWidth(s) <= w {
		return prefix
	}

	var width int
	pos := len(s)

	g := graphemes.FromString(s)
	for g.Next() {
		chWidth := c.graphemeWidth(g.Value())

		if width+chWidth > w {
			if width < w {
				pos = g.End()
				prefix += strings.Repeat(" ", width+chWidth-w)
			} else {
				pos = g.Start()
			}

			break
		}

		width += chWidth
	}

	return prefix + s[pos:]
}

// TruncatePrefix cuts the beginning of `s` so the result fits in w cells, with prefix prepended
func (c *Condition) TruncatePrefix(s string, w int, prefix string) string {
	if c.StringWidth(prefix) >= w {
		return prefix
	}

	sw := c.StringWidth(s)
	if sw <= w {
		return s
	}
	w -= c.StringWidth(prefix)
	var width int
	var pos int
	g := graphemes.FromString(s)
	for g.Next() {
		chWidth := c.graphemeWidth(g.Value())
		if sw-(width+chWidth) <= w {
			pos = g.End()
			break
		}
		width += chWidth
	}

	return prefix + s[pos:]
}

// Wrap return string wrapped with w cells
func (c *Condition) Wrap(s string, w int) string {
	width := 0
	var out strings.Builder
	out.Grow(len(s) + len(s)/w + 1)
	for _, r := range s {
		cw := c.RuneWidth(r)
		if r == '\n' {
			out.WriteRune(r)
			width = 0
			continue
		} else if width+cw > w {
			out.WriteByte('\n')
			width = 0
			out.WriteRune(r)
			width += cw
			continue
		}
		out.WriteRune(r)
		width += cw
	}
	return out.String()
}

// FillLeft return string filled in left by spaces in w cells
func (c *Condition) FillLeft(s string, w int) string {
	width := c.StringWidth(s)
	count := w - width
	if count > 0 {
		return strings.Repeat(" ", count) + s
	}
	return s
}

// FillRight return string filled in left by spaces in w cells
func (c *Condition) FillRight(s string, w int) string {
	width := c.StringWidth(s)
	count := w - width
	if count > 0 {
		return s + strings.Repeat(" ", count)
	}
	return s
}

// RuneWidth returns the number of cells in r.
// See http://www.unicode.org/reports/tr11/
func RuneWidth(r rune) int {
	return DefaultCondition.RuneWidth(r)
}

// IsAmbiguousWidth returns whether is ambiguous width or not.
func IsAmbiguousWidth(r rune) bool {
	return inTable(r, private) || inTable(r, ambiguous)
}

// IsCombiningWidth returns whether is combining width or not.
func IsCombiningWidth(r rune) bool {
	return inTable(r, combining)
}

// IsNeutralWidth returns whether is neutral width or not.
func IsNeutralWidth(r rune) bool {
	return inTable(r, neutral)
}

// StringWidth return width as you can see
func StringWidth(s string) (width int) {
	return DefaultCondition.StringWidth(s)
}

// Truncate return string truncated with w cells
func Truncate(s string, w int, tail string) string {
	return DefaultCondition.Truncate(s, w, tail)
}

// TruncateLeft cuts w cells from the beginning of the `s`.
func TruncateLeft(s string, w int, prefix string) string {
	return DefaultCondition.TruncateLeft(s, w, prefix)
}

// TruncatePrefix cuts the beginning of `s` so the result fits in w cells, with prefix prepended
func TruncatePrefix(s string, w int, prefix string) string {
	return DefaultCondition.TruncatePrefix(s, w, prefix)
}

// Wrap return string wrapped with w cells
func Wrap(s string, w int) string {
	return DefaultCondition.Wrap(s, w)
}

// FillLeft return string filled in left by spaces in w cells
func FillLeft(s string, w int) string {
	return DefaultCondition.FillLeft(s, w)
}

// FillRight return string filled in left by spaces in w cells
func FillRight(s string, w int) string {
	return DefaultCondition.FillRight(s, w)
}

// CreateLUT will create an in-memory lookup table of 557055 bytes for faster operation.
// This should not be called concurrently with other operations.
func CreateLUT() {
	if len(DefaultCondition.combinedLut) > 0 {
		return
	}
	DefaultCondition.CreateLUT()
}
