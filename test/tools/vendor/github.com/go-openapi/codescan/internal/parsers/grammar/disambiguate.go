// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Value-shape dispatch lives in this module so the lexer emits already-disambiguated typed tokens
// and the parser's productions stay context-free.
// See README §disambiguation.

// classifyDefaultValue chooses between JSON_VALUE and RAW_VALUE for the argument of
// swagger:default.
//
// Tries JSON_VALUE first (full JSON validation via the stdlib decoder), falling back to RAW_VALUE.
// The quick check uses the leading character; full JSON validation confirms.
//
// Returns either TokenJSONValue or TokenRawValue.
// The Pos is left to the caller to set.
func classifyDefaultValue(s string) TokenKind {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return TokenRawValue
	}
	switch trimmed[0] {
	case '"', '[', '{', '+', '-':
		if isJSONValue(trimmed) {
			return TokenJSONValue
		}
		return TokenRawValue
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		if isJSONValue(trimmed) {
			return TokenJSONValue
		}
		return TokenRawValue
	}
	switch trimmed {
	case "true", "false", "null":
		return TokenJSONValue
	}
	return TokenRawValue
}

// isJSONValue reports whether s is a valid JSON literal.
//
// Uses the stdlib JSON decoder for correctness.
func isJSONValue(s string) bool {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return false
	}
	if dec.More() {
		return false
	}
	return true
}

// enumArgsForm captures the four-way EnumArgs dispatch outcome.
//
// See README §disambiguation.
type enumArgsForm int

const (
	enumFormEmpty             enumArgsForm = iota // no inline argument (multi-line body may follow)
	enumFormBracketedOnly                         // EnumValuesOnly = EnumBracketedList
	enumFormPlainOnly                             // EnumValuesOnly = EnumPlainList
	enumFormNameOnly                              // EnumWithName, no values
	enumFormNamePlusBracketed                     // EnumWithName + EnumBracketedList
	enumFormNamePlusPlain                         // EnumWithName + EnumPlainList
)

// classifyEnumArgs implements the four-way dispatch on the trim-stripped argument string after
// `swagger:enum`.
//
// Returns the form plus the name (if any) and the verbatim values fragment (if any).
// The fragment is further parsed by classifyEnumValueList.
func classifyEnumArgs(arg string) (form enumArgsForm, name, values string) {
	s := strings.TrimSpace(arg)
	if s == "" {
		return enumFormEmpty, "", ""
	}
	if s[0] == '[' {
		return enumFormBracketedOnly, "", s
	}
	identEnd := scanEnumIdentifier(s)
	if identEnd == 0 {
		return enumFormPlainOnly, "", s
	}
	rest := strings.TrimLeft(s[identEnd:], " \t")
	if rest == "" {
		return enumFormNameOnly, s[:identEnd], ""
	}
	if rest[0] == '[' {
		return enumFormNamePlusBracketed, s[:identEnd], rest
	}
	return enumFormNamePlusPlain, s[:identEnd], rest
}

// scanEnumIdentifier returns the byte length of a leading IdentifierName: Letter followed by
// NameChar*.
//
// Stops at the first non-NameChar byte.
// Returns 0 if the leading character is not a letter.
func scanEnumIdentifier(s string) int {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 || !unicode.IsLetter(r) {
		return 0
	}
	i := size
	for i < len(s) {
		r, size = utf8.DecodeRuneInString(s[i:])
		if size == 0 {
			break
		}
		if !isIdentifierContinue(r) {
			break
		}
		i += size
	}
	return i
}

// isIdentifierContinue matches NameChar: Letter | Digit | "_" | "-" | ".".
func isIdentifierContinue(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) ||
		r == '_' || r == '-' || r == '.'
}

// enumValueListForm distinguishes the two surface forms.
//
//nolint:unused // full enum list values not wired yet
type enumValueListForm int

//nolint:unused // full enum list values not wired yet
const (
	enumListPlain     enumValueListForm = iota // comma-separated bare items
	enumListBracketed                          // [ … ]
)

// classifyEnumValueList decides whether values is a bracketed list or
// a plain list. The caller has already trim-stripped values; this
// function only inspects the leading character.
//
//nolint:unused // full enum list values not wired yet
func classifyEnumValueList(values string) enumValueListForm {
	trimmed := strings.TrimSpace(values)
	if strings.HasPrefix(trimmed, "[") {
		return enumListBracketed
	}
	return enumListPlain
}

// httpMethods is the closed vocabulary recognised as HTTP_METHOD.
//
//nolint:gochecknoglobals // closed-set lookup table.
var httpMethods = map[string]string{
	"get":     "GET",
	"post":    "POST",
	"put":     "PUT",
	"patch":   "PATCH",
	"head":    "HEAD",
	"delete":  "DELETE",
	"options": "OPTIONS",
	"trace":   "TRACE",
}

// classifyHTTPMethod returns the canonical method name and true iff s is a recognised HTTP method
// (case-insensitive).
func classifyHTTPMethod(s string) (string, bool) {
	canonical, ok := httpMethods[strings.ToLower(s)]
	return canonical, ok
}

// looksLikeTypeRef reports whether s is a well-formed `swagger:type` argument token — an
// optionally `[]`-prefixed (array), optionally dot-qualified Go-style identifier.
//
// It is a LEXICAL check only: the grammar no longer owns the closed type vocabulary.
// Semantic validity (is it a known keyword / scanned type? is the format compatible?) is resolved
// by the builder, which alone knows the scanned definitions and the annotated Go type (the F3
// reconciliation — see .claude/plans/quirks-F-series-fix.md).
//
// The lexer only rejects structurally malformed tokens (empty, embedded spaces, a bare `[]`, a
// leading digit, illegal characters), which the parser flags.
func looksLikeTypeRef(s string) bool {
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "[]") {
		s = s[2:]
	}
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			// always legal
		case (r >= '0' && r <= '9' || r == '.') && i > 0:
			// digits and dot-qualification legal after the first char
		default:
			return false
		}
	}
	return true
}

// looksLikeURLPath is a coarse check for the OperationArgs URL path position: must start with "/".
//
// Full RFC 3986 conformance is left to the analyzer / consumers; the lexer only needs to dispatch.
func looksLikeURLPath(s string) bool {
	return strings.HasPrefix(s, "/")
}
