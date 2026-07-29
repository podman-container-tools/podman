// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package responses

import (
	"go/ast"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/common"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
)

// fieldDocSignals carries the per-field doc-comment signals the response dispatcher reads upstream
// of the schema build.
//
// Shape parallels parameters/doc_signals.go's; the two will fold into a shared resolvers helper in
// M6.
//
// # Details
//
// See [§in-discriminator](./README.md#in-discriminator) — the `in:` vocabulary,
// default-to-header behaviour, and the invalid-`in:` diagnostic path.
type fieldDocSignals struct {
	in        string
	inSet     bool
	invalidIn string // raw value when an `in:` line was present but its value isn't in the closed vocabulary; empty otherwise. The caller emits a diagnostic.
	ignored   bool
	file      bool
	strfmt    string
	strfmtSet bool
}

// scanFieldDocSignals reads every signal the response dispatcher needs out of a pre-parsed block
// slice and the raw doc text.
//
// Callers should pass `r.ParseBlocks(afld.Doc)` so the common.Builder cache absorbs the parse cost.
//
// Returns the zero value when doc is nil.
//
// # Details
//
// See [§in-discriminator](./README.md#in-discriminator) — why `in:` is line-scanned rather than
// read as a grammar Property.
func scanFieldDocSignals(blocks []grammar.Block, doc *ast.CommentGroup) fieldDocSignals {
	var pd fieldDocSignals
	if doc == nil {
		return pd
	}

	for _, b := range blocks {
		switch b.AnnotationKind() { //nolint:exhaustive // only ignore/file/strfmt are relevant here
		case grammar.AnnIgnore:
			pd.ignored = true
		case grammar.AnnFile:
			pd.file = true
		case grammar.AnnStrfmt:
			if arg, ok := b.AnnotationArg(); ok && !strings.ContainsAny(arg, " \t") {
				pd.strfmt = arg
				pd.strfmtSet = true
			}
		}
	}

	v, ok, invalid := common.ScanInLocation(doc.Text())
	switch {
	case ok:
		pd.in = v
		pd.inSet = true
	case invalid != "":
		pd.invalidIn = invalid
	}

	return pd
}

// strfmtFromDoc returns the argument of a `swagger:strfmt <name>` annotation present in blocks.
//
// Single-word filter mirrors the schema package's `findAnnotationArg` rule.
func strfmtFromDoc(blocks []grammar.Block) (string, bool) {
	for _, b := range blocks {
		if b.AnnotationKind() != grammar.AnnStrfmt {
			continue
		}
		if arg, ok := b.AnnotationArg(); ok && !strings.ContainsAny(arg, " \t") {
			return arg, true
		}
	}
	return "", false
}
