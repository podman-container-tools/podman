// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package packages

import (
	"go/token"
	"io/fs"
)

// Option configures a [Loader].
type Option func(*options)

type options struct {
	strategy     Strategy // zero value: StrategyGoPackages
	compiledDeps bool
	fsys         fs.FS // nil: read through the real filesystem
	goEnv        GoEnv
	stubStdlib   bool
	onSynthesize func(Synthesized)
	onExportOnly func(ExportOnly)
	exportFS     fs.FS
}

// WithExportData serves dependencies from pre-computed export data instead of reading their source.
//
// fsys holds one file per package, named by import path with a ".export" suffix — the layout hack/genexportdata
// produces.
// Whole compiled archives are accepted as well as bare export sections.
//
// It applies to dependencies only.
// The module under scan is always read from source: its comments are the annotations, and export data carries none.
//
// This is the fast path with none of the fidelity loss of [WithStubbedStdlib]: the types are the ones the compiler
// computed, so fields, method sets and interface identity are all real.
// A full scan otherwise spends nearly all of its time parsing and type-checking its dependencies, and a WebAssembly
// guest pays a five- to six-fold compute tax on top of that.
//
// hack/genexportdata produces the tree, as a directory or as a zip (archive/zip's reader is an [fs.FS], so an embedded
// build carries one file and still reads per package).
//
// It is consulted per dependency, and the decision is whole.
// A package whose source carries swagger annotations is read from source in the ordinary way: export data holds
// types and not comments, and the two cannot be combined after the fact, since go/types records what a type expression
// denotes behind an unexported field.
//
// Every other package is taken from here and never parsed at all.
//
// Nothing is lost by that, and little is given up: the saving was never in the handful of packages a scan reads,
// but in the closure behind them, which this still serves.
// Where a dependency's source cannot be found at all, [WithOnExportOnly] says so.
//
// The data is only valid for the toolchain that produced it, since the export format is tied to the Go release.
// A package the tree does not cover falls back to source, and then to synthesis.
func WithExportData(fsys fs.FS) Option {
	return func(o *options) { o.exportFS = fsys }
}

// Synthesized reports an import whose types were fabricated rather than loaded.
type Synthesized struct {
	// Path is the import path.
	Path string

	// Pos is the import that triggered the synthesis — the first one seen for this path.
	Pos token.Position

	// Deliberate distinguishes the standard library withheld by [WithStubbedStdlib] from an import that simply could not
	// be found.
	//
	// The first is the caller's own choice; the second is usually a mounting or module-cache problem.
	Deliberate bool

	// Cgo marks the "C" pseudo-package, which is neither of the above: it has no source anywhere to find, and is
	// fabricated because this loader does not run the cgo tool.
	//
	// Worth telling apart, since "could not be resolved" invites a reader to go looking for something
	// that was never there.
	Cgo bool
}

// WithOnSynthesized registers a callback fired once per import path that had to be synthesized.
//
// Without it, the loss is invisible: a package that only mentions a synthesized type in a field position type-checks
// cleanly and simply produces a thinner spec.
// Otherwise only the downstream wreckage surfaces: a value-position use of a fabricated type reads as an error in
// the scanned code rather than as a missing dependency.
func WithOnSynthesized(fn func(Synthesized)) Option {
	return func(o *options) { o.onSynthesize = fn }
}

// ExportOnly reports a dependency whose types were read from export data but whose source was not available.
//
// What it says about those types — its annotations — could not be read.
type ExportOnly struct {
	// Path is the import path.
	Path string

	// Reason says which half failed: the source was not on the filesystem, or it would not parse.
	Reason string
}

// WithOnExportOnly registers a callback fired once per dependency whose types came from export data without its source.
//
// Export data plus source is the intended shape — the compiler's answer for the types, the file's own words for the
// annotations.
// This fires when only the first half was available, which is a real loss and an invisible one: the spec comes out
// valid and quieter.
func WithOnExportOnly(fn func(ExportOnly)) Option {
	return func(o *options) { o.onExportOnly = fn }
}

// WithStubbedStdlib keeps the standard library out of the package graph.
//
// Standard-library imports are then synthesized from the names selected through them — opaque types carrying the
// right package path and name — instead of being parsed and type-checked out of GOROOT.
//
// The trade is fidelity for reach.
// Everything keyed on a type's identity survives: codescan recognizes time.Time, json.RawMessage and friends by
// (package, name), never by shape.
// Everything structural is lost: a synthesized type has no fields to drill into and no method set, so a spec
// that renders json.RawMessage as a byte array, time.Duration as an integer, or that depends on a type implementing
// encoding.TextMarshaler, comes out different.
//
// The reach bought is a small footprint and no Go installation: GOROOT no longer has to exist and no module cache has
// to be populated, which for a WASI guest or a browser is the difference between scanning and not.
//
// It is not failsafe, and the failure mode is quiet — the spec comes out subtly thinner rather than erroring.
// Across codescan's own fixture corpus 133 of 138 scans are byte-identical; the rest lose a byte-array rendering, an
// integer format, or a TextMarshaler-derived string, and stdlib interfaces such as io.Reader have no identity
// recognizer to fall back on at all.
//
// [Loader.ReadBackSource] does not recover any of this.
// It gives a dependency back the source the load declined to read, and a synthesized package never had source to
// decline: it was fabricated from names, not read from files.
//
// Note that synthesis is not exclusive to this option: an import that cannot be resolved is synthesized whether or not
// the standard library was withheld.
// The option only makes it deliberate for the one dependency every Go program has.
//
// Prefer a full graph wherever GOROOT is available; reach for this where it is not.
func WithStubbedStdlib() Option {
	return func(o *options) { o.stubStdlib = true }
}

// WithCompiledDependencies takes dependency types from the compiler's export data instead of reading their source.
//
// It applies to [StrategyGoPackages] only (go toolchain's loader).
// The toolchain-free strategy has [WithExportData], which is the same idea supplied by hand; here the go command
// produces the data itself, from its build cache.
//
// The speed is not marginal — parsing and type-checking dependencies is most of what a load does, and on a warm cache
// this removes nearly all of it.
// On a cold one it is markedly slower, because the dependencies have to be compiled before their export data exists.
//
// It costs dependency SOURCE.
// Export data carries the exported type surface — fields, method sets and interface identity are all real — but no
// syntax and no comments, so anything a dependency says ABOUT its types is out of reach until something reads it.
//
// For codescan that is load-bearing rather than incidental: for example go-openapi's strfmt annotates its own types,
// and those annotations give a strfmt.DateTime field its date-time format.
//
// So the source comes back twice over: [attachAnnotatedDependencies] for the packages carrying the marker,
// and [Loader.ReadBackSource] for a declaration the spec turns out to want. This option skips the parsing
// and type-checking of everything neither of those reaches.
//
// It stays an option here while being the default above, because the loader is the mechanism and the policy is the
// scanner's. The scanner adds a retry on top: `go list -export` BUILDS what it is asked about, so a scanned
// package that does not compile fails the load outright, and it is reloaded without this option rather than allowed
// to abort. See Options.CompiledDependencies.
func WithCompiledDependencies() Option {
	return func(o *options) { o.compiledDeps = true }
}

// WithFS makes the loader read source through fsys instead of the real filesystem.
//
// Every path the loader is given — Config.Dir, the patterns, and the paths it derives from them — is then
// interpreted relative to the root of fsys, following [io/fs] conventions: slash-separated, no leading slash, no "..".
// A leading slash or an OS-specific separator is normalised away rather than rejected, so a caller can pass the same
// patterns it would use natively.
//
// This is the seam that makes a virtualized source tree possible: an in-memory tree in a WASI guest,
// a [testing/fstest.MapFS] in a unit test, an archive reader, or an overlay composed from several roots.
//
// The default (no [WithFS]) reads through the os package.
//
// Notice that this also forces [StrategyToolchainFree], overriding [WithStrategy]:
// `go list` runs against the real filesystem, so it could not honour fsys even if asked.
func WithFS(fsys fs.FS) Option {
	return func(o *options) { o.fsys = fsys }
}

func newOptions(opts []Option) *options {
	o := &options{}
	for _, apply := range opts {
		apply(o)
	}
	return o
}
