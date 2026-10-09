// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package packages loads and type-checks the Go packages codescan scans.
//
// A [Loader] can resolve a package graph two ways, and [WithStrategy] picks between them:
//
//   - [StrategyGoPackages] delegates to golang.org/x/tools/go/packages — the go command's own answer,
//     and the default.
//   - [StrategyToolchainFree] does the same job in pure Go, for the environments the first cannot
//     reach.
//
// Owning the choice here rather than at the call site keeps the two honest: options such as [WithGoEnv] mean
// the same thing under both, and a caller swaps strategies without rewriting how it asks for anything.
//
// The motivation for the second strategy is that packages.Load shells out to `go list`, so it cannot run where there is
// no toolchain and no exec — a WASI guest being the case that forced the issue, and "whichever Go version happens to
// be installed" being the case that makes it worth having anyway.
//
// The entry point mirrors the original on purpose:
//
//	upstream:  packages.Load(cfg, patterns...)             ([]*packages.Package, error)
//	here:      packages.NewLoader().Load(cfg, patterns...) ([]*packages.Package, error)
//
// Config, Package, Error and LoadMode are type aliases of the upstream types, so a caller can switch back to
// packages.Load by changing the call and nothing else.
//
// Whichever strategy runs, the result has the same shape: named packages with their files, their transitive imports,
// and full syntax plus type information.
//
// # Layout
//
// The go command's own complications live one level down, in the [list] subpackage: pattern matching, module
// boundaries, workspaces, vendoring, the module cache.
//
// They are separated because the two halves are inherited from different places — this package is a simplified
// go/packages, that one is cmd/go — and because quirks are easier to check against upstream when they are quarantined
// rather than diffused.
// [vfs] holds the filesystem seam both halves read through.
//
// The loading itself stays here: choosing a strategy, parsing, type-checking, and the two ways of standing in
// for a dependency that cannot be read from source — export data, and synthesis.
//
// # What the toolchain-free strategy does not implement
//
// The upstream Config fields Context, Logf, Fset, ParseFile, Tests and Overlay are accepted and ignored, as is Mode —
// there is no cheaper mode on offer.
// Overlay in particular is not supported by design: it is documented as slow, it requires every source file to be held
// in memory, and it works against demand-driven parsing.
//
// Use [WithFS] instead, which virtualizes the filesystem one level lower.
//
// Dir, BuildFlags and Env are honoured: they select which files a package is built from.
//
// Package fields that codescan does not consume are left unset rather than hydrated.
//
// [StrategyGoPackages] passes Config through to upstream untouched, apart from forcing Tests off and filling in Mode
// when the caller left it zero.
//
// # Build constraints
//
// This section describes [StrategyToolchainFree]; under [StrategyGoPackages] file selection is the go command's own.
//
// File selection goes through [go/build], which resolves //go:build expressions and GOOS/GOARCH filename suffixes in
// pure Go.
//
// The loader always sets GOOS and GOARCH explicitly from the scan target and never inherits [build.Default]: a loader
// running inside a WASI guest would otherwise inherit GOOS=wasip1 and silently drop every _linux.go file, producing a
// different spec than the same scan run natively.
//
// Cgo follows CGO_ENABLED from the environment, as the go command does, and files importing "C" are parsed alongside
// the ordinary ones rather than skipped. go/build sorts them into CgoFiles because compiling them needs the cgo tool
// first, but codescan only reads declarations and comments, and annotated types do live in them (go-swagger#1096).
//
// The "C" pseudo-package itself is unloadable and resolves through synthesis, like any other import whose source cannot
// be found.
package packages
