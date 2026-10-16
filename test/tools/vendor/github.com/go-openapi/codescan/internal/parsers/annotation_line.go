// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Scanning one comment line for an annotation.
//
// These are string searches for what were regular expressions until this file replaced them. The
// rules they enforce are unchanged and are stated on each function; the reason for spelling them
// out is that they are rules about where a keyword sits on a line, and reading them off a
// character walk is easier than reading them off a pattern.

// annotationKeyword prefixes every annotation.
const annotationKeyword = "swagger:"

// isCommentNoise reports whether r may precede an annotation as leading noise: a Unicode space
// separator, a tab, or one of the comment / list decorations `/`, `*`, `-`.
func isCommentNoise(r rune) bool {
	switch r {
	case '\t', '/', '*', '-':
		return true
	default:
		return unicode.Is(unicode.Zs, r)
	}
}

// isSpaceSeparator reports whether r is a Unicode space separator (which a tab is not).
func isSpaceSeparator(r rune) bool { return unicode.Is(unicode.Zs, r) }

// isNameRune reports whether r may appear in an annotation keyword or in an override name after
// its first character: a letter, a digit, a dash or a connector (`_`).
func isNameRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) ||
		unicode.Is(unicode.Pd, r) || unicode.Is(unicode.Pc, r)
}

// isASCIISpace reports whether r is one of the characters a regexp `\S` excludes.
//
// Deliberately not unicode.IsSpace: the argument matchers below reject a line whose argument starts
// with one of exactly these, and widening that would classify lines the previous rule did not.
func isASCIISpace(r rune) bool {
	switch r {
	case '\t', '\n', '\f', '\r', ' ':
		return true
	default:
		return false
	}
}

// afterCommentNoise returns the offset at which an annotation would have to start on line.
//
// It skips the leading run of comment noise, then AT MOST ONE markdown table pipe, then a further
// run of space separators.
//
// Annotations must START the comment line: any prose before the keyword disqualifies the line, so
// an annotation mentioned mid-sentence is not classified as one. The sole exception is
// `swagger:route`, which may follow a single godoc identifier — that allowance lives in the path
// annotation matchers (see rxRoutePrefix), not here.
func afterCommentNoise(line string) int {
	i := skipRunes(line, 0, isCommentNoise)
	if i < len(line) && line[i] == '|' {
		i = skipRunes(line, i+1, isSpaceSeparator)
	}

	return i
}

// skipRunes returns the offset of the first rune at or after i that keep rejects.
func skipRunes(s string, i int, keep func(rune) bool) int {
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !keep(r) {
			return i
		}
		i += size
	}

	return i
}

// annotationTail returns what follows `swagger:<keyword>` when line starts with that annotation,
// leading comment noise aside.
func annotationTail(line, keyword string) (string, bool) {
	rest := line[afterCommentNoise(line):]
	marker := annotationKeyword + keyword
	if !strings.HasPrefix(rest, marker) {
		return "", false
	}

	return rest[len(marker):], true
}

// overrideName reads the OPTIONAL name argument of a single-name struct marker on line.
//
// The shape is: the marker starting the line, space separators, an optional name, an optional
// trailing period (a marker written as the last word of a godoc sentence), and nothing else. A
// name is a letter followed by at least one further name character — a single letter is not a
// name, and neither is anything package-qualified, which is what makes MalformedModelName /
// MalformedResponseName able to report it instead of the marker being dropped in silence.
//
// wildcard admits `*` as a name, the shared-namespace synonym for the bare form that
// `swagger:response` accepts.
//
// Returns ("", true) for a bare marker, so callers can tell "no marker" from "marker, no argument".
func overrideName(line, keyword string, wildcard bool) (string, bool) {
	tail, ok := annotationTail(line, keyword)
	if !ok {
		return "", false
	}

	arg := strings.TrimSuffix(strings.TrimLeftFunc(tail, isSpaceSeparator), ".")
	switch {
	case arg == "":
		return "", true // bare marker
	case wildcard && arg == "*":
		return arg, true
	case isOverrideName(arg):
		return arg, true
	default:
		return "", false
	}
}

// isOverrideName reports whether s is a plain identifier-shaped name: a letter, then at least one
// further name character.
func isOverrideName(s string) bool {
	first, size := utf8.DecodeRuneInString(s)
	if !unicode.IsLetter(first) {
		return false
	}
	rest := s[size:]
	if rest == "" {
		return false
	}

	return skipRunes(rest, 0, isNameRune) == len(rest)
}

// annotationArgument reads the raw, unvalidated argument of a marker on line.
//
// The shape is: the marker starting the line, AT LEAST ONE space separator, then a non-empty
// argument that does not itself begin with a whitespace character, with trailing space separators
// dropped. The argument's content is not inspected — `swagger:parameters` hands it to the grammar,
// which diagnoses a malformed one rather than skipping it here, and the malformed-name detectors
// compare it against what overrideName accepts.
//
// One case reads differently from the expression this replaced, deliberately: that pattern could
// start the argument ON a space separator. Its `\S` excludes the ASCII whitespace characters and
// nothing else, so a non-breaking space passed it, and when what followed the separator run was a
// tab (or nothing at all) the pattern gave a separator back and began the argument there. The
// argument here begins where the separators end, or there is none. Nothing an author would write
// reaches the difference.
func annotationArgument(line, keyword string) (string, bool) {
	tail, ok := annotationTail(line, keyword)
	if !ok {
		return "", false
	}

	arg := strings.TrimLeftFunc(tail, isSpaceSeparator)
	if len(arg) == len(tail) {
		return "", false // no separator between the marker and its argument
	}

	arg = strings.TrimRightFunc(arg, isSpaceSeparator)
	if arg == "" {
		return "", false
	}
	if first, _ := utf8.DecodeRuneInString(arg); isASCIISpace(first) {
		return "", false
	}

	return arg, true
}

// annotationName returns the keyword of the first `swagger:<name>` marker on line.
//
// Deliberately loose — this is the classification search, not a gate: the marker may sit anywhere
// on the line provided it follows start-of-line, whitespace or a `/`. Do NOT use it as a block
// terminator; it fires on mid-prose mentions and would truncate descriptions.
func annotationName(line string) (string, bool) {
	for from := 0; from < len(line); {
		at := strings.Index(line[from:], annotationKeyword)
		if at < 0 {
			return "", false
		}
		at += from

		if at == 0 || isLooseBoundary(line[at-1]) {
			tail := line[at+len(annotationKeyword):]
			if end := skipRunes(tail, 0, isNameRune); end > 0 {
				return tail[:end], true
			}
		}
		from = at + 1
	}

	return "", false
}

// isLooseBoundary reports whether b may precede the keyword in the classification search.
//
// Byte-wise rather than rune-wise on purpose: every character it admits is ASCII, and a UTF-8
// continuation byte can never be one of them.
func isLooseBoundary(b byte) bool {
	switch b {
	case '\t', '\n', '\f', '\r', ' ', '/':
		return true
	default:
		return false
	}
}
