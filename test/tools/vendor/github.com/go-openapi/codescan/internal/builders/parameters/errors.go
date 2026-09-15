// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parameters

import "errors"

// ErrParameters is the sentinel error for all errors originating from the parameters package.
var ErrParameters = errors.New("codescan:builders:parameters")

// Two internal sentinels for "drop this field rather than fail the scan".
//
// Both are handled the same way by the field-level caller (processParamField) — record a located diagnostic, skip the
// field — but they say different things to the author, and one message cannot serve both.
//
// The distinction is not cosmetic: reported under the wrong reason, a body parameter dropped because its Go type is
// meaningless to a client was blaming a SimpleSchema restriction that does not apply to `in: body` at all, sending the
// reader to fix a location that was never the problem.
var (
	// errUnrepresentableParam signals that a field has no OAS v2 SimpleSchema representation in a non-body parameter
	// context (query/formData/path/header) — e.g. a Go map.
	//
	// The same type is perfectly representable under `in: body`, so the location is the whole of the reason.
	// See go-swagger/go-swagger#2804.
	errUnrepresentableParam = errors.New("codescan:builders:parameters:unrepresentable")

	// errNotAParameter signals a Go type that is meaningless as an inbound value in ANY location — currently `error`.
	//
	// No choice of `in:` makes it sendable, so the message must not name one.
	errNotAParameter = errors.New("codescan:builders:parameters:not-a-parameter")
)
