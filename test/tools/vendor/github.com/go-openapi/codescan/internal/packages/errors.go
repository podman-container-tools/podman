// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package packages

import "errors"

// Sentinels for the conditions a caller might reasonably branch on.
//
// The detail (which package, which pattern) goes in the wrapping message; errors.Is matches the sentinel.
var (
	// ErrImportCycle reports that the package graph loops back on itself.
	//
	// A well-formed Go program has no import cycle, so this means either the tree under scan does not compile, or the
	// resolver mapped two directories to the same import path.
	ErrImportCycle = errors.New("import cycle")

	// ErrStrategyUnavailable reports a loading strategy that cannot run in this build — the go/packages strategy under
	// WebAssembly, which has no process model to run `go list` with.
	ErrStrategyUnavailable = errors.New("loading strategy unavailable in this build")
)
