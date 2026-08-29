// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package responses

import (
	"encoding/json"
	"go/ast"

	"github.com/go-openapi/codescan/internal/builders/handlers"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/parsers/yaml"
	oaispec "github.com/go-openapi/spec"
)

// headerItemsLevelTarget pairs a 1-indexed nesting depth (matching grammar.Property.ItemsDepth)
// with an *oaispec.Items target into which items-level validations at that depth must be written.
type headerItemsLevelTarget struct {
	level int
	items *oaispec.Items
}

// collectHeaderItemsLevels walks the AST array layers of a response header field and returns the
// (level, items) pairs reachable from the header's items chain.
//
// Mirrors items.ParseArrayTypes' shape on the grammar path; identical recursion shape to
// parameters' equivalent helper.
//
// Starting level is 1 — `items.maximum:` has ItemsDepth=1 in the grammar lexer.
// Named/aliased array types opt out (parity with v1's tagger pipeline).
func collectHeaderItemsLevels(expr ast.Expr, it *oaispec.Items, level int) []headerItemsLevelTarget {
	if it == nil {
		return nil
	}

	here := headerItemsLevelTarget{level: level, items: it}

	switch e := expr.(type) {
	case *ast.ArrayType:
		rest := collectHeaderItemsLevels(e.Elt, it.Items, level+1)
		out := make([]headerItemsLevelTarget, 0, 1+len(rest))
		return append(append(out, here), rest...)

	case *ast.Ident:
		rest := collectHeaderItemsLevels(expr, it.Items, level+1)
		if e.Obj == nil {
			out := make([]headerItemsLevelTarget, 0, 1+len(rest))
			return append(append(out, here), rest...)
		}
		return rest

	case *ast.StarExpr:
		return collectHeaderItemsLevels(e.X, it, level)

	case *ast.SelectorExpr:
		return []headerItemsLevelTarget{here}

	case *ast.StructType, *ast.InterfaceType, *ast.MapType:
		return nil

	default:
		return nil
	}
}

// applyBlockToDecl parses the top-level response doc through grammar and writes the description to
// resp.Description via the grammar parser's prose accumulator, plus the response-level `examples:`
// block (the only keyword the response decl level accepts).
func (r *Builder) applyBlockToDecl(resp *oaispec.Response) {
	block := r.ParseBlock(r.Decl.Comments)
	resp.Description = r.overriddenDescription(r.CleanGoDoc(block.Prose()), r.Decl.Comments)
	r.applyResponseExamples(block, resp)
}

// overriddenDescription returns the swagger:description override for cg when present (empty
// included — godoc suppression, D7), else the godoc-derived fallback.
//
// A swagger:title on a response / header target is rejected with a context-invalid diagnostic:
// OpenAPI 2.0 Response and Header objects have no `title` field.
func (r *Builder) overriddenDescription(fallback string, cg *ast.CommentGroup) string {
	titleOv, descOv := r.HarvestOverrides(cg)
	if titleOv.Present {
		r.RecordDiagnostic(grammar.Warnf(titleOv.Pos, grammar.CodeContextInvalid,
			"swagger:title is not applicable here: OpenAPI 2.0 responses and headers have no title; ignored"))
	}
	r.WarnEmptyOverride(grammar.AnnDescription, descOv)
	if descOv.Present {
		return descOv.Value
	}
	return fallback
}

// applyResponseExamples parses a response-level `examples:` block — a YAML map keyed by mime type
// (`examples:` then `application/json: {…}`) — onto resp.Examples (the OAS2 Response.examples
// field).
//
// This is the plural, response-scoped keyword (go-swagger#2871); the singular schema `example:` is
// handled per-field / per-body elsewhere.
// The `swagger:operation` YAML path already carries examples for free via the spec unmarshal —
// this covers only the struct-based `swagger:response`.
func (r *Builder) applyResponseExamples(block grammar.Block, resp *oaispec.Response) {
	var prop grammar.Property
	var found bool
	for p := range block.Properties() {
		if p.Keyword.Name == grammar.KwExamples {
			prop, found = p, true
			break
		}
	}
	if !found || prop.Body == "" {
		return
	}

	examples := make(map[string]any)
	if err := yaml.UnmarshalBody(prop.Body, func(b []byte) error {
		return json.Unmarshal(b, &examples)
	}); err != nil {
		r.RecordDiagnostic(grammar.Warnf(
			prop.Pos, grammar.CodeInvalidAnnotation,
			"could not parse response examples block as a mime-keyed YAML map: %v", err,
		))
		return
	}

	if len(examples) > 0 {
		resp.Examples = examples
	}
}

// applyBlockToHeader parses afld.Doc through grammar and dispatches description, header
// validations, items-level validations, and user-authored vendor extensions into ps.
//
// # Details
//
// See [§dispatch](./README.md#dispatch) — the three-phase Walker dispatch for headers, the
// omitted `required:` write, and how items-level validations chain.
func (r *Builder) applyBlockToHeader(afld *ast.Field, header *oaispec.Header) {
	block := r.ParseBlock(afld.Doc)

	header.Description = r.overriddenDescription(r.CleanGoDoc(block.Prose()), afld.Doc)
	handlers.DispatchHeaderLevel0(block, header, r.RecordDiagnostic)

	if arrayType, ok := afld.Type.(*ast.ArrayType); ok {
		for _, tgt := range collectHeaderItemsLevels(arrayType.Elt, header.Items, 1) {
			handlers.DispatchItemsLevel(block, tgt.items, tgt.level, r.RecordDiagnostic)
		}
	}
}
