// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"go/ast"
	"go/constant"
	"go/token"
	"strconv"
	"strings"
)

// enumConstantValue converts a type-checked Go constant into its runtime value — int64 / uint64 /
// float64 / string / bool — for emission as an enum entry on the Swagger schema the scanner is
// building.
//
// This is the primary reading: go/types has already evaluated the constant exactly, so `iota`
// sequences, expressions (`1 << 3`, `Prev * 2`), references to other constants, rune literals
// (`'a'` → 97), raw/escaped strings and every integer base all arrive resolved. Reading them back
// out of the literal syntax instead would mean reimplementing Go's constant evaluator — see
// [§enum-values](./README.md#enum-values).
//
// Returns ok=false for a constant with no JSON representation (complex) or one the type-checker
// could not evaluate: the caller drops the member rather than emitting a null enum entry.
func enumConstantValue(value constant.Value) (any, bool) {
	switch value.Kind() {
	case constant.Int:
		if i, exact := constant.Int64Val(value); exact {
			return i, true
		}
		// A uint64 above math.MaxInt64 is a legal member of an unsigned enum.
		if u, exact := constant.Uint64Val(value); exact {
			return u, true
		}

		return nil, false

	case constant.Float:
		// exact is false whenever the value needs rounding to fit a float64 (1/3, big rationals);
		// the rounded value is still the best JSON can carry, so it is kept.
		f, _ := constant.Float64Val(value)

		return f, true

	case constant.String:
		return constant.StringVal(value), true

	case constant.Bool:
		return constant.BoolVal(value), true

	default: // constant.Complex (no JSON representation), constant.Unknown (evaluation failed)
		return nil, false
	}
}

// enumValue converts the RHS of a `const Foo Kind = "bar"` declaration into its runtime value by
// reading the literal syntax.
//
// This is the DEGRADED reading, used only when the type-checker has no value for the constant — a
// partially loaded package (see ErrDegradedLoad). It sees a strict subset of what
// [enumConstantValue] sees: a lone literal, optionally signed. `iota`, expressions and references
// to other constants are invisible to it by construction, since their value is not in the syntax.
//
// A signed numeric constant (`-1`, `+2.5`) is not a literal in the Go grammar: it reaches the AST
// as a unary expression wrapping the literal, so the sign is folded back into the literal text
// before parsing. Without this, negative enum members were silently dropped (go-swagger#3412).
//
// Returns nil when the expression is neither a literal nor a signed literal — the caller drops the
// value rather than emitting a null enum member.
func enumValue(expr ast.Expr) any {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return enumLiteralValue(e.Kind, e.Value)

	case *ast.UnaryExpr:
		if e.Op != token.SUB && e.Op != token.ADD {
			return nil
		}

		basicLit, ok := e.X.(*ast.BasicLit)
		if !ok || (basicLit.Kind != token.INT && basicLit.Kind != token.FLOAT) {
			return nil // a sign only ever applies to a number
		}

		return enumLiteralValue(basicLit.Kind, e.Op.String()+basicLit.Value)

	default:
		return nil
	}
}

// enumLiteralValue parses the textual form of a literal of the given kind.
//
// Integers are parsed with base detection so the non-decimal Go forms (`0x2a`, `0b101010`, `0o52`)
// and digit separators (`1_000`) resolve to their value like the decimal form does. A value above
// math.MaxInt64 — legal for a `uint64` enum — falls back to an unsigned parse rather than being
// dropped.
//
// A rune literal ('a', '\t') yields its code point, matching the integer type such a constant must
// have. Strings are unquoted rather than trimmed, so escape sequences resolve and the raw
// (backquoted) form loses its delimiters like the interpreted form does.
//
// Returns nil when the textual value fails to parse (rare — Go's own parser would have caught it
// upstream, but the safety net is cheap).
func enumLiteralValue(kind token.Token, value string) any {
	switch kind {
	case token.INT:
		if result, err := strconv.ParseInt(value, 0, 64); err == nil {
			return result
		}
		if result, err := strconv.ParseUint(value, 0, 64); err == nil {
			return result
		}

	case token.FLOAT:
		if result, err := strconv.ParseFloat(value, 64); err == nil {
			return result
		}

	case token.CHAR:
		// Drop the opening quote only: UnquoteChar resolves the escape (if any) and stops at the
		// closing one, so an escaped quote ('\'') survives where trimming both delimiters would eat
		// half of it.
		quoted, ok := strings.CutPrefix(value, "'")
		if !ok {
			return nil
		}
		if result, _, _, err := strconv.UnquoteChar(quoted, '\''); err == nil {
			return int64(result)
		}

	default:
		if result, err := strconv.Unquote(value); err == nil {
			return result
		}

		return strings.Trim(value, "\"")
	}

	return nil
}
