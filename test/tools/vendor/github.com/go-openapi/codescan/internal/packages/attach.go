// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package packages

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
)

// ReadBackSource parses a package's source onto it and reports whether it now carries syntax.
//
// The scanner calls this when a declaration is wanted from a dependency the load took types-only.
//
// This gives callers of the [Loader] the ability to call for parsed source on-demand: they are not compelled to
// resort to an eager full compilation of the entire dependency graph.
//
// Both [Loader] strategies pass over such a dependency on the same reasoning — export data holds types and not comments,
// so a package that says nothing about its own types has nothing to say.
//
// Both are asking the wrong question the moment some scanned code names one of its types as a model.
// A package says things in its comments and declares them in its source,
// and a definition renders from that declaration or not at all.
//
// Asking here rather than reading every dependency up front keeps it affordable: the cost is one parse per
// declaration wanted, against one per dependency loaded, typically single digits against several hundred.
//
// This is a method on the [Loader] because reading is: [WithFS] means a scan's whole world can be a virtual tree,
// and a read-back going to the real filesystem would answer from outside it.
//
// There is deliberately no marker check because the caller has already established that the source is wanted,
// by a better question than the marker asks.
//
// It is idempotent: asking twice only costs one parse.
func (l *Loader) ReadBackSource(pkg *Package) bool {
	return readBackSource(pkg, l.vfs.Open)
}

func readBackSource(pkg *Package, open func(string) (io.ReadCloser, error)) bool {
	if pkg == nil || pkg.Types == nil || pkg.Fset == nil {
		return false
	}
	if len(pkg.Syntax) > 0 {
		return true
	}
	if len(pkg.GoFiles) == 0 {
		return false
	}

	syntax := parseFilesForComments(pkg.Fset, pkg.GoFiles, open)
	if len(syntax) == 0 {
		return false
	}

	pkg.Syntax = syntax
	pkg.CompiledGoFiles = pkg.GoFiles
	pkg.TypesInfo = bridgeDefs(syntax, pkg.Types)

	return true
}

// parseFilesForComments reads a package's source for what it says, not for what it means.
//
// No type-checking follows, so this is parsing alone — the cheap half — and the comments are the entire reason for
// doing it. Object resolution is skipped for the same reason: the objects are already in the export-data scope.
//
// The bytes come from the caller's filesystem rather than from the parser's own read, so a virtual tree is honoured;
// the path is still handed to [parser.ParseFile], because it is what the positions are recorded against.
//
// Those positions have to line up with the ones export data carries, and only the line and the file's base name
// actually do. The compiler and `go list` do not spell a path the same way — on Windows they differ in separator and
// in drive-letter case — so nothing downstream may compare these names whole.
func parseFilesForComments(fset *token.FileSet, paths []string, open func(string) (io.ReadCloser, error)) []*ast.File {
	syntax := make([]*ast.File, 0, len(paths))

	for _, path := range paths {
		src, err := readSource(open, path)
		if err != nil {
			continue
		}

		f, err := parser.ParseFile(fset, path, src, parser.ParseComments|parser.SkipObjectResolution)
		if f == nil {
			continue
		}
		_ = err // a partially parsed file still carries the declarations above the fault

		syntax = append(syntax, f)
	}

	return syntax
}

func readSource(open func(string) (io.ReadCloser, error), path string) ([]byte, error) {
	f, err := open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return io.ReadAll(f)
}

// bridgeDefs joins parsed declarations to the objects export data already holds.
//
// It is a name lookup rather than a type-check: a top-level declaration is in the package scope
// under exactly its own name, which is all [types.Info.Defs] is asked for here.
//
// Unexported names are absent from export data and stay unmapped, which is correct.
// Nothing outside the package can refer to one.
func bridgeDefs(syntax []*ast.File, tpkg *types.Package) *types.Info {
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}}
	scope := tpkg.Scope()

	for _, f := range syntax {
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range gen.Specs {
				switch sp := spec.(type) {
				case *ast.TypeSpec:
					if obj := scope.Lookup(sp.Name.Name); obj != nil {
						info.Defs[sp.Name] = obj
					}
				case *ast.ValueSpec:
					for _, name := range sp.Names {
						if obj := scope.Lookup(name.Name); obj != nil {
							info.Defs[name] = obj
						}
					}
				}
			}
		}
	}

	return info
}

// attachAnnotatedDependencies gives a dependency its source back when that source has something to say.
//
// The two strategies arrive at the same policy from opposite ends, because they have opposite amounts of control.
//
// The toolchain-free one resolves imports itself, so it decides per dependency while the load is happening:
// a package whose source carries the marker is read from source, one that does not is taken from export data untouched.
//
// Under [StrategyGoPackages], `go list` and go/packages own resolution, and the only lever is a [LoadMode]:
// one value for the whole load, with no hook to say "except this one".
//
// So the choice cannot be made during the load and is made after it: take every dependency from export data,
// then hand back the source of the few that were worth reading.
//
// This works because the cheap load still records where the source IS.
// compiledDepsMode keeps packages.NeedFiles, so a dependency comes back with its GoFiles populated,
// its types complete and no syntax — locatable, just unread.
// Parsing those files is the whole of the work; nothing is type-checked twice, because every declaration the source
// names is already an object in the export-data scope and the two halves are joined by name.
//
// The assembled shape — export-data types beside separately parsed syntax — carries no [types.Info.Types],
// and cannot do so: its entries are unconstructible outside go/types.
//
// The builders don't read that map, and a spec builds identically without it, which is what makes
// the whole approach workable.
//
// See also [§annotated-dependencies](../scanner/README.md#annotated-dependencies).
func attachAnnotatedDependencies(roots []*Package, onExportOnly func(ExportOnly)) {
	seen := make(map[string]bool, len(roots))

	// One buffer for the whole walk. The marker scan holds nothing else, so the largest source file in the graph
	// costs what the smallest one does.
	var buf [annotationChunk]byte

	var walk func(*Package)
	walk = func(pkg *Package) {
		if pkg == nil || seen[pkg.ID] {
			return
		}
		seen[pkg.ID] = true

		attachSource(pkg, buf[:], onExportOnly)

		for _, imp := range pkg.Imports {
			walk(imp)
		}
	}

	for _, root := range roots {
		walk(root)
	}
}

// attachSource reads one package's source back onto it, or says why it did not.
//
// The three refusals are deliberately distinguished rather than collapsed into "no source": each is announced with
// its own reason, and the scanner replays that reason at the point some builder actually wanted the declaration.
// "Nothing in it is annotated" is the ordinary case and by far the most common — it is the policy working, not a
// fault, and it is worth recording only because a lookup landing there later has no other way to explain itself.
func attachSource(pkg *Package, buf []byte, onExportOnly func(ExportOnly)) {
	if pkg.Types == nil || len(pkg.Syntax) > 0 {
		// Loaded in the ordinary way: the roots, and anything else go list handed over whole.
		return
	}
	if pkg.Fset == nil {
		// Positions parsed into a private FileSet would not resolve against the ones the scan already holds, which
		// is worse than not attaching at all.
		announceExportOnly(onExportOnly, pkg.PkgPath, "the load left it with no position information")

		return
	}

	if len(pkg.GoFiles) == 0 {
		announceExportOnly(onExportOnly, pkg.PkgPath, "its source is not on the filesystem")

		return
	}

	if !filesCarryMarker(pkg.GoFiles, buf) {
		announceExportOnly(onExportOnly, pkg.PkgPath, "nothing in its source is annotated, so it was not parsed")

		return
	}

	if !readBackSource(pkg, openOSFile) {
		announceExportOnly(onExportOnly, pkg.PkgPath, "its source could not be parsed")
	}
}

// announceExportOnly reports a dependency whose types were read but whose source was not.
//
// Every package is visited once, so there is no dedup to do here.
func announceExportOnly(onExportOnly func(ExportOnly), importPath, why string) {
	if onExportOnly == nil {
		return
	}

	onExportOnly(ExportOnly{Path: importPath, Reason: why})
}

// filesCarryMarker reports whether any of a package's files contains the annotation marker.
//
// The real filesystem, unconditionally: these paths came from `go list`, which only ever runs against it.
func filesCarryMarker(paths []string, buf []byte) bool {
	for _, path := range paths {
		if fileCarriesMarker(openOSFile, path, buf) {
			return true
		}
	}

	return false
}

func openOSFile(path string) (io.ReadCloser, error) { return os.Open(path) }
