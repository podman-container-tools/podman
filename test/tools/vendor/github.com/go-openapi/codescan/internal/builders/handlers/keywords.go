// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package handlers

import "github.com/go-openapi/codescan/internal/parsers/grammar"

// simpleSchemaAllowed enumerates the grammar keyword names that
// can legally appear on an OAS v2 SimpleSchema site (parameter with
// `in != body`, response header, and the items chain within either).
//
// Source of truth: the OAS v2 Parameter Object and Header Object allowed-keyword tables.
//
// Vendor extensions (`x-*`) are NOT listed here — they are gated by
// classify.IsAllowedExtension, which runs by name-prefix.
//
// See [§simple-schema-keywords](./README.md#simple-schema-keywords)
// for the `required:` carve-out (valid on parameters, skipped on
// headers, silently dropped under SimpleSchema mode on the schema
// walker).
//
//nolint:gochecknoglobals // closed-vocabulary lookup table; one allocation, read-only.
var simpleSchemaAllowed = map[string]struct{}{
	grammar.KwMaximum:          {},
	grammar.KwMinimum:          {},
	grammar.KwMultipleOf:       {},
	grammar.KwMinLength:        {},
	grammar.KwMaxLength:        {},
	grammar.KwPattern:          {},
	grammar.KwMinItems:         {},
	grammar.KwMaxItems:         {},
	grammar.KwUnique:           {},
	grammar.KwCollectionFormat: {},
	grammar.KwDefault:          {},
	grammar.KwExample:          {},
	grammar.KwEnum:             {},
	grammar.KwRequired:         {},
}

// IsSimpleSchemaKeyword reports whether keyword is legal on an OAS v2 SimpleSchema site.
//
// Returns false for full-Schema-only keywords (`readOnly`, `discriminator`, `$ref`, `allOf`, ...)
// and for unknown names.
//
// Consumers wired in SimpleSchema mode (the schema builder under WithSimpleSchema, the parameters
// dispatcher, the responses dispatcher) use this predicate to gate writes and emit
// CodeUnsupportedInSimpleSchema diagnostics on miss.
func IsSimpleSchemaKeyword(keyword string) bool {
	_, ok := simpleSchemaAllowed[keyword]
	return ok
}
