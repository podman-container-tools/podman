// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

// Walker is the functional-visitor surface a Block exposes for bulk dispatch.
//
// Consumers wire only the callbacks they care about — every nil field is a silent no-op.
//
// # Details
//
// See README §walker-contract for the dispatch order, the per- Keyword.Shape callback table, the
// Number/Integer/Bool typing- failure rule, the FilterDepth gating, and the concurrency contract.
type Walker struct {
	Title       func(s string)
	Description func(s string)

	Number  func(p Property, val float64, exclusive bool)
	Integer func(p Property, val int64)
	Bool    func(p Property, val bool)
	String  func(p Property, val string)
	Raw     func(p Property)
	Unknown func(p Property)

	Extension  func(ext Extension)
	Diagnostic func(d Diagnostic)

	FilterDepth int
}

// AllDepths is the FilterDepth sentinel meaning "fire property callbacks regardless of ItemsDepth".
//
// Use it explicitly rather than -1 so the intent reads at the call site.
const AllDepths = -1

// Walk dispatches one Block through w. Nil callbacks are silently ignored.
//
// See Walker for the dispatch contract.
//
// Walk reads only from b — it never mutates the Block or its properties.
// Walk is safe to call concurrently on the same Block from multiple goroutines as long as the
// Walker callbacks are themselves safe.
func (b *baseBlock) Walk(w Walker) {
	// Block-level diagnostics fire first so consumers see them regardless of whether they wired any
	// property callbacks.
	if w.Diagnostic != nil {
		for _, d := range b.diagnostics {
			w.Diagnostic(d)
		}
	}

	if w.Title != nil && b.title != "" {
		w.Title(b.title)
	}
	if w.Description != nil && b.description != "" {
		w.Description(b.description)
	}

	for _, p := range b.properties {
		if !walkerAcceptsDepth(w.FilterDepth, p.ItemsDepth) {
			continue
		}
		walkerDispatchProperty(w, p)
	}

	if w.Extension != nil {
		for _, e := range b.extensions {
			w.Extension(e)
		}
	}
}

// walkerAcceptsDepth reports whether a property at itemsDepth should be dispatched given the
// walker's FilterDepth.
//
// AllDepths admits everything.
func walkerAcceptsDepth(filter, itemsDepth int) bool {
	if filter == AllDepths {
		return true
	}
	return filter == itemsDepth
}

// walkerDispatchProperty routes one property to the matching callback.
//
// Dispatch is by Keyword.Shape (the table-declared shape) rather than Typed.Type, so failed-typing
// properties (Typed.Type == ShapeNone with a CodeInvalidNumber diagnostic in tow) still reach their
// shape-typed callback — consumers see the zero payload alongside the diagnostic.
//
// Unknown keywords (empty Keyword.Name, no entry in the keyword table) take the Unknown path
// regardless of Shape.
// See README §walker-contract.
func walkerDispatchProperty(w Walker, p Property) {
	if p.Keyword.Name == "" {
		if w.Unknown != nil {
			w.Unknown(p)
		}
		return
	}

	switch p.Keyword.Shape {
	case ShapeNumber:
		if w.Number != nil {
			w.Number(p, p.Typed.Number, isExclusiveOp(p.Typed.Op))
		}
	case ShapeInt:
		if w.Integer != nil {
			w.Integer(p, p.Typed.Integer)
		}
	case ShapeBool:
		if w.Bool != nil {
			w.Bool(p, p.Typed.Boolean)
		}
	case ShapeString:
		// String-shaped keywords keep the raw value in p.Value; Typed.String is only set for
		// ShapeEnumOption.
		if w.String != nil {
			w.String(p, p.Value)
		}
	case ShapeEnumOption:
		if w.String != nil {
			w.String(p, p.Typed.String)
		}
	case ShapeNone, ShapeCommaList, ShapeRawBlock, ShapeRawValue:
		if w.Raw != nil {
			w.Raw(p)
		}
	default:
		if w.Raw != nil {
			w.Raw(p)
		}
	}
}

// isExclusiveOp reports whether the leading operator on a Number value signals exclusive-bound
// semantics.
//
// The grammar accepts `maximum: <5` (exclusive max) and `maximum: <=5` (inclusive max); the lexer
// keeps the operator on Typed.Op and Walker collapses it to a bool here.
func isExclusiveOp(op string) bool {
	return op == "<" || op == ">"
}
