package runewidth

import (
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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
	zerowidth      table // combining + nonprint merged for faster zero-width lookup
	widewidth      table // ambiguous + doublewidth merged for EA path
	eastAsianWidth widthTable
	tablesOnce     sync.Once

	// strictWidthLUT is mostly built lazily on the first width lookup so
	// that importing the package costs neither the build time nor the 2 MB
	// of resident memory; see issue #104. Only the entries below
	// strictWidthLUTLimit are valid: init fills the first 0x300 entries of
	// both planes, and the lazy build fills the rest — never rewriting the
	// low region, so readers of it cannot race with the build. The limit
	// is loaded with acquire semantics, which makes the non-atomic reads
	// of the high region safe once it reports 0x110000. Keeping the whole
	// check down to one compare-and-branch matters: RuneWidth is only a
	// dozen instructions long.
	strictWidthLUT      [2][0x110000]byte
	strictWidthLUTLimit atomic.Int32
	strictWidthLUTOnce  sync.Once

	// joinerBits is the joiner table as a bitmap over
	// [joinerBase, 0x10000), the range where the per-rune test has to be
	// cheap for the fast paths in StringWidth and Wrap to pay off. It is
	// 7.6 KB and init fills it in a few microseconds. Runes above it stay
	// on the interval search: they are rare in the text these fast paths
	// are for, and the emoji that are common there leave the fast path at
	// their first joiner anyway.
	joinerBits [(0x10000 - joinerBase) / 8]byte
)

// joinerBase is the lowest rune in the joiner table. CR is the only rune
// below it that can form a multi-rune cluster, by pairing with LF.
const joinerBase = 0x300

func init() {
	initStrictWidthLUTLow()
	fillJoinerBits()
	strictWidthLUTLimit.Store(0x300)
	handleEnv()
}

// initStrictWidthLUTLow paints the first 0x300 entries of strictWidthLUT
// from the static interval tables. The result must stay identical to
// runeWidthNoLUT for runes below 0x300, which TestStrictWidthLUT verifies.
func initStrictWidthLUTLow() {
	for i := 0; i < 0x300; i++ {
		r := rune(i)
		w := byte(1)
		if r < 0x20 || (r >= 0x7F && r <= 0x9F) || r == 0xAD { // nonprint
			w = 0
		}
		strictWidthLUT[0][i] = w
	}

	ea := strictWidthLUT[1][:0x300]
	fillBytes(ea, 1)
	paint := func(t table, w byte) {
		for _, iv := range t {
			if iv.first >= 0x300 {
				break
			}
			last := iv.last
			if last > 0x2FF {
				last = 0x2FF
			}
			fillBytes(ea[iv.first:last+1], w)
		}
	}
	paint(ambiguous, 2)
	paint(doublewidth, 2)
	// zero-width wins over wide on overlap, so paint it last.
	paint(combining, 0)
	paint(nonprint, 0)
}

// initTables builds the merged lookup tables. It runs lazily through
// tablesOnce so that merely importing the package stays cheap; see issue
// #104.
func initTables() {
	zerowidth = mergeIntervals(combining, nonprint)
	widewidth = mergeIntervals(ambiguous, doublewidth)
	eastAsianWidth = makeWidthTable(zerowidth, widewidth)
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
	tablesOnce.Do(initTables)
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
		return int(strictWidthLUT[1][r])
	}
	if w, ok := inWidthTable(r, eastAsianWidth); ok {
		return w
	}
	if !strictEmojiNeutral && inTable(r, emoji) {
		return 2
	}
	return 1
}

// fillBytes sets every byte of b to v. It doubles the copied region on each
// iteration so large slices are filled at memcpy speed instead of one byte
// per loop iteration.
func fillBytes(b []byte, v byte) {
	if len(b) == 0 {
		return
	}
	b[0] = v
	for i := 1; i < len(b); i *= 2 {
		copy(b[i:], b[:i])
	}
}

// buildStrictWidthLUT builds the strict-width lookup table above 0x300
// exactly once. It paints whole intervals instead of computing every rune
// through the binary searches in runeWidthNoLUT. It must not write below
// 0x300: that region was filled by init and may be read concurrently. The
// result must stay identical to runeWidthNoLUT(r, eastAsian, true), which
// TestStrictWidthLUT verifies.
func buildStrictWidthLUT() {
	strictWidthLUTOnce.Do(func() {
		tablesOnce.Do(initTables)

		// paintHigh fills lut with w over each interval, clipped to 0x300+.
		paintHigh := func(lut []byte, first, last rune, w byte) {
			if first < 0x300 {
				if last < 0x300 {
					return
				}
				first = 0x300
			}
			fillBytes(lut[first:last+1], w)
		}

		// EastAsianWidth=false, StrictEmojiNeutral=true
		lut := strictWidthLUT[0][:]
		fillBytes(lut[0x300:], 1)
		for _, iv := range doublewidth {
			paintHigh(lut, iv.first, iv.last, 2)
		}
		// zerowidth is checked before doublewidth, so it wins on overlap.
		for _, iv := range zerowidth {
			paintHigh(lut, iv.first, iv.last, 0)
		}

		// EastAsianWidth=true, StrictEmojiNeutral=true
		lut = strictWidthLUT[1][:]
		fillBytes(lut[0x300:], 1)
		for _, iv := range eastAsianWidth {
			paintHigh(lut, iv.first, iv.last, iv.width)
		}

		strictWidthLUTLimit.Store(0x110000)
	})
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
	combinedLut []byte
	// The flags combinedLut was built from, so that CreateLUT can tell a
	// table that is still current from one that has to be rebuilt.
	lutEastAsianWidth     bool
	lutStrictEmojiNeutral bool

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
	// This one compare doubles as the range check and the lazy-LUT check:
	// out-of-range runes and runes above the built portion of
	// strictWidthLUT both take the slow path. Once the LUT is fully built
	// the limit is 0x110000 and only invalid runes go slow.
	if uint32(r) >= uint32(strictWidthLUTLimit.Load()) {
		return c.runeWidthSlow(r)
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

func (c *Condition) runeWidthSlow(r rune) int {
	if r < 0 || r > 0x10FFFF {
		return 0
	}
	buildStrictWidthLUT()
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
		if c.lutEastAsianWidth == c.EastAsianWidth && c.lutStrictEmojiNeutral == c.StrictEmojiNeutral {
			// The table still matches the flags, so rebuilding it
			// would produce the same bytes.
			return
		}
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
	c.lutEastAsianWidth = c.EastAsianWidth
	c.lutStrictEmojiNeutral = c.StrictEmojiNeutral
}

// isASCII reports whether s has no byte above 0x7F, in which case every
// grapheme cluster in s is a single byte apart from CRLF.
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// fillJoinerBits paints joinerBits from the joiner table.
func fillJoinerBits() {
	for _, iv := range joiner {
		if iv.first >= 0x10000 {
			break // the table is sorted, the rest is astral
		}
		lo, hi := int(iv.first)-joinerBase, int(iv.last)-joinerBase
		if hi >= len(joinerBits)*8 {
			hi = len(joinerBits)*8 - 1
		}
		for lo <= hi {
			if lo&7 == 0 && lo+7 <= hi {
				joinerBits[lo>>3] = 0xFF
				lo += 8
				continue
			}
			joinerBits[lo>>3] |= 1 << uint(lo&7)
			lo++
		}
	}
	// A byte that is not valid UTF-8 decodes to U+FFFD, and the segmenter
	// can gather a run of such bytes into one cluster, which a rune loop
	// has no way to see. Marking U+FFFD keeps those strings on the
	// segmenter, and the only string it holds back needlessly is one that
	// really contains U+FFFD.
	i := int(utf8.RuneError) - joinerBase
	joinerBits[i>>3] |= 1 << uint(i&7)
}

// isJoiner reports whether r can join with a neighbour into a multi-rune
// grapheme cluster. A string of runes for which it reports false has one
// rune per cluster, so measuring it needs no grapheme segmentation.
func isJoiner(r rune) bool {
	if r < joinerBase {
		return r == '\r'
	}
	if r < 0x10000 {
		i := r - joinerBase
		return joinerBits[i>>3]&(1<<(uint(i)&7)) != 0
	}
	return inTable(r, joiner)
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
	// Runes first: until one of them can join a cluster, each cluster is a
	// single rune and segmenting the string would only find that out the
	// expensive way.
	if w, ok := c.runeWidthSum(s); ok {
		width = w
		return
	}
	width = 0
	g := graphemes.FromString(s)
	for g.Next() {
		width += c.graphemeWidth(g.Value())
	}
	return
}

// runeWidthSum adds up the widths of the runes in s, and reports false
// without a total once it meets a rune that can join a cluster, which is
// where per-rune widths stop being the whole story.
func (c *Condition) runeWidthSum(s string) (int, bool) {
	width := 0
	for _, r := range s {
		if isJoiner(r) {
			return 0, false
		}
		width += c.RuneWidth(r)
	}
	return width, true
}

// Truncate return string truncated with w cells
func (c *Condition) Truncate(s string, w int, tail string) string {
	if c.StringWidth(s) <= w {
		return s
	}
	w -= c.StringWidth(tail)
	if pos, ok := c.truncateRunes(s, w); ok {
		return s[:pos] + tail
	}
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

// truncateRunes is the loop in Truncate with every rune taken for a whole
// cluster, which holds until a rune can join one. It reports false there
// and leaves the string to the segmenter.
func (c *Condition) truncateRunes(s string, w int) (int, bool) {
	width := 0
	for i, r := range s {
		if isJoiner(r) {
			return 0, false
		}
		cw := c.RuneWidth(r)
		if width+cw > w {
			return i, true
		}
		width += cw
	}
	return len(s), true
}

// endsCluster reports whether the byte at i, which follows a rune that
// cannot join a cluster, also starts one. Cutting there is only safe when
// the rune that follows does not reach back.
func endsCluster(s string, i int) bool {
	if i >= len(s) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return !isJoiner(r)
}

// TruncateLeft cuts w cells from the beginning of the `s`.
func (c *Condition) TruncateLeft(s string, w int, prefix string) string {
	if c.StringWidth(s) <= w {
		return prefix
	}

	if pos, pad, ok := c.truncateLeftRunes(s, w); ok {
		return prefix + strings.Repeat(" ", pad) + s[pos:]
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

// truncateLeftRunes is the loop in TruncateLeft with every rune taken for a
// whole cluster, returning the cut and the padding that replaces the cell
// the cut lands inside. It reports false at the first rune that can join a
// cluster.
func (c *Condition) truncateLeftRunes(s string, w int) (pos, pad int, ok bool) {
	width := 0
	for i, r := range s {
		if isJoiner(r) {
			return 0, 0, false
		}
		cw := c.RuneWidth(r)
		if width+cw > w {
			if width >= w {
				return i, 0, true
			}
			end := i + utf8.RuneLen(r)
			if !endsCluster(s, end) {
				return 0, 0, false
			}
			return end, width + cw - w, true
		}
		width += cw
	}
	return len(s), 0, true
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
	if pos, ok := c.truncatePrefixRunes(s, sw, w); ok {
		return prefix + s[pos:]
	}
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

// truncatePrefixRunes is the loop in TruncatePrefix with every rune taken
// for a whole cluster, reporting false at the first rune that can join one.
func (c *Condition) truncatePrefixRunes(s string, sw, w int) (int, bool) {
	width := 0
	for i, r := range s {
		if isJoiner(r) {
			return 0, false
		}
		cw := c.RuneWidth(r)
		if sw-(width+cw) <= w {
			end := i + utf8.RuneLen(r)
			if !endsCluster(s, end) {
				return 0, false
			}
			return end, true
		}
		width += cw
	}
	return 0, true
}

// wrapRunes wraps s treating every rune as its own grapheme cluster, and
// reports false without a result once it meets a rune that can join one.
func (c *Condition) wrapRunes(s string, w int) (string, bool) {
	width := 0
	var out strings.Builder
	out.Grow(len(s) + len(s)/max(w, 1) + 1)
	for _, r := range s {
		if isJoiner(r) {
			return "", false
		}
		cw := c.RuneWidth(r)
		if r == '\n' {
			out.WriteRune(r)
			width = 0
			continue
		}
		if width+cw > w {
			out.WriteByte('\n')
			width = 0
		}
		out.WriteRune(r)
		width += cw
	}
	return out.String(), true
}

// Wrap return string wrapped with w cells
func (c *Condition) Wrap(s string, w int) string {
	// ASCII fast path: no grapheme clustering needed for pure ASCII
	if isASCII(s) {
		width := 0
		var out strings.Builder
		// max keeps the capacity hint from dividing by zero when w is 0;
		// a non-positive width breaks before every cluster, as it always
		// has.
		out.Grow(len(s) + len(s)/max(w, 1) + 1)
		for i := 0; i < len(s); i++ {
			b := s[i]
			if b == '\n' {
				out.WriteByte(b)
				width = 0
				continue
			}
			// Same rule as the StringWidth fast path: no ASCII byte is
			// wide or ambiguous, so the flags in c cannot change this.
			cw := 0
			if b >= 0x20 && b != 0x7F {
				cw = 1
			}
			if width+cw > w {
				out.WriteByte('\n')
				width = 0
			}
			out.WriteByte(b)
			width += cw
		}
		return out.String()
	}
	// Runes first, as in StringWidth. Reaching a rune that can join a
	// cluster throws the wrapped text away and starts over on the
	// segmenter, which is the uncommon case.
	if out, ok := c.wrapRunes(s, w); ok {
		return out
	}
	width := 0
	var out strings.Builder
	out.Grow(len(s) + len(s)/max(w, 1) + 1)
	g := graphemes.FromString(s)
	for g.Next() {
		cluster := g.Value()
		// LF and CRLF are each a single cluster
		if strings.HasSuffix(cluster, "\n") {
			out.WriteString(cluster)
			width = 0
			continue
		}
		cw := c.graphemeWidth(cluster)
		if width+cw > w {
			out.WriteByte('\n')
			width = 0
		}
		out.WriteString(cluster)
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
// If flags in DefaultCondition are changed, CreateLUT should be called again;
// a call that finds the table already current is a no-op.
func CreateLUT() {
	DefaultCondition.CreateLUT()
}
