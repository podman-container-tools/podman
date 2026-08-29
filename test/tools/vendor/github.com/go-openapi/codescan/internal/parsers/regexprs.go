// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import "regexp"

const (
	// rxCommentPrefix matches the leading comment noise that precedes an annotation keyword on a raw
	// comment line: whitespace, tabs, slashes, asterisks, dashes, optional markdown table pipe, then
	// trailing spaces.
	//
	// Annotations must START the comment line — any prose before the `swagger:xxx` keyword
	// disqualifies the line, so an annotation buried in prose is ignored.
	//
	// The sole documented exception is `swagger:route`, which is allowed to follow a single godoc
	// identifier (see rxRoutePrefix).
	rxCommentPrefix = `^[\p{Zs}\t/\*-]*\|?\p{Zs}*`

	// rxRoutePrefix extends rxCommentPrefix with an OPTIONAL single leading identifier.
	//
	// Godoc convention places the function/type name before the annotation body, e.g. `// DoBad
	// swagger:route GET /path`.
	// The allowance is intentionally narrow — ONE identifier, then whitespace — so multi-word
	// prose prefixes still fail.
	//
	// This exception is reserved for `swagger:route`.
	// All other annotations must start the comment line, per rxCommentPrefix.
	rxRoutePrefix = rxCommentPrefix + `(?:\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]*\p{Zs}+)?`

	rxMethod = "(\\p{L}+)"
	rxPath   = "((?:/[\\p{L}\\p{N}\\p{Pd}\\p{Pc}{}\\-\\.\\?_~%!$&'()*+,;=:@/]*)+/?)"
	rxOpTags = "(\\p{L}[\\p{L}\\p{N}\\p{Pd}\\.\\p{Pc}\\p{Zs}]+)"
	rxOpID   = "((?:\\p{L}[\\p{L}\\p{N}\\p{Pd}\\p{Pc}]+)+)"
)

// compile-once regexes; read-only.
var (
	// rxSwaggerAnnotation matches `swagger:<name>` anywhere on a comment line where it is preceded by
	// whitespace, `/`, or start-of-line.
	//
	// Kept loose because it is the classification regex consumed by scanner.index.ExtractAnnotation;
	// `swagger:route` is allowed to follow a godoc-style identifier per rxRoutePrefix.
	//
	// Do NOT use this regex as a block terminator — it triggers on mid-prose mentions and would
	// truncate descriptions.
	rxSwaggerAnnotation = regexp.MustCompile(`(?:^|[\s/])swagger:([\p{L}\p{N}\p{Pd}\p{Pc}]+)`)

	rxModelOverride = regexp.MustCompile(rxCommentPrefix + `swagger:model\p{Zs}*(\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]+)?(?:\.)?$`)
	// rxResponseOverride is the scanner's classification gate for `swagger:response`.
	//
	// The argument is optional (bare marker → name inferred from the type), an identifier name, or
	// the shared-namespace wildcard `*` (a synonym for the bare form — `swagger:response *` and
	// `swagger:response` both register a shared response keyed by the type name).
	//
	// A malformed name (e.g. a package-qualified `utils.Error`) still fails the match, so
	// MalformedResponseName can flag it.
	// The response NAME itself is resolved from the grammar (grammar.ResponseBlock), not from this
	// capture.
	rxResponseOverride = regexp.MustCompile(rxCommentPrefix + `swagger:response\p{Zs}*(\*|\p{L}[\p{L}\p{N}\p{Pd}\p{Pc}]+)?(?:\.)?$`)
	// rxParametersOverride is the scanner's PERMISSIVE classification gate for `swagger:parameters`:
	// it matches the keyword followed by any non-empty argument and captures it.
	//
	// The capture is required by the shared comment matcher (commentMultipleSubMatcher), but its
	// CONTENT is unused — the argument tokens are parsed and validated by the grammar
	// (grammar.ParametersBlock), which emits diagnostics for malformed forms.
	//
	// The scanner does not re-validate arg shapes: a malformed-but-non-empty argument is classified
	// and passed on so the grammar can diagnose it, rather than being silently skipped here.
	rxParametersOverride = regexp.MustCompile(rxCommentPrefix + `swagger:parameters\p{Zs}+(\S.*?)\p{Zs}*$`)

	// rxModelArg / rxResponseArg loosely capture the raw name argument following a single-name struct
	// marker, regardless of whether it is a well-formed identifier.
	//
	// They back the malformed-name detection that warns instead of silently dropping a marker whose
	// name the strict rxModelOverride / rxResponseOverride rejects (e.g. a package-qualified
	// `utils.Error`).
	// See parsers.MalformedModelName / MalformedResponseName.
	rxModelArg    = regexp.MustCompile(rxCommentPrefix + `swagger:model\p{Zs}+(\S.*?)\p{Zs}*$`)
	rxResponseArg = regexp.MustCompile(rxCommentPrefix + `swagger:response\p{Zs}+(\S.*?)\p{Zs}*$`)

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
