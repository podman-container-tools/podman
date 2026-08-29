// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package godoclink rewrites godoc-specific syntax that reads as noise when a Go doc comment is
// carried into a Swagger title / description (the Options.CleanGoDoc feature).
//
// Two transforms apply to godoc-derived prose:
//
//   - resolution-free cleanup: reference-style link definition lines
//     (`[text]: url`) are dropped; godoc doc-link spans (`[Widget]`,
//     `[pkg.Type]`, `[Order.Field]`) have their brackets removed and the (leaf)
//     identifier humanized via the swag name mangler — e.g. `[CustName]` →
//     "cust name"; the first identifier of the prose is restored to sentence
//     case;
//   - idiom recomposition: when a [Resolver] maps a doc-link (or the leading
//     godoc-convention self-name) to an emitted schema, the span is replaced by
//     a [marker] carrying that schema's fully-qualified definition key. Markers
//     are resolved to the schema's final exposed name by [SubstituteMarkers],
//     run after the spec builder has reduced definition names. This two-step
//     dance is needed because the final name is only known at the very end of
//     the build, whereas which prose is godoc-derived is only known here, at the
//     consumption seam.
//
// With a nil [Options.Resolver] (and nil Self), only the resolution-free cleanup runs and no marker
// is produced.
//
// The recognizer regexes are adapted from the battle-tested
// github.com/fredbi/go-fred-mcp/pkg/doc-filters/godoc-filter; the key difference is that this
// package *rewrites* the prose whereas that tool *redacts* (length-preserving blanking) for
// masking.
package godoclink

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-openapi/swag/mangling"
)

// docLinkRE matches a godoc doc-link span: a dotted identifier chain (`[pkg.Type]`,
// `[Order.Field]`) OR an uppercase-led single (`[Widget]`), tolerating a leading `*`
// (`[*time.Time]`).
//
// Bare-lowercase singles (`[id]`), empty (`[]byte`), digit-led (`[0]`) and multi-word (`[see
// notes]`) spans are deliberately not matched, so ordinary prose brackets are left intact.
var docLinkRE = regexp.MustCompile(`\[\*?\w+(?:\.\w+)+\]|\[\*?[A-Z]\w*\]`)

// refDefLineRE matches a reference-style link definition line — `[text]: url` on its own line
// (optionally indented).
//
// Such a line is godoc/markdown link plumbing and carries no prose, so the whole line is dropped.
var refDefLineRE = regexp.MustCompile(`^[ \t]*\[[^\]]+\]:[ \t]+\S.*$`)

// multiNewlineRE collapses a 3+ newline run (left behind by dropping an interior
// reference-definition line) back to a single paragraph break.
var multiNewlineRE = regexp.MustCompile(`\n{3,}`)

// identRE matches a leading Go-identifier chain (the godoc-convention self-name at the start of a
// doc comment, e.g. "Widget" in "Widget does things").
var identRE = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*`)

// Resolution is the outcome of resolving a doc-link reference to an emitted schema.
//
// DefKey is the referenced type's fully-qualified definition key (whose final exposed name is
// substituted later); Suffix is the already-exposed field chain for a dotted member reference (e.g.
// ".customer_name"), or "" for a bare type reference.
type Resolution struct {
	DefKey string
	Suffix string
}

// Resolver maps a doc-link reference — the bracket content with any leading `*` stripped, e.g.
// "Order.CustName" or "pkg.Type" — to a [Resolution].
//
// It returns ok=false when the reference does not resolve to an emitted schema, in which case the
// caller humanizes the leaf identifier instead.
type Resolver func(ref string) (Resolution, bool)

// SelfRef describes the declaration whose godoc is being cleaned, so the leading godoc-convention
// self-name ("Widget" in "Widget does things") can be recomposed to the declaration's own exposed
// name.
//
// Name is the Go identifier; DefKey is its fully-qualified definition key.
type SelfRef struct {
	Name   string
	DefKey string
}

// Options configures Clean.
type Options struct {
	// Mangler humanizes leaf identifiers (and marker fallbacks).
	//
	// Required.
	Mangler *mangling.NameMangler
	// Resolver, when non-nil, recomposes resolvable doc-links into markers; nil selects
	// resolution-free cleanup only.
	Resolver Resolver
	// Self, when non-nil, enables leading self-name recomposition.
	//
	// It has effect only together with a non-nil Resolver.
	Self *SelfRef
}

// Clean applies godoc filtering to text.
//
// It is the caller's responsibility to apply Clean ONLY to godoc-derived prose — never to
// author-written swagger:title / swagger:description override text.
// Clean never mutates o.
func Clean(text string, o Options) string {
	if text == "" {
		return text
	}

	text = dropRefDefLines(text)
	text = rewrite(text, o)

	return text
}

// dropRefDefLines removes reference-style link definition lines, collapsing the blank runs a
// mid-text drop would otherwise leave and trimming trailing space.
func dropRefDefLines(text string) string {
	if !strings.Contains(text, "]:") {
		return text
	}

	lines := strings.Split(text, "\n")
	kept := make([]string, 0, len(lines))
	dropped := false
	for _, ln := range lines {
		if refDefLineRE.MatchString(ln) {
			dropped = true
			continue
		}
		kept = append(kept, ln)
	}
	if !dropped {
		return text
	}

	out := strings.Join(kept, "\n")
	// A dropped interior line can leave a 3+ newline run; fold to a paragraph break.
	// Trailing whitespace/newlines from a dropped final line are trimmed.
	out = multiNewlineRE.ReplaceAllString(out, "\n\n")

	return strings.TrimRight(out, " \t\n")
}

// span is one stretch of text to replace: a doc-link bracket span or the leading self-name.
type span struct {
	lo, hi int
	ref    string // resolver reference (full inner chain, `*` stripped); self-name = Self.Name
	leaf   string // trailing identifier, for the humanize fallback
	self   bool
}

// rewrite replaces every doc-link span (and, when enabled, the leading self-name) left to right,
// emitting a marker for resolvable references and the humanized leaf otherwise.
func rewrite(text string, o Options) string {
	spans := collectSpans(text, o)
	if len(spans) == 0 {
		return text
	}

	var b strings.Builder
	b.Grow(len(text))
	last := 0
	for _, s := range spans {
		if s.lo < last {
			continue // overlap guard (defensive; spans are non-overlapping)
		}
		b.WriteString(text[last:s.lo])
		b.WriteString(replacement(s, text, o))
		last = s.hi
	}
	b.WriteString(text[last:])

	return b.String()
}

// collectSpans gathers the leading self-name (when it matches Self.Name) and every doc-link bracket
// span, in ascending position order.
func collectSpans(text string, o Options) []span {
	var spans []span

	if o.Self != nil && o.Self.Name != "" {
		if loc := identRE.FindStringIndex(strings.TrimLeft(text, " \t")); loc != nil {
			offset := len(text) - len(strings.TrimLeft(text, " \t"))
			lo, hi := offset+loc[0], offset+loc[1]
			if text[lo:hi] == o.Self.Name {
				spans = append(spans, span{lo: lo, hi: hi, ref: o.Self.Name, leaf: o.Self.Name, self: true})
			}
		}
	}

	for _, loc := range docLinkRE.FindAllStringIndex(text, -1) {
		inner := strings.TrimPrefix(text[loc[0]+1:loc[1]-1], "*")
		spans = append(spans, span{lo: loc[0], hi: loc[1], ref: inner, leaf: leafOf(inner)})
	}

	return spans
}

// replacement computes the substitution text for one span: a marker when the reference resolves to
// a schema, otherwise the humanized leaf (sentence-cased at the start of the prose).
//
// A self-name without a Resolver is left verbatim (its exposed name is unknowable without
// resolution).
func replacement(s span, text string, o Options) string {
	titleize := isSentenceStart(text, s.lo)
	fallback := o.humanize(s.leaf)

	if o.Resolver != nil {
		if s.self {
			return encodeMarker(o.Self.DefKey, "", fallback, titleize)
		}
		if res, ok := o.Resolver(s.ref); ok {
			return encodeMarker(res.DefKey, res.Suffix, fallback, titleize)
		}
	} else if s.self {
		return text[s.lo:s.hi]
	}

	if titleize {
		return capitalizeFirst(fallback)
	}

	return fallback
}

// humanize renders an identifier as lower-case words; a nil mangler is treated as the identity
// (defensive — callers always supply one).
func (o Options) humanize(ident string) string {
	if o.Mangler == nil {
		return ident
	}

	return o.Mangler.ToHumanNameLower(ident)
}

// leafOf returns the trailing dot-separated segment of a reference chain (`inventory.Ledger` →
// "Ledger").
func leafOf(ref string) string {
	if i := strings.LastIndex(ref, "."); i >= 0 {
		return ref[i+1:]
	}

	return ref
}

// isSentenceStart reports whether pos is the first non-space byte of text — the sentence-initial
// position where a substituted name is restored to title case.
func isSentenceStart(text string, pos int) bool {
	for i := range pos {
		if !unicode.IsSpace(rune(text[i])) {
			return false
		}
	}

	return true
}

// capitalizeFirst upper-cases the first rune of s, leaving the rest untouched.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}

	return string(unicode.ToUpper(r)) + s[size:]
}
