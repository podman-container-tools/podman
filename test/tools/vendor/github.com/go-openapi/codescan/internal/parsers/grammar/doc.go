// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package grammar is the annotation parser for codescan.
//
// It consumes one Go comment group at a time, recognises the swagger:<name> annotation header, and
// produces a typed Block carrying:
//
//   - the recognised annotation as an AnnotationKind;
//   - per-Block fields for the annotation's positional arguments;
//   - Property entries for every recognised body keyword;
//   - prose lines split into Title() / Description();
//   - diagnostics for malformed inputs (the parser never aborts).
//
// Pipeline:
//
//	*ast.CommentGroup
//	     │
//	     ▼
//	  Preprocess  → []Line       (comment-marker stripping)
//	     │
//	     ▼
//	     Lex      → []Token      (line classifier + body accumulator + prose classifier)
//	     │
//	     ▼
//	    Parse     → Block        (dispatch by annotation family)
//
// The Token vocabulary is defined in token.go.
//
// # Details
//
// See README.md in this package for the full contract: pipeline stages, lexer / parser rules,
// keyword table, walker dispatch table, body-termination rules, diagnostics codes, and known
// follow-ups.
package grammar
