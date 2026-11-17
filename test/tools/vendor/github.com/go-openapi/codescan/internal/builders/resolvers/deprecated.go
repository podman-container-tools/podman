// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package resolvers

import oaispec "github.com/go-openapi/spec"

// ExtDeprecated is the vendor-extension key codescan uses to flag a deprecated model or field.
//
// OAS v2 has a native `deprecated` field only on operations; for schemas (models and their fields)
// the widely-used convention is the `x-deprecated: true` vendor extension.
// It is NOT an `x-go-*` reflection-metadata extension, so it is emitted regardless of
// Options.SkipExtensions — it carries author intent, not Go internals.
//
// Detection of the deprecation (explicit `deprecated:` keyword or a godoc "Deprecated:" paragraph)
// is done in the grammar; builders consume the single grammar.Block.IsDeprecated() signal.
// See go-swagger/go-swagger#3138.
const ExtDeprecated = "x-deprecated"

// MarkDeprecated sets `x-deprecated: true` on the schema.
//
// Idempotent.
func MarkDeprecated(ps *oaispec.Schema) {
	if ps == nil {
		return
	}
	ps.AddExtension(ExtDeprecated, true)
}
