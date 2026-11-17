// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import "regexp"

// The path annotations — `swagger:route` and `swagger:operation` — are the only regexes left in the
// scanner. They match four things in sequence (method, path, optional tags, operationId) and the
// path's alphabet is the bulk of the pattern; the parse reads the result back by submatch index.
// The other classifiers, which only ever asked where a keyword sat on a line, are string searches
// in annotation_line.go.

const (
	// rxCommentPrefix matches the leading comment noise that precedes an annotation keyword on a raw comment line:
	// whitespace, tabs, slashes, asterisks, dashes, optional markdown table pipe, then trailing spaces.
	//
	// Annotations must START the comment line — any prose before the `swagger:xxx` keyword disqualifies the line, so an
	// annotation buried in prose is ignored.
	//
	// The sole documented exception is `swagger:route`, which is allowed to follow a single godoc identifier (see
	// rxRoutePrefix).
	rxCommentPrefix = `^[\p{Zs}\t/\*-]*\|?\p{Zs}*`

	// rxRoutePrefix extends rxCommentPrefix with an OPTIONAL single leading identifier.
	//
	// Godoc convention places the function/type name before the annotation body, e.g. `// DoBad swagger:route GET /path`.
	// The allowance is intentionally narrow — ONE identifier, then whitespace — so multi-word prose prefixes still
	// fail.
	//
	// This exception is reserved for `swagger:route`.
	// All other annotations must start the comment line, per rxCommentPrefix.
	rxRoutePrefix = rxCommentPrefix + `(?:\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]*\p{Zs}+)?`

	rxMethod = "(\\p{L}+)"
	rxPath   = "((?:/[\\p{L}\\p{N}\\p{Pd}\\p{Pc}{}\\-\\.\\?_~%!$&'()*+,;=:@/]*)+/?)"

	// rxOpTags and rxOpID both accept a name of a SINGLE character: a letter, then zero or more further characters.
	//
	// They required one further character until 2026-08-02, which silently voided the whole annotation.
	// The failure is not local to the offending name, because the tags group is optional: on `swagger:route GET /pets e
	// listPets` the parse does not stop at `e`, it falls back to matching with NO tags, which leaves rxOpID to swallow `e
	// listPets` — and its alphabet has no space.
	//
	// The line then matches nothing, and a `swagger:route` matching nothing is not a malformed route, it is not a route at
	// all, so there was nothing left to raise a diagnostic about.
	//
	// OAS 2.0 puts no such floor on either: a tag and an operationId are free-form strings.
	rxOpTags = "(\\p{L}[\\p{L}\\p{N}\\p{Pd}\\.\\p{Pc}\\p{Zs}]*)"
	rxOpID   = "(\\p{L}[\\p{L}\\p{N}\\p{Pd}\\p{Pc}]*)"
)

// compile-once regexes; read-only.
var (
	// rxRouteHead / rxOperationHead match the HEAD of a path annotation — its full regex up to and including the path,
	// with the tags and operationId left off.
	//
	// They exist to tell "this line is not an annotation" apart from "this line meant to be one and did not parse".
	// The full regexes cannot make that distinction: a line that fails them is indistinguishable from prose, which is why
	// a malformed route used to disappear in silence.
	//
	// Matching the keyword alone is NOT enough to tell those apart.
	// Annotations must start the comment line, so a doc comment whose sentence happens to begin `swagger:route response
	// lines are …` also starts with the keyword — three such lines exist in this repo's own fixtures.
	//
	// Requiring a method and a `/`-rooted path costs nothing (a real annotation always has both) and drops every one of
	// them, since prose after the keyword does not reach a path.
	rxRouteHead     = regexp.MustCompile(rxRoutePrefix + `swagger:route\p{Zs}+` + rxMethod + `\p{Zs}*` + rxPath)
	rxOperationHead = regexp.MustCompile(rxCommentPrefix + `swagger:operation\p{Zs}+` + rxMethod + `\p{Zs}*` + rxPath)

	rxRoute = regexp.MustCompile(
		rxRoutePrefix +
			"swagger:route\\p{Zs}*" +
			rxMethod +
			"\\p{Zs}*" +
			rxPath +
			"(?:\\p{Zs}+" +
			rxOpTags +
			")?\\p{Zs}+" +
			rxOpID + "\\p{Zs}*$")
	rxOperation = regexp.MustCompile(
		rxCommentPrefix +
			"swagger:operation\\p{Zs}*" +
			rxMethod +
			"\\p{Zs}*" +
			rxPath +
			"(?:\\p{Zs}+" +
			rxOpTags +
			")?\\p{Zs}+" +
			rxOpID + "\\p{Zs}*$")
)
