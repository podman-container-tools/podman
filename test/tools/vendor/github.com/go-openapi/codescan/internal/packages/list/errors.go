// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package list

import "errors"

// Sentinels for the conditions a caller might reasonably branch on.
//
// The detail — which pattern, which file — goes in the wrapping message; errors.Is matches the sentinel.
var (
	// ErrUnresolvedPattern reports a scan pattern that names no directory the resolver can reach — most often a virtual
	// filesystem mounted narrower than the pattern reaches.
	ErrUnresolvedPattern = errors.New("cannot resolve pattern")

	// ErrInvalidGoMod reports a go.mod whose requirements could not be read.
	//
	// It is fatal rather than degraded because the alternative is placing no dependency at all and synthesizing the lot,
	// which buries the real fault under a wall of unrelated warnings.
	ErrInvalidGoMod = errors.New("cannot read go.mod")
)
