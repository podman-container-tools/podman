// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import "go/token"

// NewSyntheticBlock builds a Block from a manually-curated set of properties.
//
// Used by sub-parsers (routebody, future input modes) that lower a non-grammar text surface into
// the standard Block shape so consumers can dispatch through the usual Walker.
//
// title and description become the Block's Title()/Description(), also surfaced via Prose() with
// internal blank separation. pos is the source position of the synthetic block's head —
// Properties that lack their own Pos inherit it implicitly when consumers build diagnostics.
//
// The returned Block exposes empty Diagnostics(), AnnotationKind() == AnnUnknown, no YAML blocks,
// no extensions, and no security requirements.
// AnnotationArg() returns ("", false).
//
// Walk fires Title/Description first when non-empty, then properties in slice order — the regular
// Walker contract.
// See README §synthetic-block.
func NewSyntheticBlock(pos token.Position, title, description string, props []Property) Block {
	return &baseBlock{
		pos:                 pos,
		title:               title,
		description:         description,
		preambleTitle:       title,
		preambleDescription: description,
		properties:          props,
	}
}
