// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codescan

import "github.com/go-openapi/codescan/internal/parsers/grammar"

// Diagnostic is one observation the scanner makes about the source it processes — a
// parse/validation issue, a dropped construct, or an informational note.
//
// Every scan-time observation is delivered to [Options.OnDiagnostic]; codescan never writes to
// stdout/stderr.
//
// Fields: Pos (a go/token.Position, zero when no single source location applies), Severity, Code (a
// stable machine-readable identifier), and a human-readable Message.
// The String method renders it in compiler-style one-line form.
type Diagnostic = grammar.Diagnostic

// Severity classifies a [Diagnostic]'s seriousness.
//
// The scan never aborts on a Warning or Hint; the caller decides policy.
// Compare against [SeverityError], [SeverityWarning] and [SeverityHint].
type Severity = grammar.Severity

// Code is a stable, machine-readable identifier for a class of [Diagnostic] (e.g.
// "validate.unsupported-go-type", "scan.ignored-by-tag").
//
// Codes are grouped by prefix: parse.* (lexer/parser), validate.* (semantic), scan.* (scan
// environment).
// Callers may switch on it to filter or route diagnostics.
type Code = grammar.Code

// Severity levels, ordered from most to least serious.
const (
	SeverityError   = grammar.SeverityError
	SeverityWarning = grammar.SeverityWarning
	SeverityHint    = grammar.SeverityHint
)
