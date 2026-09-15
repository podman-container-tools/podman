// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package packages

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/go-openapi/codescan/internal/packages/list"
	"github.com/go-openapi/codescan/internal/packages/vfs"
)

// Loader loads and type-checks Go packages.
//
// The [Loader] supports two loading strategies: using the standard go tool chain (golang.org/x/tools/go/packages,
// bends on "go list" and requires a go compiler to be installed) or as pure go, without toolchain.
//
// The latter strategy remains experimental for now: we use it to support a WASI run.
// The go toolchain has evolved over time and we'll always support the standard toolchain strategy.
//
// However, our pure go loader comes with some other benefits (significant less memory is used, may usee [fs.FS])
// and we may transition to pure go being the default strategy in future releases.
//
// A Loader is single-use per Load call in the sense that it caches nothing across calls.
// It is safe to keep one around and call Load repeatedly.
type Loader struct {
	opts *options
	vfs  *vfs.FS
}

// NewLoader returns a Loader reading through the real filesystem unless [WithFS] says otherwise.
func NewLoader(opts ...Option) *Loader {
	o := newOptions(opts)
	return &Loader{opts: o, vfs: vfs.New(o.fsys)}
}

// Load resolves patterns into type-checked packages, using whichever strategy the Loader was configured with.
//
// See [WithStrategy].
//
// The signature mirrors [golang.org/x/tools/go/packages.Load] so the two are interchangeable at the call site.
//
// Both strategies deliver the same shape: named packages with files, transitive imports, syntax and type information,
// so a caller reads the result the same way whichever one ran.
func (l *Loader) Load(cfg *Config, patterns ...string) ([]*Package, error) {
	if cfg == nil {
		cfg = &Config{}
	}

	if l.Strategy() == StrategyGoPackages {
		return l.loadViaGoPackages(cfg, patterns...)
	}

	return l.loadFromSource(cfg, patterns...)
}

// loadFromSource is [StrategyToolchainFree]: resolve, parse and type-check in pure Go.
//
// cfg.Mode is accepted and ignored here: there is no cheaper mode on offer, so this strategy always produces syntax
// and type information.
func (l *Loader) loadFromSource(cfg *Config, patterns ...string) ([]*Package, error) {
	env := environment(cfg)
	ctx := l.buildContext(cfg, env)

	goEnv := l.opts.goEnv.resolve(env)
	res, err := list.NewResolver(list.Config{
		FS:         l.vfs,
		Context:    ctx,
		Dir:        cfg.Dir,
		Env:        env,
		GOWORK:     goEnv.GOWORK,
		ModFlag:    modFlag(goEnv.GOFLAGS, cfg.BuildFlags),
		StubStdlib: l.opts.stubStdlib,
	})
	if err != nil {
		return nil, err
	}

	dirs, err := res.ResolvePatterns(patterns)
	if err != nil {
		return nil, err
	}

	ld := &loadState{
		vfs:       l.vfs,
		ctx:       ctx,
		res:       res,
		fset:      token.NewFileSet(),
		byPath:    map[string]*Package{},
		inProg:    map[string]bool{},
		stubs:     map[string]*types.Package{},
		stubbable: map[string]bool{},
		reported:  map[string]bool{},

		onSynthesize: l.opts.onSynthesize,
		onExportOnly: l.opts.onExportOnly,

		exportOnlyReported: map[string]bool{},
		annotated:          map[string]bool{},
		srcFiles:           map[string][]string{},
		stubStdlib:         l.opts.stubStdlib,

		exportFS:         l.opts.exportFS,
		exported:         map[string]*types.Package{},
		exportInProgress: map[string]bool{},
	}

	roots := make([]*Package, 0, len(dirs))
	for _, d := range dirs {
		p, err := ld.loadDir(d.Dir, d.PkgPath, true)
		if err != nil {
			return nil, err
		}
		if p != nil {
			roots = append(roots, p)
		}
	}
	return roots, nil
}

// buildContext derives the go/build context that decides which files each package is built from.
//
// The build environment is resolved in three tiers, weakest first:
//   - the platform the loader runs on,
//   - then Config.Env (parity with packages.Load),
//   - then an explicit [WithGoEnv].
//
// The tiering matters because the weakest tier is the one that may lie: inside a WASI guest it says wasip1, and every
// _linux.go file would silently vanish from the spec.
func (l *Loader) buildContext(cfg *Config, env map[string]string) *build.Context {
	ctx := build.Default
	ctx.GOOS = runtime.GOOS
	ctx.GOARCH = runtime.GOARCH
	ctx.Dir = cfg.Dir

	// GOROOT and GOPATH are locations, not build inputs, so they stay ambient rather than joining GoEnv.
	for k, v := range env {
		switch k {
		case "GOROOT":
			ctx.GOROOT = v
		case "GOPATH":
			ctx.GOPATH = v
		case "CGO_ENABLED":
			ctx.CgoEnabled = v == "1"
		}
	}

	goEnv := l.opts.goEnv.resolve(env)
	if goEnv.GOOS != "" {
		ctx.GOOS = goEnv.GOOS
	}
	if goEnv.GOARCH != "" {
		ctx.GOARCH = goEnv.GOARCH
	}

	// GOFLAGS supplies defaults; flags passed explicitly win, so it goes on first.
	if tags := buildTags(goEnv.GOFLAGS, cfg.BuildFlags); len(tags) > 0 {
		ctx.BuildTags = append(ctx.BuildTags, tags...)
	}
	ctx.ToolTags = applyExperiments(ctx.ToolTags, goEnv)

	// Route every read through the vfs so a virtualized tree is honoured by constraint matching too.
	ctx.OpenFile = l.vfs.Open
	ctx.ReadDir = l.vfs.ReadDir
	ctx.IsDir = l.vfs.IsDir
	ctx.JoinPath = l.vfs.Join
	ctx.IsAbsPath = l.vfs.IsAbs
	ctx.HasSubdir = l.vfs.HasSubdir

	return &ctx
}

// buildTags extracts -tags values from GOFLAGS and BuildFlags, accepting the three spellings the go command does.
//
// GOFLAGS is read first because the go command treats it as a source of defaults that an explicit flag overrides.
// Tags accumulate rather than replace, which is also what the go command does for this particular flag.
func buildTags(goflags string, buildFlags []string) []string {
	flags := append(strings.Fields(goflags), buildFlags...)

	var tags []string
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		switch {
		case f == "-tags" && i+1 < len(flags):
			i++
			tags = append(tags, splitTags(flags[i])...)
		case strings.HasPrefix(f, "-tags="):
			tags = append(tags, splitTags(strings.TrimPrefix(f, "-tags="))...)
		}
	}

	return tags
}

// applyExperiments folds GOEXPERIMENT into the toolchain tags go/build starts from.
//
// build.Default.ToolTags is computed at init from the configuration CODESCAN was built with, so it is a baseline, not
// an answer about the code under scan.
// Enabling adds a tag, "noX" removes one, and "none" clears the lot — which is why removals have to be honoured and
// not just additions.
func applyExperiments(base []string, goEnv GoEnv) []string {
	if goEnv.GOEXPERIMENT == "" {
		return base
	}

	keep := base
	if goEnv.clearsExperiments() {
		keep = nil
		for _, t := range base {
			if !strings.HasPrefix(t, "goexperiment.") {
				keep = append(keep, t)
			}
		}
	}

	add, remove := goEnv.experimentTags()
	out := make([]string, 0, len(keep)+len(add))
	for _, t := range keep {
		if !slices.Contains(remove, t) {
			out = append(out, t)
		}
	}
	for _, t := range add {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}

	return out
}

// modFlag extracts -mod= from GOFLAGS and BuildFlags, with the explicit flag winning.
//
// It decides whether a vendor directory is authoritative, so it belongs with the other inputs that change what gets
// built rather than merely how.
func modFlag(goflags string, buildFlags []string) string {
	flags := append(strings.Fields(goflags), buildFlags...)

	mode := ""
	for i := 0; i < len(flags); i++ {
		f := flags[i]
		switch {
		case f == "-mod" && i+1 < len(flags):
			i++
			mode = flags[i]
		case strings.HasPrefix(f, "-mod="):
			mode = strings.TrimPrefix(f, "-mod=")
		}
	}

	return mode
}

func splitTags(s string) []string {
	var out []string
	for _, t := range strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' }) {
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// environment renders the effective environment for a load.
//
// A nil Config.Env means the current environment, as it does for packages.Load.
// It is not cosmetic: GOROOT and GOMODCACHE are how the resolver finds the standard library and the module cache, so an
// empty environment silently reduces every external import to an unresolved stub.
func environment(cfg *Config) map[string]string {
	if cfg.Env == nil {
		return envMap(os.Environ())
	}

	return envMap(cfg.Env)
}

func envMap(env []string) map[string]string {
	m := make(map[string]string, len(env))
	for _, e := range env {
		if k, v, ok := strings.Cut(e, "="); ok {
			m[k] = v
		}
	}
	return m
}

// loadState carries the per-Load caches.
type loadState struct {
	vfs  *vfs.FS
	ctx  *build.Context
	res  *list.Resolver
	fset *token.FileSet

	byPath map[string]*Package
	inProg map[string]bool
	stubs  map[string]*types.Package

	// stubbable memoizes whether an import path resolves.
	// The synthesis pre-walk does not stat the same paths once per importing file.
	stubbable map[string]bool

	// reported tracks which synthesized paths have already been announced.
	// A package imported from fifty files is reported once.
	reported map[string]bool

	onSynthesize func(Synthesized)
	onExportOnly func(ExportOnly)
	stubStdlib   bool

	// exportOnlyReported keeps the export-without-source notice to once per path.
	exportOnlyReported map[string]bool

	// annotated memoizes whether a dependency's source carries annotations, so the files behind one import are read once
	// however many packages import it.
	annotated map[string]bool

	// srcFiles memoizes where a dependency's source is, so the marker scan and any later read-back resolve one import
	// once. See sourceFiles.
	srcFiles map[string][]string

	// exportFS serves pre-computed export data.
	// It is keyed by import path with an ".export" suffix and remans nil when the caller supplied none,
	// in which case every package is read from source.
	exportFS         fs.FS
	exported         map[string]*types.Package
	exportInProgress map[string]bool
}

// loadDir parses and type-checks the package rooted at dir.
func (ld *loadState) loadDir(dir, pkgPath string, root bool) (*Package, error) {
	if p, ok := ld.byPath[pkgPath]; ok {
		return p, nil
	}
	if ld.inProg[pkgPath] {
		return nil, fmt.Errorf("%w through %q", ErrImportCycle, pkgPath)
	}
	ld.inProg[pkgPath] = true
	defer delete(ld.inProg, pkgPath)

	var errs []Error

	bp, err := ld.ctx.ImportDir(dir, 0)
	if err != nil {
		var nogo *build.NoGoError
		if errors.As(err, &nogo) {
			// A constraint-excluded directory is not an error, it is simply not a package.
			// Callers discriminate on the nil *Package, which is why no sentinel is invented for it.
			return nil, nil //nolint:nilnil // "not a package" is a normal outcome, not a failure
		}

		// Anything else — two package clauses in one directory, an unparseable file, an invalid import path — is reported
		// ON the package rather than raised, which is what `go list -e` does.
		//
		// Failing the whole load instead means one broken file anywhere under ./... yields no spec at all,
		// and a tree with a file mid-edit is the ordinary case, not the exotic one.
		//
		// go/build fills the package in as far as it got before giving up.
		// The files it classified are still reported: eecovering them by hand would mean reimplementing
		// its constraint matching to answer a question it has already answered.
		if bp == nil {
			bp = &build.Package{}
		}

		errs = append(errs, Error{Pos: dir, Msg: err.Error(), Kind: ListError})
	}

	// Cgo files may be part of the package too.
	//
	// go/build keeps them in a list of their own because they need the cgo tool before they can be compiled.
	// Codescan never compiles, so for reading declarations and comments they are ordinary source,
	// and annotated types do live in them (go-swagger#1096).
	//
	// The "C" pseudo-package such files import resolves like any other unloadable import, through synthesis.
	files := make([]string, 0, len(bp.GoFiles)+len(bp.CgoFiles))
	for _, f := range bp.GoFiles {
		files = append(files, ld.vfs.Join(dir, f))
	}
	cgoFiles := make([]string, 0, len(bp.CgoFiles))
	for _, f := range bp.CgoFiles {
		full := ld.vfs.Join(dir, f)
		files = append(files, full)
		cgoFiles = append(cgoFiles, full)
	}

	syntax := make([]*ast.File, 0, len(files))
	for _, fn := range files {
		src, err := ld.vfs.ReadFile(fn)
		if err != nil {
			errs = append(errs, Error{Pos: fn, Msg: err.Error(), Kind: ParseError})
			continue
		}

		// Comments are the payload for codescan, never skip them.
		// Object resolution is the opposite: it builds the legacy [ast.Object] graph that predates go/types,
		// which does its own resolution and ignores it.
		// Nothing here reads it.
		f, err := parser.ParseFile(ld.fset, fn, src, parser.ParseComments|parser.SkipObjectResolution)
		if f == nil {
			errs = append(errs, Error{Pos: fn, Msg: err.Error(), Kind: ParseError})
			continue
		}
		if err != nil {
			errs = append(errs, Error{Pos: fn, Msg: err.Error(), Kind: ParseError})
		}
		syntax = append(syntax, f)
	}

	pkg := &Package{
		ID:              pkgPath,
		Name:            bp.Name,
		PkgPath:         pkgPath,
		GoFiles:         files,
		CompiledGoFiles: files,
		Syntax:          syntax,
		Fset:            ld.fset,
		Imports:         map[string]*Package{},
	}
	ld.byPath[pkgPath] = pkg

	// Fabricate whatever this package selects through imports that will not resolve.
	// This way, the checker has something to bind those selectors to.
	ld.synthesizeFrom(syntax, dir)

	// Only the two maps codescan reads.
	// [types.Info] is pure output — every map handed in is one the checker then fills for every node it visits.
	//
	// Notice that Types alone was 30% of the loader's allocation as Uses, Implicits, Selections, Scopes and Instances
	// were being built in full and never consulted by the scanner.
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
		Defs:  map[*ast.Ident]types.Object{},
	}
	conf := types.Config{
		Importer: &importer{ld: ld, from: pkg, fromDir: dir},
		// Function bodies are checked for the packages under scan.
		//
		// An annotated type may be declared inside one (see the swagger:response in classification/operations/responses.go),
		// and skipping bodies leaves no TypesInfo.Defs entry for it, so the scanner never sees it.
		//
		// A dependency is a different matter.
		// Nothing inside its function bodies can be reached from outside it, and codescan reads a dependency only for the
		// types its declarations expose.
		//
		// Since dependencies are the bulk of any graph — the standard library alone dwarfs the module under scan —
		// checking their bodies was most of the work the previous naive loader did, and all of it was eventually discarded.
		IgnoreFuncBodies:         !root,
		DisableUnusedImportCheck: true,
		Error: func(err error) {
			errs = append(errs, typeError(err))
		},
	}
	tpkg, _ := conf.Check(pkgPath, ld.fset, syntax, info)

	pkg.Types = tpkg
	pkg.TypesInfo = info
	pkg.Errors = dropCgoFileErrors(errs, cgoFiles)
	pkg.IllTyped = len(pkg.Errors) > 0
	return pkg, nil
}

// typeError converts a type-checker error, keeping the position separate from the message.
//
// go/types reports a *types.Error carrying a token.Pos; flattening it into the message alone loses the one field
// a caller can act on, and renders as "-: file:line: msg" when something later asks where the error was.
func typeError(err error) Error {
	var terr types.Error
	if errors.As(err, &terr) {
		return Error{
			Pos:  terr.Fset.Position(terr.Pos).String(),
			Msg:  terr.Msg,
			Kind: TypeError,
		}
	}

	return Error{Msg: err.Error(), Kind: TypeError}
}

// dropCgoFileErrors removes the type errors that exist only because we do not run the cgo tool.
//
// A file importing "C" is read for its declarations — that is the whole of go-swagger#1096 — but the C
// pseudo-package is fabricated rather than generated, so every mention of it fails to type-check: "name int not
// exported by package C", "undefined: C.malloc", "cannot convert 1 to type C.malloc".
//
// None of that says anything about the code under scan, and left in place it surfaces as a warning telling the author
// their package did not type-check.
//
// The filter is by POSITION rather than by message, because the messages are many and the position is exact.
// An error inside a cgo file is one we cannot have an opinion on, since the C half of that file was never compiled.
//
// Errors elsewhere in the package are untouched and still reported.
func dropCgoFileErrors(errs []Error, cgoFiles []string) []Error {
	if len(cgoFiles) == 0 || len(errs) == 0 {
		return errs
	}

	kept := errs[:0:0]
	for _, e := range errs {
		if e.Kind == TypeError && inAnyFile(e.Pos, cgoFiles) {
			continue
		}
		kept = append(kept, e)
	}

	return kept
}

// inAnyFile reports whether a "file:line:col" position falls in one of files.
func inAnyFile(pos string, files []string) bool {
	for _, f := range files {
		if strings.HasPrefix(pos, f+":") {
			return true
		}
	}

	return false
}

// importer resolves the imports of one package.
type importer struct {
	ld   *loadState
	from *Package
	// fromDir is the directory holding the importing package's source.
	//
	// Carried because it decides whether the standard library's own vendor tree is in scope: a package inside GOROOT/src
	// resolves golang.org/x/crypto/... to the copy vendored beside it, and a package anywhere else must not.
	fromDir string
}

func (i *importer) Import(importPath string) (*types.Package, error) {
	if importPath == "unsafe" {
		return types.Unsafe, nil
	}
	// Pre-computed types beat reading source: the same answer, none of the work.
	// A gap in the tree falls through to source, and from there to synthesis (dependencies only).
	//
	// The module under scan is always read from source, because its comments are the annotations
	// and export data does not carry them.
	if i.ld.exportFS != nil && !i.ld.res.InMainModule(importPath) {
		if tpkg, err := i.ld.importExported(importPath); err == nil && !i.ld.carriesAnnotations(importPath) {
			// Nothing in this package's source speaks to a scan, so its types are the whole of what it has to offer.
			// One that DOES speak falls through and is read from source like any other — half-loading it would give the
			// builders declarations with no record of what they denote.
			//
			// Recorded, not announced. Whether taking only the types costs anything is not knowable here — it depends on
			// whether some builder later wants a declaration out of this package — so the fact is kept and replayed at
			// the lookup that wanted one. Saying it now would bury the reader under the whole standard library.
			i.ld.reportExportOnly(importPath, "nothing in its source is annotated, so it was not parsed")
			i.from.Imports[importPath] = i.ld.exportedPackage(importPath, tpkg)

			return tpkg, nil
		}
	}

	dir, pkgPath, ok := i.ld.res.ResolveImportFrom(importPath, i.fromDir)
	if ok {
		dep, err := i.ld.loadDir(dir, pkgPath, false)
		if err == nil && dep != nil && dep.Types != nil {
			i.from.Imports[importPath] = dep
			return dep.Types, nil
		}
	}

	// Unresolvable: hand back an empty package rather than failing the whole check.
	// The referencing type resolves to "invalid", which the builders report at the point of use — a far more useful
	// diagnostic than aborting the scan.
	return i.ld.stub(importPath), nil
}

func (ld *loadState) stub(importPath string) *types.Package {
	if p, ok := ld.stubs[importPath]; ok {
		return p
	}
	p := types.NewPackage(importPath, pkgNameFor(importPath))
	p.MarkComplete()
	ld.stubs[importPath] = p
	return p
}

func pkgNameFor(importPath string) string {
	if i := strings.LastIndex(importPath, "/"); i >= 0 {
		return importPath[i+1:]
	}
	return importPath
}
