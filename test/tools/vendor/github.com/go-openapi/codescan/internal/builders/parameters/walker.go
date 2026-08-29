// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parameters

import (
	"go/ast"

	"github.com/go-openapi/codescan/internal/builders/handlers"
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	oaispec "github.com/go-openapi/spec"
)

// paramItemsLevelTarget pairs a 1-indexed nesting depth (matching grammar.Property.ItemsDepth) with
// the *oaispec.Items target into which validations at that depth must be written.
type paramItemsLevelTarget struct {
	level int
	items *oaispec.Items
}

// collectParamItemsLevels walks the AST array layers of a parameter field type and returns the
// (level, items) pairs reachable from the param's items chain.
//
// Mirrors items.ParseArrayTypes' shape on the grammar path; the schema builder handles the
// Schema-side equivalent internally.
//
// Starting level is 1 — `items.maximum:` has ItemsDepth=1 in the grammar lexer.
// Named/aliased array types opt out (parity with v1's tagger pipeline).
func collectParamItemsLevels(expr ast.Expr, it *oaispec.Items, level int) []paramItemsLevelTarget {
	if it == nil {
		return nil
	}

	here := paramItemsLevelTarget{level: level, items: it}

	switch e := expr.(type) {
	case *ast.ArrayType:
		rest := collectParamItemsLevels(e.Elt, it.Items, level+1)
		out := make([]paramItemsLevelTarget, 0, 1+len(rest))
		return append(append(out, here), rest...)

	case *ast.Ident:
		rest := collectParamItemsLevels(expr, it.Items, level+1)
		if e.Obj == nil {
			out := make([]paramItemsLevelTarget, 0, 1+len(rest))
			return append(append(out, here), rest...)
		}
		return rest

	case *ast.StarExpr:
		return collectParamItemsLevels(e.X, it, level)

	case *ast.SelectorExpr:
		return []paramItemsLevelTarget{here}

	case *ast.StructType, *ast.InterfaceType, *ast.MapType:
		return nil

	default:
		return nil
	}
}

// applyBlockToField parses afld.Doc through grammar and dispatches description, level-0
// validations, required flag, vendor extensions, and items-level validations into param.
//
// # Details
//
// See [§dispatch](./README.md#dispatch) — the three-phase Walker dispatch, why `in:` is resolved
// upstream, and the items-level chain.
func (p *Builder) applyBlockToField(afld *ast.Field, param *oaispec.Parameter) error {
	block := p.ParseBlock(afld.Doc)

	param.Description = p.CleanGoDoc(block.Prose())
	param.Description = resolvers.AppendEnumDesc(param.Description, param.Extensions, p.Ctx.SkipEnumDescriptions())

	if err := handlers.DispatchParamLevel0(block, param, p.RecordDiagnostic); err != nil {
		return err
	}

	if arrayType, ok := afld.Type.(*ast.ArrayType); ok {
		for _, tgt := range collectParamItemsLevels(arrayType.Elt, param.Items, 1) {
			handlers.DispatchItemsLevel(block, tgt.items, tgt.level, p.RecordDiagnostic)
		}
	}
	return nil
}
