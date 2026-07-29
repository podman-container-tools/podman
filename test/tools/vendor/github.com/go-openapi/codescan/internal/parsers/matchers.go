// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"go/ast"
	"regexp"
	"strings"
)

const minMatchCount = 2

// ExtractAnnotation returns the trailing identifier of a `swagger:<name>` marker found anywhere on
// line, or ("", false) if no marker is present.
//
// Used by the scanner's annotation-classification index.
func ExtractAnnotation(line string) (string, bool) {
	matches := rxSwaggerAnnotation.FindStringSubmatch(line)
	if len(matches) < minMatchCount {
		return "", false
	}

	return matches[1], true
}

// ModelOverride returns the name argument of a `swagger:model <name>` marker found anywhere in
// comments, or ("", true) when the marker is present without an argument (bare `swagger:model`).
//
// Returns ("", false) when no marker is found.
func ModelOverride(comments *ast.CommentGroup) (string, bool) {
	return commentBlankSubMatcher(rxModelOverride)(comments)
}

// ResponseOverride returns the name argument of a `swagger:response <name>` marker, matching the
// bare-marker semantics of ModelOverride.
func ResponseOverride(comments *ast.CommentGroup) (string, bool) {
	return commentBlankSubMatcher(rxResponseOverride)(comments)
}

// ParametersOverride returns every operation-id reference attached to a `swagger:parameters`
// marker.
//
// One marker can carry several operation ids; multiple markers across comments accumulate.
func ParametersOverride(comments *ast.CommentGroup) ([]string, bool) {
	return commentMultipleSubMatcher(rxParametersOverride)(comments)
}

// MalformedModelName reports a `swagger:model` marker on line whose name argument is not a plain
// identifier.
//
// Definition/response names are JSON labels, not Go-qualified identifiers, so a package-qualified
// name such as `utils.Error` is rejected by the strict rxModelOverride and the marker would
// otherwise be silently dropped.
//
// Returns the offending raw argument and true; a bare marker or a well-formed name returns ("",
// false).
func MalformedModelName(line string) (string, bool) {
	return malformedOverrideName(line, rxModelArg, rxModelOverride)
}

// MalformedResponseName is the `swagger:response` counterpart of MalformedModelName.
func MalformedResponseName(line string) (string, bool) {
	return malformedOverrideName(line, rxResponseArg, rxResponseOverride)
}

// malformedOverrideName returns the raw name argument and true when line carries a single-name
// struct marker (captured by rxArg) whose name the strict rxOverride rejects.
//
// A bare marker (no argument) or a name the strict matcher accepts returns ("", false).
func malformedOverrideName(line string, rxArg, rxOverride *regexp.Regexp) (string, bool) {
	m := rxArg.FindStringSubmatch(line)
	if len(m) < minMatchCount {
		return "", false // bare marker or no argument
	}
	if rxOverride.MatchString(line) {
		return "", false // strict matcher accepts the name (incl. a trailing period)
	}

	return strings.TrimSpace(m[1]), true
}

// commentBlankSubMatcher returns a matcher that searches comments for any line matching rx and
// returns the first non-blank submatch.
//
// When the marker is present but carries no argument, returns ("", true) so callers can distinguish
// "no marker" from "bare marker."
// See ModelOverride / ResponseOverride.
func commentBlankSubMatcher(rx *regexp.Regexp) func(*ast.CommentGroup) (string, bool) {
	return func(comments *ast.CommentGroup) (string, bool) {
		if comments == nil {
			return "", false
		}
		var found bool

		for _, cmt := range comments.List {
			for ln := range strings.SplitSeq(cmt.Text, "\n") {
				matches := rx.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					return strings.TrimSpace(matches[1]), true
				}
				if len(matches) > 0 {
					found = true
				}
			}
		}

		return "", found
	}
}

// commentMultipleSubMatcher returns a matcher that collects every non-blank submatch from comments,
// splitting whitespace-separated arguments into individual entries.
//
// See ParametersOverride.
func commentMultipleSubMatcher(rx *regexp.Regexp) func(*ast.CommentGroup) ([]string, bool) {
	return func(comments *ast.CommentGroup) ([]string, bool) {
		if comments == nil {
			return nil, false
		}

		var result []string
		for _, cmt := range comments.List {
			for ln := range strings.SplitSeq(cmt.Text, "\n") {
				matches := rx.FindStringSubmatch(ln)
				if len(matches) < minMatchCount {
					continue
				}
				trimmed := strings.TrimSpace(matches[1])
				if len(trimmed) == 0 {
					continue
				}

				result = append(result, trimmed)
			}
		}

		return result, len(result) > 0
	}
}
