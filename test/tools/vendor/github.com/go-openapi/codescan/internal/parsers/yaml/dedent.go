// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yaml

import "strings"

// RemoveIndent normalises the common leading indentation of a YAML body lifted from a godoc comment
// block.
//
// It runs in two passes:
//
//  1. Expand the leading tabs of every (non-blank) line to two spaces.
//     YAML refuses tab indentation, and gofmt renders an indented
//     doc-comment line (a code block) with a leading tab; expanding up
//     front makes tab- and space-indented lines comparable.
//  2. Treat the first non-blank line's indent length as the canonical
//     strip width and strip it from every line.
//
// The first-(non-blank-)line dedent (vs "shortest leading-whitespace run across every non-blank
// line") is the operations / meta path's contract — the existing operation goldens depend on it.
// The typed-extensions path uses common-prefix dedent instead; see README.md §dedent.
//
// Why expand BEFORE stripping (not the post-strip retab it replaced): gofmt rewrites a column-0
// doc-comment key as a prose line (one leading space) and its indented value block as a code block
// (a leading tab), separated by a blank "//" line.
//
// A swagger:meta body is uniformly tab-prefixed (quirk F7), but a swagger:operation body
// interleaves 1-space prose keys (`responses:`, `x-*:`) with tab-prefixed value blocks.
// Stripping the prose-keyed width (1) off a child's lone leading tab would consume the child's
// whole indent and flatten the nesting; the YAML parser then rejects or mis-nests the body.
//
// Expanding first turns each tab into two spaces, so the strip leaves the relative nesting intact
// for both shapes.
// Leading blank lines are skipped when choosing the canonical line (gofmt inserts one under a
// column-0 key).
//
// Whitespace tokens recognised here are space (' '), tab ('\t'), and the leading `/` characters
// that survive when the lexer hasn't stripped a godoc comment marker yet.
// Unicode space separators (\p{Zs}) are NOT recognised: real Go source code uses ASCII whitespace.
// If a corpus surfaces that depends on it, reintroduce the Unicode branch.
func RemoveIndent(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}

	// Expand leading tabs to two spaces on EVERY line first, so tab- and space-indented lines become
	// comparable before the common-indent strip. gofmt renders an indented doc-comment line (a code
	// block) with a leading tab, and YAML refuses tab indentation.
	//
	// A swagger:meta body is uniformly tab-prefixed (quirk F7), but a swagger:operation body
	// interleaves 1-space prose keys (`responses:`, `x-*:`) with tab-prefixed value blocks; retabbing
	// only the post-strip remainder (the previous approach) left the children's lone leading tab to be
	// stripped by the prose-keyed width, collapsing the nesting.
	//
	// Expanding up front, then stripping a pure-space width, preserves the relative nesting in both
	// shapes.
	expanded := make([]string, len(lines))
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			// Whitespace-only lines are blank separators with no content to indent; leave them untouched (a
			// lone tab here is not YAML indentation).
			expanded[i] = line
			continue
		}
		expanded[i] = retabLeading(line)
	}
	lines = expanded

	first := 0
	for first < len(lines) && strings.TrimSpace(lines[first]) == "" {
		first++
	}
	if first == len(lines) {
		return lines
	}

	indent := leadingIndent(lines[first])
	if indent == 0 {
		return lines
	}

	out := make([]string, len(lines))
	for i, line := range lines {
		if len(line) < indent {
			out[i] = line
			continue
		}
		out[i] = line[indent:]
	}
	return out
}

// leadingIndent returns the byte position of the first non-indent character on line — i.e., the
// number of bytes the line should be stripped by to remove its leading indent.
//
// An "indent" here is the maximal prefix matching the pattern (ws* / ws*)+ followed by ws*, where
// ws is space or tab.
// The optional `/` run absorbs godoc comment markers (`//`, `///`) when they slipped through
// preprocessing.
//
// Lines that are pure indent (empty or all-whitespace) return 0 — there's no meaningful strip
// count for them.
func leadingIndent(line string) int {
	i := 0
	for i < len(line) && isIndentSpace(line[i]) {
		i++
	}
	for i < len(line) && line[i] == '/' {
		i++
	}
	for i < len(line) && isIndentSpace(line[i]) {
		i++
	}
	if i >= len(line) {
		return 0
	}
	return i
}

// retabLeading replaces every tab in the leading-whitespace run of line with two spaces.
//
// Tabs past the first non-whitespace character are left untouched.
func retabLeading(line string) string {
	n := 0
	for n < len(line) && isIndentSpace(line[n]) {
		n++
	}
	if n == 0 {
		return line
	}
	return strings.ReplaceAll(line[:n], "\t", "  ") + line[n:]
}

func isIndentSpace(b byte) bool { return b == ' ' || b == '\t' }
