// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package list answers the question `go list` answers: given a pattern or an import path, which directory holds that
// package, and what is it called.
//
// It is separate from the loader above it because the two are inherited from different places and age differently.
// The loader is a simplified golang.org/x/tools/go/packages — a small, stable shape that codescan owns.
// This package is cmd/go: module boundaries, workspaces, vendoring, the module cache, and a wildcard grammar with
// documented exceptions to its own rules.
//
// That is not a design anyone would arrive at; it is a set of behaviours the go command has and every consumer must
// reproduce exactly.
// Keeping them apart means the quirks are quarantined where they can be checked against upstream rather than diffused
// through the loader.
//
// The parts, roughly in the order a scan meets them:
//
//   - resolve.go — the [Resolver]: patterns to directories, import paths to directories, the main
//     module, the module cache, `replace`, and a vendor directory when one is authoritative.
//   - workspace.go — go.work, whose `use` directives place a sibling module at the copy being worked
//     on rather than at whatever the cache holds.
//   - pattern.go — where a `...` walk starts, and which walked directories a pattern matches.
//   - pkgpattern.go — the wildcard matcher, copied verbatim from the Go distribution under its own
//     licence. See its header, and NOTICE at the repository root.
//
// # What it deliberately does not do
//
// No GOPATH mode, no module-graph walk (versions are read already-selected out of the main go.mod), no `internal/`
// visibility enforcement, and no query syntax (`all`, `std`, `pattern=`).
// Two of those omissions let codescan read a tree the go command refuses; the rest are documented limits.
//
// Some extra tooling in hack/go-loader is available to help maintainers verify the behavior against changes in the
// go toolchain.
package list
