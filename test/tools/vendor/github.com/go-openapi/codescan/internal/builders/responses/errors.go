// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package responses

import "errors"

// ErrResponses is the sentinel error for all errors originating from the responses package.
var ErrResponses = errors.New("codescan:builders:responses")

// errUnrepresentableHeader is an internal sentinel signalling that a response field has no OAS v2
// SimpleSchema representation in a header (non-body) context — e.g. a Go map.
//
// The field-level caller (processResponseField) recognizes it, records a diagnostic, and skips the
// header instead of corrupting the response body schema.
// Mirrors parameters.errUnrepresentableParam.
// See go-swagger/go-swagger#2804.
var errUnrepresentableHeader = errors.New("codescan:builders:responses:unrepresentable")
