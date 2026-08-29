// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package routebody parses the body sub-language of swagger:route / swagger:operation `Parameters:`
// and `Responses:` raw blocks.
//
// The package tokenises the `+ name:` chunk grammar (parameters) and the `<code>: <tokens>` line
// grammar (responses) into typed declarations ([ParamDecl], [ResponseDecl]) plus a [grammar.Block]
// carrying the validation properties.
//
// The orchestrating builder reads the head fields directly and dispatches the Block through the
// shared handlers seam ([handlers.DispatchParamLevel0], [handlers.DispatchSchemaLevel0]).
//
// Diagnostics ride on a single code, [grammar.CodeInvalidAnnotation].
// The diag callback may be nil — a nil sink drops diagnostics silently and matches the
// optional-sink posture used elsewhere.
//
// See the package README for the full grammar specifications, the definition-fallback behaviour on
// untagged response refs, and the list of head vs validation fields.
package routebody
