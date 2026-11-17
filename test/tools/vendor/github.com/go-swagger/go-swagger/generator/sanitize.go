// SPDX-FileCopyrightText: Copyright 2015-2026 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"go/token"
	"log"
	"strings"
)

// Failsafe codegen & Security helpers for spec-supplied identifier overrides.
//
// Extensions such as x-go-name and x-go-type let a spec author choose the exact
// Go identifier emitted into generated source.
//
// Those values are written verbatim into identifier or type-reference positions,
// so a value that is not a plain Go identifier (e.g. one carrying newlines, braces or backticks)
// can break out of its syntactic slot and inject arbitrary top-level declarations
// into the generated code.
//
// Whether accidental or from a malicious origin, we want the codegen to detect this situation early on.
//
// We validate-and-reject rather than silently mangle: a legitimate override is a valid Go identifier
// and passes unchanged, while a hostile value fails generation with a clear error instead of being quietly rewritten.

// isGoIdentifier reports whether s is a single, valid, non-blank Go identifier.
//
// go/token.IsIdentifier already rejects keywords, the empty string and any
// string containing non-identifier runes (whitespace, punctuation, …). We also
// reject the blank identifier "_", which is never a meaningful override.
func isGoIdentifier(s string) bool {
	return s != "_" && token.IsIdentifier(s)
}

// isGoQualifiedType reports whether s is a valid Go type-name reference: a
// dotted sequence of identifiers, e.g. "MyType" or "pkg.MyType".
//
// It deliberately rejects anything carrying whitespace, brackets, operators or
// statement separators — exactly what an injection payload needs to escape the
// type-reference position.
func isGoQualifiedType(s string) bool {
	if s == "" {
		return false
	}

	for part := range strings.SplitSeq(s, ".") {
		if !isGoIdentifier(part) {
			return false
		}
	}

	return true
}

// validateGoIdentifierExtension validates an x-go-name value that is emitted as a
// Go identifier. It returns a descriptive error when the value is not a plain Go
// identifier.
func validateGoIdentifierExtension(value string) error {
	if isGoIdentifier(value) {
		return nil
	}

	return fmt.Errorf("%s value %q is not a valid Go identifier: a spec-supplied Go name is emitted verbatim into generated source and must match the Go identifier syntax", xGoName, value)
}

// validateGoTypeExtension validates an x-go-type "type" value that is emitted as a
// Go type reference. It accepts a (possibly qualified) identifier and returns a
// descriptive error otherwise.
func validateGoTypeExtension(value string) error {
	if isGoQualifiedType(value) {
		return nil
	}

	return fmt.Errorf("%s value %q is not a valid Go type name: a spec-supplied type is emitted verbatim into generated source and must be a (possibly qualified) Go identifier", xGoType, value)
}

// sanitizeGoNameOverride returns an x-go-name override when it is a valid Go
// identifier (or empty), and "" with a warning otherwise.
//
// It is used at the call sites that consume x-go-name raw and cannot return an
// error: returning "" makes the caller fall back to its default, mangled name,
// so a hostile value can neither break out of an identifier position nor leak
// into a comment.
func sanitizeGoNameOverride(value string) string {
	if value == "" || isGoIdentifier(value) {
		return value
	}

	log.Printf("warning: %s value %q is not a valid Go identifier. Skipped for security reasons", xGoName, value)

	return ""
}
