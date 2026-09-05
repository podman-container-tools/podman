// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build wasm

package packages

import "fmt"

// loadViaGoPackages is absent from WebAssembly builds.
//
// packages.Load resolves a package graph by running `go list`, and WebAssembly has no process model, so this strategy
// could only ever fail — and it would fail deep inside the go command's plumbing, reading as a broken toolchain
// rather than as a strategy that cannot exist here.
// Refusing it up front is the whole point of the split.
//
// It does NOT keep golang.org/x/tools/go/packages out of the artifact: the package is imported untagged for the type
// aliases in aliases.go, which are load-bearing for the "swap the call and nothing else" contract.
// The saving is a clear error, not a smaller binary.
func (l *Loader) loadViaGoPackages(_ *Config, _ ...string) ([]*Package, error) {
	return nil, fmt.Errorf("%w: %s needs a process model; use %s",
		ErrStrategyUnavailable, StrategyGoPackages, StrategyToolchainFree)
}
