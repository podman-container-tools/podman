// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"go/ast"
	"go/token"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Line is one preprocessed comment line ready for the lexer.
//
// Text has the Go comment markers (// /* */) stripped along with the godoc continuation decoration
// (leading whitespace, asterisks, slashes, optional markdown table pipe).
// Used for keyword and annotation classification.
//
// Raw is the same source line with only the comment marker removed — content whitespace,
// indentation, and list markers are preserved.
// Used by the body accumulator so YAML / nested-map indentation survives intact.
//
// Pos points to the first character of Text in the source file.
type Line struct {
	Text string
	Raw  string
	Pos  token.Position
}

//nolint:gochecknoglobals // this is fine to declare a replacer as private global
var eolReplacer = strings.NewReplacer("\r\n", "\n", "\r", "\n")

// Preprocess turns a Go comment group into a position-tagged []Line.
//
// Returns nil for nil inputs.
// Pure: no syscalls, no side-effects.
//
// Line endings are normalised before line splitting (\r\n → \n, lone \r → \n).
// See README §preprocess-contract.
func Preprocess(cg *ast.CommentGroup, fset *token.FileSet) []Line {
	if cg == nil || fset == nil {
		return nil
	}

	var out []Line
	for _, c := range cg.List {
		out = append(out, stripComment(eolReplacer.Replace(c.Text), fset.Position(c.Slash))...)
	}

	return out
}

// stripComment yields one Line per physical source line of one *ast.Comment.
//
// Handles both the // and /* */ forms.
func stripComment(raw string, basePos token.Position) []Line {
	const markerLen = 2 // "//" / "/*"
	switch {
	case strings.HasPrefix(raw, "//"):
		pos := basePos
		pos.Column += markerLen
		pos.Offset += markerLen
		return []Line{stripLine(raw[markerLen:], pos, stripSingleGodocSpace)}

	case strings.HasPrefix(raw, "/*"):
		body := strings.TrimSuffix(raw[markerLen:], "*/")
		out := []Line{}
		offset := 0
		for idx := 0; ; idx++ {
			nl := strings.IndexByte(body[offset:], '\n')

			var seg string
			if nl < 0 {
				seg = body[offset:]
			} else {
				seg = body[offset : offset+nl]
			}

			pos := basePos
			if idx == 0 {
				pos.Column += markerLen
				pos.Offset += markerLen
			} else {
				pos.Line += idx
				pos.Column = 1
				pos.Offset += markerLen + offset
			}
			out = append(out, stripLine(seg, pos, stripBlockContinuation))

			if nl < 0 {
				break
			}
			offset += nl + 1
		}
		return out

	default:
		return []Line{{Text: raw, Raw: raw, Pos: basePos}}
	}
}

func stripLine(s string, pos token.Position, rawStrip func(string) string) Line {
	// Apply the comment-kind decoration strip (block-comment `* ` continuation, or a no-op for `//`)
	// BEFORE the content-prefix trim, so the only leading `*` trimContentPrefix can see is a markdown
	// list bullet — never godoc decoration.
	//
	// This keeps `* item` bullets identifiable as lists without mangling block-comment framing
	// (go-swagger#1726).
	raw := rawStrip(s)
	stripped := trimContentPrefix(raw)
	consumed := len(s) - len(stripped)
	pos.Column += consumed
	pos.Offset += consumed
	return Line{Text: stripped, Raw: raw, Pos: pos}
}

// stripSingleGodocSpace is intentionally a no-op so Line.Raw preserves every character after the
// comment marker.
//
// Kept named so a future per-comment-kind raw-stripping strategy can slot in here.
func stripSingleGodocSpace(s string) string { return s }

// stripBlockContinuation removes the `\s*\*\s?` decoration godoc /* */ continuation lines carry,
// preserving all indentation otherwise.
func stripBlockContinuation(s string) string {
	leading := -1
	for i, r := range s {
		if !unicode.IsSpace(r) {
			leading = i
			break
		}
	}

	if leading < 0 || s[leading] != '*' { // also: empty string, all-blanks string, or no '*' continuation go there
		return s
	}

	s = s[leading+1:]
	r, offset := utf8.DecodeRuneInString(s)
	if unicode.IsSpace(r) {
		return s[offset:]
	}

	return s
}

// trimContentPrefix strips godoc-style leading decoration (indentation, slashes, an optional
// markdown table pipe) and normalises a markdown list bullet to the canonical YAML `- ` form.
//
// `*` is NOT in the strip set: block-comment `* ` continuation decoration is already removed by
// stripLine before this runs, so a leading `*`/`+` here is a markdown bullet, not decoration.
//
// Normalising `* item` / `+ item` to `- item` makes every downstream consumer that already
// understands `- ` (prose descriptions, Property.AsList, enum bodies) treat markdown-style and
// YAML-style lists identically, in one place (go-swagger#1726). gofmt performs the same `*`→`-`
// rewrite on // doc-comment bullets, so this also matches the gofmt-canonical source form.
//
// A leading `-` is preserved so the YAML fence `---` survives intact.
func trimContentPrefix(s string) string {
	s = strings.TrimLeft(s, " \t/")
	s = strings.TrimPrefix(s, "|")
	s = strings.TrimLeft(s, " \t")
	return normalizeBullet(s)
}

// normalizeBullet rewrites a leading markdown bullet marker (`* ` or `+ `) to the canonical `- `.
//
// The marker must be followed by a space (a CommonMark bullet), so `*emphasis*` and `**bold**`
// prose are left untouched.
func normalizeBullet(s string) string {
	if len(s) >= 2 && (s[0] == '*' || s[0] == '+') && s[1] == ' ' {
		return "- " + s[2:]
	}
	return s
}

// preprocessText handles raw text inputs (already stripped of comment markers) for ParseText /
// ParseAs.
//
// Line endings are normalised (\r\n → \n, lone \r → \n) before splitting.
func preprocessText(text string, basePos token.Position) []Line {
	text = normaliseLineEndings(text)
	rows := strings.Split(text, "\n")
	out := make([]Line, 0, len(rows))
	for i, r := range rows {
		pos := basePos
		pos.Line += i
		if i > 0 {
			pos.Column = 1
		}
		out = append(out, Line{Text: trimContentPrefix(r), Raw: r, Pos: pos})
	}
	return out
}

// normaliseLineEndings collapses \r\n and lone \r sequences into \n.
func normaliseLineEndings(s string) string {
	return eolReplacer.Replace(s)
}
