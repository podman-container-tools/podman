// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package validations owns cross-builder validation and coercion concerns shared by the schema,
// parameters, responses, and items/headers code paths.
//
// The package has two halves:
//
//   - Value coercion (this file) — turns raw annotation text into
//     the Go value implied by the target schema's type+format,
//     for keywords whose payload is a primitive literal
//     (default:, example:, enum:).
//   - Shape legality (shape.go) — answers whether a given keyword
//     is legal on a schema of a given Swagger type.
//
// # Details
//
// See [§contract](./README.md#contract) — why these helpers live in the builder layer rather
// than in the grammar parser.
package validations

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/go-openapi/spec"
)

// CoerceValue converts a raw annotation value to the Go representation implied by the target
// schema's Type/Format.
//
// Used by default:/example: setters where the annotation body is a primitive literal whose meaning
// depends on the target: `default: 3` becomes int(3) against `Type: "integer"`, "3" against `Type:
// "string"`, and so on.
//
// A nil schema yields the raw string unchanged.
// Numeric and boolean parsing errors are surfaced to the caller; JSON parse failures on
// object/array targets are absorbed and the raw string is returned.
//
// Dispatch is on [spec.SimpleSchema.TypeName] — Format wins when set, otherwise Type is
// consulted.
//
// # Details
//
// See [§coercion-dispatch](./README.md#coercion-dispatch) — the per-type dispatch table, and the
// rationale for absorbing object/array JSON parse failures.
func CoerceValue(s string, schema *spec.SimpleSchema) (any, error) {
	if schema == nil {
		return s, nil
	}

	switch strings.Trim(schema.TypeName(), "\"") {
	case "integer", "int", "int64", "int32", "int16":
		return strconv.Atoi(s)
	case "bool", "boolean":
		return strconv.ParseBool(s)
	case "number", "float64", "float32":
		return strconv.ParseFloat(s, 64)
	case "string":
		// Surrounding quotes are delimiters, not part of the value: `example: ""` is the empty string,
		// `example: "Foo"` is Foo (go-swagger#2547 / #2899, quirk F8).
		// An unquoted value (`Foo`) is returned unchanged.
		return unquoteIfQuoted(s), nil
	case "object":
		var obj map[string]any
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			return s, nil //nolint:nilerr // fallback: return raw string when JSON is invalid
		}
		return obj, nil
	case "array":
		var slice []any
		if err := json.Unmarshal([]byte(s), &slice); err != nil {
			return s, nil //nolint:nilerr // fallback: return raw string when JSON is invalid
		}
		return slice, nil
	default:
		return s, nil
	}
}

// CoerceJSONOrString coerces a default:/example: value whose target type is unknown — the case of
// a $ref override arm, which carries no Type of its own (the type lives on the referenced
// definition, not on the allOf sibling).
//
// A JSON object- or array-literal body is parsed structurally so it matches the direct-field path
// (quirk G3): `example: {"k":"v"}` yields an object on a $ref'd field just as it does on a plain
// map field.
//
// Any other value is treated as a string scalar, with surrounding double quotes stripped (mirroring
// [CoerceValue]'s "string" arm).
//
// Never errors: a malformed JSON literal falls back to the unquoted string.
// Scalar literals (numbers, booleans) are intentionally NOT coerced here — the referenced type is
// unknown, so a bare `example: 42` against a string-format $ref must not be silently turned into a
// number.
func CoerceJSONOrString(s string) any {
	if t := strings.TrimSpace(s); len(t) > 0 && (t[0] == '{' || t[0] == '[') {
		var v any
		if err := json.Unmarshal([]byte(t), &v); err == nil {
			return v
		}
	}
	return unquoteIfQuoted(s)
}

// unquoteIfQuoted strips a single pair of surrounding double quotes from a string annotation value,
// honouring Go/JSON escape sequences.
//
// A value that is not a double-quoted literal is returned unchanged, so a bare `example: Foo` stays
// Foo while `example: "Foo"` becomes Foo and `example: ""` becomes the empty string
// (go-swagger#2547 / #2899, F8).
func unquoteIfQuoted(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		if unq, err := strconv.Unquote(s); err == nil {
			return unq
		}
	}
	return s
}

// ParseDefault is the two-axis entry point for default:/example: coercion.
//
// It dispatches on schemaType alone — schemaFormat is accepted for surface stability but not
// consulted today.
//
// # Details
//
// See [§coercion-dispatch](./README.md#coercion-dispatch) and
// [§format-axis](./README.md#format-axis) — why the two-axis surface exists alongside
// [CoerceValue] and which refinements the schemaFormat argument is reserved for.
func ParseDefault(s, schemaType, schemaFormat string) (any, error) {
	_ = schemaFormat // reserved for format-specific paths
	return CoerceValue(s, &spec.SimpleSchema{Type: schemaType})
}

// ParseEnumValues is the two-axis entry point for enum: coercion.
//
// Mirrors [ParseDefault] — dispatches on schemaType, with schemaFormat reserved for future
// per-format paths.
func ParseEnumValues(val, schemaType, schemaFormat string) []any {
	_ = schemaFormat // reserved for format-specific paths
	return CoerceEnum(val, &spec.SimpleSchema{Type: schemaType})
}

// CoerceEnum turns an `enum: …` annotation value into a typed []any.
//
// Accepts the JSON-array form (`enum: ["a","b"]`) and the comma-list form (`enum: a, b`).
// Per-value typing is applied via [CoerceValue] against the target schema.
//
// # Details
//
// See [§enum-shapes](./README.md#enum-shapes) — how the two input forms are detected and how
// each element is normalised before per-value coercion.
func CoerceEnum(val string, s *spec.SimpleSchema) []any {
	var rawElements []json.RawMessage
	if err := json.Unmarshal([]byte(val), &rawElements); err != nil {
		return coerceEnumCommaList(val, s)
	}

	out := make([]any, len(rawElements))
	for i, d := range rawElements {
		ds, err := strconv.Unquote(string(d))
		if err != nil {
			ds = string(d)
		}

		v, err := CoerceValue(ds, s)
		if err != nil {
			out[i] = ds
			continue
		}
		out[i] = v
	}

	return out
}

// coerceEnumCommaList handles the comma-list `enum: a, b, c` form.
//
// Per-value whitespace is trimmed before coercion; per-value parse errors fall back to the raw
// string.
//
// The bracketed `enum: [a, b, c]` form also lands here when its values are unquoted (so
// [CoerceEnum]'s JSON-array parse fails — only the quoted `["a","b"]` / numeric `[1,2]` variants
// are valid JSON).
//
// The surrounding `[`/`]` are delimiters and must be stripped, else they glue onto the first/last
// value (go-swagger#2396).
func coerceEnumCommaList(val string, s *spec.SimpleSchema) []any {
	if v := strings.TrimSpace(val); len(v) >= 2 && v[0] == '[' && v[len(v)-1] == ']' {
		val = v[1 : len(v)-1]
	}
	list := strings.Split(val, ",")
	out := make([]any, len(list))

	for i, d := range list {
		d = strings.TrimSpace(d)
		v, err := CoerceValue(d, s)
		if err != nil {
			out[i] = d
			continue
		}
		out[i] = v
	}

	return out
}
