// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import "strings"

// NormalizeIn returns the canonical OAS v2 parameter-location value matching raw,
// case-insensitively, against the closed vocabulary declared on KwIn (`query` / `path` / `header` /
// `body` / `formData`).
//
// Returns ("", false) when raw is not recognised.
//
// When allowFormAlias is true, the routes-inline-param affordance from v1 is enabled: a raw value
// of `form` (case-insensitive) is accepted and normalised to `formData`.
//
// This is documented in observed-quirks Q27 and is intentionally contained to
// internal/parsers/routebody — every other capture site MUST pass allowFormAlias=false so the
// canonical OAS v2 vocabulary is the single source of truth.
//
// The normalisation here mirrors what the grammar's enum-option parser does for typed KwIn
// properties (parser.go:740, strings.EqualFold).
// It exists as a public helper because three capture sites read `in:` by scanning doc text directly
// rather than going through grammar's typed property path:
//
//   - internal/builders/parameters/doc_signals.go (scanInLocation)
//   - internal/builders/responses/doc_signals.go (scanInLocation)
//   - internal/parsers/routebody/parameters.go (applyParamLine)
//
// All three route through this helper so case-insensitivity is enforced uniformly and the closed
// vocabulary lives in one place.
//
// Q29 (2026-06-03) — go-swagger-generated code emits capitalised forms like `in: Body`; the
// pre-fix strict-case map lookup silently miscategorised them, dropping fields to the `query`
// default.
func NormalizeIn(raw string, allowFormAlias bool) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false
	}
	for _, allowed := range inLocations {
		if strings.EqualFold(raw, allowed) {
			return allowed, true
		}
	}
	if allowFormAlias && strings.EqualFold(raw, "form") {
		return "formData", true
	}
	return "", false
}

// inLocations is the canonical OAS v2 parameter-location vocabulary.
// Mirrors KwIn's asEnumOption values; kept as a package-private
// slice so NormalizeIn iterates a fixed order (deterministic for
// future diagnostic emission) instead of map iteration.
//
//nolint:gochecknoglobals // immutable canonical vocabulary, read-only.
var inLocations = []string{"query", "path", "header", "body", "formData"}
