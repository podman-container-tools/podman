// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validations

import "math"

// int64 bounds expressed in float64 so a float constant can be range-checked before conversion: converting an
// out-of-range float64 to int64 is implementation-defined in Go, so the check must happen first.
//
// The upper bound is exclusive — 2^63 is exactly representable as a float64 but is one past math.MaxInt64.
const (
	minInt64AsFloat = -9223372036854775808.0
	maxInt64AsFloat = 9223372036854775808.0
)

// CoerceConstant normalises a Go constant value — as extracted from an enum's const block — to the representation
// implied by the enum's declared Swagger type.
//
// It is the invariant guard on what reaches an `enum` member: whatever the scanner read, a value that contradicts the
// schema's own `type` is never emitted.
//
// The scanner's primary reading takes its values from the type-checker, which already normalises a constant's kind to
// its declared type (`TiltFlat Tilt = 0` on a `float64` enum is a Float constant, `Answer Count = 42.0` on an integer
// one is an Int), so on that path this is a no-op.
//
// It earns its keep on the degraded reading, which falls back to literal syntax — where `= 0` is an int64 whatever
// type it was declared with.
// See [§enum-const-values](./README.md#enum-const-values).
//
// schemaType is the Swagger type ("integer", "number", "string", "boolean"); any other value — including the empty
// string for a type that did not resolve — passes the value through untouched.
//
// Reports (nil, false) when the value is not representable in the target type.
// Code that compiles cannot produce that (Go rejects `const x IntType = 0.5`), so a caller should treat it as a defect
// worth a diagnostic rather than a routine outcome.
func CoerceConstant(value any, schemaType string) (any, bool) {
	switch schemaType {
	case "integer":
		switch v := value.(type) {
		case int64, uint64:
			return v, true
		case float64:
			// An integral float literal is legal Go for an integer constant (`= 42.0`).
			if v != math.Trunc(v) || v < minInt64AsFloat || v >= maxInt64AsFloat {
				return nil, false
			}
			return int64(v), true
		default:
			return nil, false
		}

	case "number":
		switch v := value.(type) {
		case float64:
			return v, true
		case int64:
			return float64(v), true
		case uint64:
			return float64(v), true
		default:
			return nil, false
		}

	case "string":
		if v, ok := value.(string); ok {
			return v, true
		}
		return nil, false

	case "boolean":
		if v, ok := value.(bool); ok {
			return v, true
		}
		return nil, false

	default:
		// No resolved type to normalise against (or a type with no numeric domain): leave the value exactly as the scanner
		// produced it rather than guessing.
		return value, true
	}
}
