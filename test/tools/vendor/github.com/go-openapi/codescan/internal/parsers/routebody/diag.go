// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package routebody

import (
	"fmt"
	"go/token"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
)

// emitDiagf forwards a CodeInvalidAnnotation diagnostic to the caller's sink.
//
// A nil sink silently drops the diagnostic — matches the optionality used by handlers'
// dispatchers.
//
// Routebody emits exactly one diagnostic code, so the helper bakes it in rather than accepting an
// argument.
// Future codes can be added by adding sibling helpers (emitShapeMismatchf, etc.) — keep the call
// sites explicit about what code they ride on.
func emitDiagf(diag func(grammar.Diagnostic), pos token.Position, format string, args ...any) {
	if diag == nil {
		return
	}
	diag(grammar.Diagnostic{
		Pos:      pos,
		Severity: grammar.SeverityWarning,
		Code:     grammar.CodeInvalidAnnotation,
		Message:  fmt.Sprintf(format, args...),
	})
}

// offsetPos returns a new Position whose Line is base.Line + lineNo - 1 (1-indexed line numbers
// within body).
//
// Column and Filename are inherited from base.
// Used to give per-line diagnostics a reasonable Pos when the body parser doesn't track precise lex
// state.
func offsetPos(base token.Position, lineNo int) token.Position {
	return token.Position{
		Filename: base.Filename,
		Offset:   base.Offset,
		Line:     base.Line + lineNo - 1,
		Column:   base.Column,
	}
}
