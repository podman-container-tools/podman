// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package routebody

import (
	"go/token"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
)

// ResponseDecl is one parsed response line from a swagger:route `Responses:` body.
//
// Code is the line's <code> head — "default" (case-insensitive) or a decimal HTTP status code
// string.
// BodyTypeRef / ResponseRef are mutually exclusive: at most one is non-empty.
// Arrays carries the number of `[]` prefixes stripped from the ref target (0 for a scalar ref).
//
// Description is the post-tag prose tail.
//
// An empty-value line (`204:` with nothing after the colon) produces a ResponseDecl with Code set
// and every other field zero.
// The orchestrator emits the response with an explicitly empty description.
type ResponseDecl struct {
	Code        string
	BodyTypeRef string
	ResponseRef string
	Arrays      int
	Description string
	Pos         token.Position
}

// ParseResponses lowers a Responses: raw block body into typed response lines.
//
// See package godoc for the grammar spec.
//
// basePos is the source position of the `responses:` keyword head; each line's Pos is offset by the
// line number within body (1-indexed) so diagnostics point at the offending line.
//
// diag may be nil; when nil, diagnostics are dropped.
func ParseResponses(body string, basePos token.Position, diag func(grammar.Diagnostic)) []ResponseDecl {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	var out []ResponseDecl
	lines := strings.Split(body, "\n")
	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		pos := offsetPos(basePos, lineNo)

		before, after, ok0 := strings.Cut(line, ":")
		if !ok0 {
			emitDiagf(diag, pos,
				"response line %q has no `:` separator", line)
			continue
		}

		code := strings.TrimSpace(before)
		if code == "" {
			emitDiagf(diag, pos,
				"response line missing status code before `:`")
			continue
		}

		value := strings.TrimSpace(after)
		decl, ok := parseResponseValue(code, value, pos, diag)
		if !ok {
			continue
		}
		out = append(out, decl)
	}

	return out
}

// parseResponseValue tokenises the right-hand side of a response line and lowers it into a
// ResponseDecl.
//
// Empty value yields an empty-body Decl carrying just the code.
func parseResponseValue(code, value string, pos token.Position, diag func(grammar.Diagnostic)) (ResponseDecl, bool) {
	decl := ResponseDecl{Code: code, Pos: pos}
	if value == "" {
		return decl, true
	}

	tokens := strings.Fields(value)
	descTokens := []string{}
	seenBodyOrResponse := false

	for i, tok := range tokens {
		tag, val, isTagged := splitTagToken(tok)
		switch {
		case isTagged && tag == "body":
			if seenBodyOrResponse {
				emitDiagf(diag, pos,
					"response line %q: duplicate body/response tag", code+": "+value)
				return ResponseDecl{}, false
			}
			seenBodyOrResponse = true
			decl.BodyTypeRef, decl.Arrays = stripArrayPrefixes(val)
		case isTagged && tag == "response":
			if seenBodyOrResponse {
				emitDiagf(diag, pos,
					"response line %q: duplicate body/response tag", code+": "+value)
				return ResponseDecl{}, false
			}
			seenBodyOrResponse = true
			decl.ResponseRef, decl.Arrays = stripArrayPrefixes(val)
		case isTagged && tag == "description":
			// `description:Foo bar baz` — value is everything after the colon on this token, joined with
			// subsequent tokens as raw prose.
			// Skip the empty val that arises from a bare `description:` token (val=="") so the joined result
			// does not lead with a stray space.
			if val != "" {
				descTokens = append(descTokens, val)
			}
			if i < len(tokens)-1 {
				descTokens = append(descTokens, tokens[i+1:]...)
			}
			decl.Description = strings.Join(descTokens, " ")
			return decl, true
		case isTagged:
			emitDiagf(diag, pos,
				"response line %q: unknown tag %q", code+": "+value, tag)
			return ResponseDecl{}, false
		default:
			// Untagged token.
			// If the first untagged token is literally "body" or "response", treat it as a typo for
			// `body:Foo` / `response:Foo` (missing colon) and drop the line with a diagnostic rather than
			// silently parsing it as a ref named "body" / "response".
			if i == 0 && (tok == "body" || tok == "response") {
				emitDiagf(diag, pos,
					"response line %q: missing `:` after %q — write `%s:Foo` not `%s Foo`",
					code+": "+value, tok, tok, tok)
				return ResponseDecl{}, false
			}
			if i == 0 {
				// First untagged token is the response ref candidate.
				// The orchestrator resolves it against the responses map first and falls back to definitions
				// (treating the hit as a body ref).
				//
				// `[]` prefixes apply just as on tagged refs, so the orchestrator can wrap arrays around the
				// resolved body schema.
				seenBodyOrResponse = true
				decl.ResponseRef, decl.Arrays = stripArrayPrefixes(tok)
				continue
			}
			descTokens = append(descTokens, tok)
		}
	}

	if len(descTokens) > 0 {
		decl.Description = strings.Join(descTokens, " ")
	}
	return decl, true
}

// splitTagToken splits a single `tag:value` token.
//
// Returns (tag, value, true) when the colon is present; (_, _, false) otherwise.
// The split takes only the FIRST colon — anything after it is the value.
func splitTagToken(tok string) (tag, value string, ok bool) {
	before, after, ok := strings.Cut(tok, ":")
	if !ok {
		return "", "", false
	}
	return before, after, true
}

// stripArrayPrefixes counts leading `[]` prefixes on a body/response ref token.
//
// Returns (name, arrayCount).
// `[][]Foo` → ("Foo", 2).
func stripArrayPrefixes(ref string) (string, int) {
	arrays := 0
	for strings.HasPrefix(ref, "[]") {
		arrays++
		ref = ref[2:]
	}
	return ref, arrays
}
