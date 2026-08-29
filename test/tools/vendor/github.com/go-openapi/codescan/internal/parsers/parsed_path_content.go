// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	rxStripComments = regexp.MustCompile(`^[^\p{L}\p{N}\p{Pd}\p{Pc}\+]*`)
	rxSpace         = regexp.MustCompile(`\p{Zs}+`)
)

type ParsedPathContent struct {
	Method, Path, ID string
	Tags             []string
	Remaining        *ast.CommentGroup

	// Pos is the position of the matched route/operation annotation line (the comment group's start).
	//
	// Used to anchor diagnostics such as the stripped-regex warning.
	// Pos is the (coarse) source position of the matched route/operation annotation line — the
	// comment's Slash.
	//
	// Used as:
	// - anchor for diagnostics, such as the stripped-regex warning.
	// - the cross-ref anchor for the /paths/{path}/{method} node. Invalid when no annotation matched.
	Pos token.Pos

	// StrippedParams names the path parameters whose inline regex constraint (gorilla/chi style, e.g.
	// `{id:[0-9]+}`) was stripped to the bare `{id}` template form.
	//
	// OpenAPI 2.0 path templating supports only RFC 6570 URI Template Level-1 expansion (simple
	// `{var}` substitution), so any `:regex` constraint is dropped and surfaced to the author by the
	// route/operation builder.
	StrippedParams []string
}

// stripPathParamRegex rewrites inline-regex path-parameter segments (`{name:regex}`) to the bare
// RFC 6570 Level-1 form (`{name}`) and returns the cleaned string alongside the names whose
// constraint was stripped.
//
// Brace matching is depth-aware so regex quantifiers carrying their own braces (`{id:[0-9]{2,4}}`)
// are handled.
// A plain `{name}` template (no colon) and unbalanced braces are left untouched.
func stripPathParamRegex(s string) (cleaned string, stripped []string) {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '{' {
			b.WriteByte(s[i])
			i++

			continue
		}

		// Find the brace that closes the one at i, counting depth so nested `{...}` (regex quantifiers)
		// don't terminate early.
		depth, j := 0, i
		for ; j < len(s); j++ {
			switch s[j] {
			case '{':
				depth++
			case '}':
				depth--
			}
			if depth == 0 {
				break
			}
		}
		if j >= len(s) {
			// Unbalanced — emit the remainder verbatim.
			b.WriteString(s[i:])

			break
		}

		inner := s[i+1 : j]
		if colon := strings.IndexByte(inner, ':'); colon > 0 {
			name := inner[:colon]
			b.WriteByte('{')
			b.WriteString(name)
			b.WriteByte('}')
			stripped = append(stripped, name)
		} else {
			b.WriteString(s[i : j+1]) // plain `{name}` (or `{:..}`): keep as-is
		}
		i = j + 1
	}

	return b.String(), stripped
}

func ParseOperationPathAnnotation(lines []*ast.Comment) (cnt ParsedPathContent) {
	return parsePathAnnotation(rxOperation, lines)
}

func ParseRoutePathAnnotation(lines []*ast.Comment) (cnt ParsedPathContent) {
	return parsePathAnnotation(rxRoute, lines)
}

// ensureCommentMarker returns line with a leading `// ` prepended unless it already starts with
// `//` or `/*`.
//
// Used by parsePathAnnotation when reshaping a multi-line block comment into per-line *ast.Comment
// entries; the grammar lexer only runs its content-prefix strip on the `//` / `/*` branches of
// stripComment.
func ensureCommentMarker(line string) string {
	if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") {
		return line
	}
	return "// " + line
}

// stripBlockFraming removes the `/* … */` markers from a block comment's full text, leaving the
// inner lines.
//
// A `swagger:operation` written in a `/* */` block comment arrives as a single *ast.Comment whose
// Text still carries the framing; without stripping it, the closing `*/` (and the empty opener
// line) leak into the reconstructed Remaining block, where a stray `*/` reads as a YAML alias
// indicator and fails the body parse (go-swagger#1595).
//
// Returns the text unchanged for `//` comments.
func stripBlockFraming(text string) string {
	if !strings.HasPrefix(text, "/*") {
		return text
	}
	return strings.TrimSuffix(strings.TrimPrefix(text, "/*"), "*/")
}

// stripBlockContinuation removes the `\s*\*\s?` godoc decoration a `/* */` continuation line may
// carry, preserving all other indentation (so YAML body indentation under a block-comment
// swagger:operation survives).
//
// It mirrors grammar.stripBlockContinuation; the duplication keeps the scanner-level parsers
// package free of a dependency on the grammar sub-package.
// A line with no `*` decoration (the flush-left block style) is returned untouched.
func stripBlockContinuation(s string) string {
	leading := -1
	for i, r := range s {
		if !unicode.IsSpace(r) {
			leading = i
			break
		}
	}
	if leading < 0 || s[leading] != '*' {
		return s
	}
	s = s[leading+1:]
	r, offset := utf8.DecodeRuneInString(s)
	if unicode.IsSpace(r) {
		return s[offset:]
	}
	return s
}

func parsePathAnnotation(annotation *regexp.Regexp, lines []*ast.Comment) (cnt ParsedPathContent) {
	const routeTagsIndex = 3 // routeTagsIndex is the regex submatch index where route tags begin.
	var justMatched bool

	for _, cmt := range lines {
		txt := cmt.Text
		isBlock := strings.HasPrefix(txt, "/*")
		txt = stripBlockFraming(txt)
		for line := range strings.SplitSeq(txt, "\n") {
			if isBlock {
				// Shed the godoc `* ` continuation decoration so a `*`-styled block comment's body lines don't
				// carry a leading `*` into Remaining / the YAML body.
				// Indentation is preserved for flush-left block bodies (go-swagger#1595).
				line = stripBlockContinuation(line)
			}
			// Strip inline-regex path-param constraints (`{id:[0-9]+}`) to the RFC 6570 Level-1 form
			// (`{id}`) BEFORE matching: rxPath's alphabet has no `[`/`]`, so the raw line would fail to
			// match and the route would be dropped silently.
			// The original `line` is preserved for the Remaining block.
			cleaned, stripped := stripPathParamRegex(line)
			matches := annotation.FindStringSubmatch(cleaned)
			if len(matches) > routeTagsIndex {
				cnt.Method, cnt.Path, cnt.ID = matches[1], matches[2], matches[len(matches)-1]
				cnt.Tags = rxSpace.Split(matches[3], -1)
				if len(matches[3]) == 0 {
					cnt.Tags = nil
				}
				cnt.Pos = cmt.Slash
				cnt.StrippedParams = stripped
				justMatched = true

				continue
			}

			if cnt.Method == "" {
				continue
			}

			if cnt.Remaining == nil {
				cnt.Remaining = new(ast.CommentGroup)
			}

			if !justMatched || strings.TrimSpace(rxStripComments.ReplaceAllString(line, "")) != "" {
				cc := new(ast.Comment)
				cc.Slash = cmt.Slash
				// Force a `//` prefix on the synthetic per-line comment so grammar's lexer sees a shape it
				// strips (the `//` branch of stripComment runs trimContentPrefix, which sheds leading `
				// \t*/|`).
				//
				// Without the prefix, the lexer falls through its default case and preserves the source line
				// verbatim — leading tabs from `/* ... */` block- comment route docs then leak into Title /
				// Description.
				//
				// Lines that already start with `//` or `/*` are left alone so their leading whitespace is
				// recorded correctly via the matching strip path.
				cc.Text = ensureCommentMarker(line)
				cnt.Remaining.List = append(cnt.Remaining.List, cc)
				justMatched = false
			}
		}
	}

	return cnt
}
