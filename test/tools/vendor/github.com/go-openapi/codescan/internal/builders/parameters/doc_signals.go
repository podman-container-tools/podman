// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parameters

import (
	"go/ast"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/common"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
)

// fieldDocSignals carries the per-field doc-comment signals the parameter dispatcher reads upstream of the schema
// build: the `in:` location, presence of `swagger:ignore`, presence of `swagger:file`, the `swagger:strfmt` argument,
// and the `swagger:type` override argument when set.
//
// Replaces the four v1 regex helpers (parsers.ParamLocation / parsers.FileParam / parsers.StrfmtName / parsers.Ignored)
// with grammar lookups plus a small `in:` line scan.
type fieldDocSignals struct {
	in          string
	inSet       bool
	ignored     bool
	file        bool
	strfmt      string
	strfmtSet   bool
	swaggerType string
	swTypeSet   bool
}

// scanFieldDocSignals reads every signal the parameter dispatcher needs out of a pre-parsed block slice and the raw doc
// text.
//
// Callers should pass `p.ParseBlocks(afld.Doc)` so the common.Builder cache absorbs the parse cost.
//
// Returns the zero value when doc is nil.
//
// # Details
//
// See [§in-discriminator](./README.md#in-discriminator) — why `in:` is line-scanned rather than read as a grammar
// Property.
func scanFieldDocSignals(blocks []grammar.Block, doc *ast.CommentGroup) fieldDocSignals {
	var pd fieldDocSignals
	if doc == nil {
		return pd
	}

	for _, b := range blocks {
		switch b.AnnotationKind() { //nolint:exhaustive // only ignore/file/strfmt/type are relevant here
		case grammar.AnnIgnore:
			pd.ignored = true
		case grammar.AnnFile:
			pd.file = true
		case grammar.AnnStrfmt:
			if arg, ok := b.AnnotationArg(); ok && !strings.ContainsAny(arg, " \t") {
				pd.strfmt = arg
				pd.strfmtSet = true
			}
		case grammar.AnnType:
			// A field-level swagger:type overrides the parameter type (go-swagger#1499).
			// Single-word filter mirrors strfmt and the schema builder's findAnnotationArg rule.
			if arg, ok := b.AnnotationArg(); ok && !strings.ContainsAny(arg, " \t") {
				pd.swaggerType = arg
				pd.swTypeSet = true
				// `swagger:type file` is a synonym for `swagger:file`, and the preferred spelling: `file` is an OAS v2 type name
				// like any other, so the annotation that names types should be able to name it.
				// Raising the same signal reuses the location gate that already governs swagger:file (formData only) rather than
				// adding a second one.
				if arg == fileTypeName {
					pd.file = true
				}
			}
		}
	}

	if v, ok, _ := common.ScanInLocation(doc.Text()); ok {
		pd.in = v
		pd.inSet = true
	}

	return pd
}
