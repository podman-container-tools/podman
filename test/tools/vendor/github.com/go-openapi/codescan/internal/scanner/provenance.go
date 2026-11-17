// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"go/token"
	"strings"
)

// Provenance ties a node in the produced Swagger spec (by RFC 6901 JSON pointer) to the source
// position of the Go construct that produced it.
//
// It is the source-side half of the cross-ref linker (see the genspec-tui linkage design).
// Provenance is emitted via [Options.OnProvenance] only at "anchor" nodes — those born from a
// code detail (a type declaration, a struct field, a const/var value, a route/meta annotation
// block).
//
// Finer nodes carry no Provenance of their own; a consumer resolves them to their nearest anchored
// ancestor.
//
// Experimental: this surface may change while LSP / TUI integration matures.
type Provenance struct {
	// Pointer is the RFC 6901 JSON pointer of the anchored spec node, e.g. "/definitions/User" or
	// "/paths/~1pets/get".
	Pointer string
	// Pos is the source location (file:line:col) of the producing construct.
	Pos token.Position
}

// JSONPointer builds an RFC 6901 pointer from raw (unescaped) segments, escaping each per the spec
// (~ → ~0, / → ~1).
//
// The output matches what the spec-side index derives via jsontext, so source- and spec-side
// pointers for the same node are byte-identical and join cleanly.
func JSONPointer(segments ...string) string {
	var b strings.Builder
	for _, seg := range segments {
		b.WriteByte('/')
		b.WriteString(escapePointerToken(seg))
	}
	return b.String()
}

func escapePointerToken(seg string) string {
	seg = strings.ReplaceAll(seg, "~", "~0")
	seg = strings.ReplaceAll(seg, "/", "~1")
	return seg
}
