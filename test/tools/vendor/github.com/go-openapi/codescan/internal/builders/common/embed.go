// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"go/ast"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
)

// EmbedInheritance carries the doc-comment directives an embedded (anonymous) struct field passes
// down to the members it promotes: the `in:` location and the `required:` flag.
//
// It is the shared kernel of the "annotations on an embed apply to its promoted members" rule, used
// by the schema, parameters, and responses builders so the behaviour is identical everywhere.
// See go-swagger#2701.
//
// Semantics:
//   - A member's own `in:`/`required:` always wins; the inherited value is
//     a fallback the member consults only when it sets nothing itself.
//   - An embed that omits a directive leaves the corresponding inherited
//     value unchanged, so nesting accumulates (an inner embed without its
//     own `in:` keeps the outer one).
//   - `in:` is meaningful only to the parameters and responses builders
//     (the schema builder has no location concept); `required:` is
//     meaningful to the parameters and schema builders (OAS2 response
//     headers carry no `required`).
type EmbedInheritance struct {
	In          string
	InSet       bool
	Required    bool
	RequiredSet bool
}

// ReadEmbedInheritance merges the `in:`/`required:` directives on an embedded field's doc comment
// into current, returning the context to pass down to the fields that embed promotes.
//
// The embed's own directives override the inherited ones; absent directives carry current through
// unchanged.
//
// doc is the embed field's *ast.CommentGroup (nil → current unchanged).
func (s *Builder) ReadEmbedInheritance(doc *ast.CommentGroup, current EmbedInheritance) EmbedInheritance {
	if doc == nil {
		return current
	}

	next := current
	if in, ok, _ := ScanInLocation(doc.Text()); ok {
		next.In = in
		next.InSet = true
	}
	if req, ok := s.ParseBlock(doc).GetBool(grammar.KwRequired); ok {
		next.Required = req
		next.RequiredSet = true
	}

	return next
}

// ScanInLocation finds the first `in: X` line in text and returns the canonical OAS v2 form of X
// (when recognised, case-insensitive via [grammar.NormalizeIn]) or the raw candidate (when present
// but out-of-vocabulary).
//
// The `form` alias is NOT accepted here — it is contained to the routes inline-param path.
//
// Shared by the parameters and responses field-signal scanners and by
// [Builder.ReadEmbedInheritance]; `in:` is line-scanned rather than read as a grammar Property
// because grammar attaches pre-annotation lines to the following annotation's prose, not its
// property list.
func ScanInLocation(text string) (value string, valid bool, invalid string) {
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "in:")
		if !ok {
			rest, ok = strings.CutPrefix(line, "In:")
		}
		if !ok {
			continue
		}
		v := strings.TrimSpace(rest)
		v = strings.TrimSuffix(v, ".")
		if v == "" {
			continue
		}
		if canonical, ok := grammar.NormalizeIn(v, false); ok {
			return canonical, true, ""
		}
		// First `in:` line with a non-vocab value — record so the caller can diagnose.
		// Don't keep scanning: a later valid `in:` after an invalid one would be a bizarre input we don't
		// need to model.
		return "", false, v
	}
	return "", false, ""
}
