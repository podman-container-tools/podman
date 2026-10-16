// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"go/ast"
	"strings"
)

// The annotation keywords this package classifies. The vocabulary itself is the scanner's
// (see scanner.detectNodes); these are the three markers whose ARGUMENT is read here.
const (
	keywordModel      = "model"
	keywordResponse   = "response"
	keywordParameters = "parameters"
)

// ExtractAnnotation returns the trailing identifier of a `swagger:<name>` marker found anywhere on line, or ("", false)
// if no marker is present.
//
// Used by the scanner's annotation-classification index.
func ExtractAnnotation(line string) (string, bool) {
	return annotationName(line)
}

// ModelOverride returns the name argument of a `swagger:model <name>` marker found anywhere in comments, or ("", true)
// when the marker is present without an argument (bare `swagger:model`).
//
// Returns ("", false) when no marker is found.
func ModelOverride(comments *ast.CommentGroup) (string, bool) {
	return firstNamedOverride(comments, keywordModel, false)
}

// ResponseOverride returns the name argument of a `swagger:response <name>` marker, matching the bare-marker semantics
// of ModelOverride.
//
// The shared-namespace wildcard `*` is accepted as a synonym for the bare form.
func ResponseOverride(comments *ast.CommentGroup) (string, bool) {
	return firstNamedOverride(comments, keywordResponse, true)
}

// ParametersOverride returns every operation-id reference attached to a `swagger:parameters` marker.
//
// One marker can carry several operation ids; multiple markers across comments accumulate.
//
// The classification is deliberately permissive: the argument's CONTENT is parsed and validated by the grammar
// (grammar.ParametersBlock), which emits diagnostics for malformed forms, so a malformed-but-non-empty argument is
// classified and passed on rather than silently skipped here.
func ParametersOverride(comments *ast.CommentGroup) ([]string, bool) {
	var result []string

	for line := range commentLines(comments) {
		arg, ok := annotationArgument(line, keywordParameters)
		if !ok {
			continue
		}
		if trimmed := strings.TrimSpace(arg); trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result, len(result) > 0
}

// MalformedModelName reports a `swagger:model` marker on line whose name argument is not a plain identifier.
//
// Definition/response names are JSON labels, not Go-qualified identifiers, so a package-qualified name such as
// `utils.Error` is rejected by the strict name rule and the marker would otherwise be silently dropped.
//
// Returns the offending raw argument and true; a bare marker or a well-formed name returns ("", false).
func MalformedModelName(line string) (string, bool) {
	return malformedOverrideName(line, keywordModel, false)
}

// MalformedResponseName is the `swagger:response` counterpart of MalformedModelName.
func MalformedResponseName(line string) (string, bool) {
	return malformedOverrideName(line, keywordResponse, true)
}

// malformedOverrideName returns the raw name argument and true when line carries a single-name struct marker whose
// argument the strict name rule rejects.
//
// A bare marker (no argument) or a name the strict rule accepts returns ("", false).
func malformedOverrideName(line, keyword string, wildcard bool) (string, bool) {
	arg, ok := annotationArgument(line, keyword)
	if !ok {
		return "", false // bare marker or no argument
	}
	if _, strict := overrideName(line, keyword, wildcard); strict {
		return "", false // the strict rule accepts the name (incl. a trailing period)
	}

	return strings.TrimSpace(arg), true
}

// firstNamedOverride searches comments for a single-name struct marker and returns its name argument.
//
// When the marker is present but carries no argument, returns ("", true) so callers can distinguish "no marker" from
// "bare marker".
func firstNamedOverride(comments *ast.CommentGroup, keyword string, wildcard bool) (string, bool) {
	var found bool

	for line := range commentLines(comments) {
		name, ok := overrideName(line, keyword, wildcard)
		if !ok {
			continue
		}
		if name != "" {
			return name, true
		}
		found = true
	}

	return "", found
}

// commentLines iterates the raw lines of every comment in a group, in source order.
func commentLines(comments *ast.CommentGroup) func(func(string) bool) {
	return func(yield func(string) bool) {
		if comments == nil {
			return
		}
		for _, cmt := range comments.List {
			for line := range strings.SplitSeq(cmt.Text, "\n") {
				if !yield(line) {
					return
				}
			}
		}
	}
}
