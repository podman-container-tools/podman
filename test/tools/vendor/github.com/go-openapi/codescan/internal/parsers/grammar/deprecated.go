// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"regexp"
	"strings"
)

// deprecatedRx matches a godoc-style "Deprecated:" mark at the start of a paragraph — the pkgsite
// convention.
//
// Reused verbatim from golang.org/x/pkgsite internal/godoc/dochtml/deprecated.go so a single
// `Deprecated:` doc paragraph drives both godoc and the spec, with no need to repeat the mark as an
// explicit `deprecated: true` annotation.
//
// The lexer leaves such a paragraph as prose (a `Deprecated:` line whose argument is not a bool is
// not lexed as the `deprecated` keyword — see lexKeyword), so the reason text survives in the
// description while IsDeprecated reports the deprecation.
var deprecatedRx = regexp.MustCompile(`(^|\n\s*\n)\s*Deprecated:`)

// IsDeprecated reports whether the block marks its subject deprecated, via either trigger:
//
//   - the explicit `deprecated: true` keyword (this also drives the
//     native OAS2 operation `deprecated` field for route/operation
//     blocks); an explicit `deprecated: false` wins and returns false.
//   - a godoc-style "Deprecated:" paragraph in the prose.
//
// Schema builders emit `x-deprecated: true` (see resolvers.MarkDeprecated) for a deprecated model
// or field.
// See go-swagger/go-swagger#3138.
func (b *baseBlock) IsDeprecated() bool {
	if v, ok := b.GetBool(KwDeprecated); ok {
		return v
	}
	return deprecatedRx.MatchString(strings.Join(b.proseLines, "\n"))
}
