// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validations

import (
	"fmt"
	"slices"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
)

// keywordTypeRules returns the set of Swagger schema types each keyword is legal on.
//
// Keywords absent from the table are legal on any type (or the rule is type-independent —
// `required`, `readOnly`, `deprecated`, `discriminator`).
//
// # Details
//
// See [§type-domain-table](./README.md#type-domain-table) — the source dialect, the keywords
// intentionally absent from the table, and the rationale for returning a fresh map per call.
func keywordTypeRules() map[string][]string {
	return map[string][]string{
		"pattern":           {"string"},
		"minLength":         {"string"},
		"maxLength":         {"string"},
		"maximum":           {"integer", "number"},
		"minimum":           {"integer", "number"},
		"multipleOf":        {"integer", "number"},
		"minItems":          {"array"},
		"maxItems":          {"array"},
		"uniqueItems":       {"array"},
		"minProperties":     {"object"},
		"maxProperties":     {"object"},
		"patternProperties": {"object"},
	}
}

// IsLegalForType reports whether keyword is legal on a schema with the given resolved Swagger type.
//
// Returns ok=true with empty hint when the keyword has no type constraint or the type matches.
// Returns ok=false with a human-readable hint when the type mismatches the keyword's domain.
//
// Empty schemaType is treated as "type unknown" and accepted; the caller decides whether to apply
// the keyword to a typeless schema.
// Format is not consulted — the domain rules apply at the type level.
//
// # Details
//
// See [§empty-type](./README.md#empty-type) — the best-effort-apply rule for unknown schema
// types, and why Format is intentionally kept off this axis.
func IsLegalForType(keyword grammar.Keyword, schemaType string) (ok bool, hint string) {
	rules, hasRule := keywordTypeRules()[keyword.Name]
	if !hasRule {
		return true, ""
	}
	if schemaType == "" {
		return true, ""
	}
	if slices.Contains(rules, schemaType) {
		return true, ""
	}
	return false, fmt.Sprintf(
		"keyword %q is only valid on schemas typed %v (got %q)",
		keyword.Name, rules, schemaType,
	)
}
