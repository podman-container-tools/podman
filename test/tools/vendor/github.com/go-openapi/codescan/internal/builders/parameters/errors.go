// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parameters

import "errors"

// ErrParameters is the sentinel error for all errors originating from the parameters package.
var ErrParameters = errors.New("codescan:builders:parameters")

// errUnrepresentableParam is an internal sentinel signalling that a struct field has no OAS v2
// SimpleSchema representation in a non-body parameter context (query/formData/path/header) — e.g.
// a Go map.
//
// The field-level caller (processParamField) recognizes it, records a diagnostic, and skips the
// field instead of failing the whole scan.
// See go-swagger/go-swagger#2804.
var errUnrepresentableParam = errors.New("codescan:builders:parameters:unrepresentable")
