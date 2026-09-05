// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package packages

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/go-openapi/codescan/internal/packages/list"
)

// Synthesizing a package from usage.
//
// When an import cannot be resolved to source — because nothing on the filesystem provides it, or because
// [WithStubbedStdlib] deliberately withholds it — the type-checker still needs something to resolve `pkg.Name`
// against.
//
// A package fabricated from the names actually selected through it is enough for codescan's name-keyed recognizers,
// which ask (package, type name) and never look at the type's shape.
//
// This is not enough for anything structural: a synthesized type has no fields to walk and no methods,
// so drilling into it, or asking whether it implements an interface, both fail.

// willStub reports whether an import path will be synthesized rather than loaded, for an import made from fromDir.
//
// The importing directory is part of the question, not context around it: the standard library's vendored packages
// resolve for an importer inside GOROOT/src and for no other, so the same path has two honest answers. Memoizing on
// the path alone gave whichever was asked first to everybody, which is how a resolvable import came to be announced
// as synthesized.
func (ld *loadState) willStub(path, fromDir string) bool {
	if path == "unsafe" {
		return false
	}
	key := stubKey(path, ld.res.UnderGoroot(fromDir))
	if known, ok := ld.stubbable[key]; ok {
		return known
	}
	if ld.hasExportData(path) {
		ld.stubbable[key] = false

		return false
	}

	_, _, resolved := ld.res.ResolveImportFrom(path, fromDir)
	ld.stubbable[key] = !resolved

	return !resolved
}

// stubKey distinguishes the two answers an import path can honestly have.
func stubKey(path string, fromGoroot bool) string {
	if fromGoroot {
		return "goroot\x00" + path
	}

	return path
}

// synthesizeFrom records the names a package selects through its unresolvable imports, so that the synthesized packages
// hold them before this package is type-checked against them.
//
// Names are collected per importing package, which is why this runs before each Check rather than once up front: the
// set of packages is discovered lazily, as imports are followed.
func (ld *loadState) synthesizeFrom(files []*ast.File, fromDir string) {
	for _, f := range files {
		aliases := ld.stubbedImports(f, fromDir)
		if len(aliases) == 0 {
			continue
		}

		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if path, ok := aliases[ident.Name]; ok {
				ld.addSynthesizedName(path, sel.Sel.Name)
			}

			return true
		})
	}
}

// stubbedImports maps the local name of each to-be-synthesized import onto its path.
func (ld *loadState) stubbedImports(f *ast.File, fromDir string) map[string]string {
	var aliases map[string]string
	for _, spec := range f.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil || !ld.willStub(path, fromDir) {
			continue
		}

		ld.reportSynthesized(path, spec.Path.Pos())

		name := pkgNameFor(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "_" || name == "." {
			continue // no qualified selector can name this import
		}

		if aliases == nil {
			aliases = make(map[string]string, len(f.Imports))
		}
		aliases[name] = path
	}

	return aliases
}

// reportSynthesized announces an import path the loader had to fabricate, once per path.
//
// It fires on the import rather than on first use, so that an import whose names are never selected — and which
// therefore contributes no fabricated type at all — is still reported.
func (ld *loadState) reportSynthesized(path string, pos token.Pos) {
	if ld.onSynthesize == nil || ld.reported[path] {
		return
	}
	ld.reported[path] = true

	ld.onSynthesize(Synthesized{
		Path:       path,
		Pos:        ld.fset.Position(pos),
		Deliberate: ld.stubStdlib && list.IsStdlibPath(path),
		Cgo:        path == cgoPseudoPackage,
	})
}

// cgoPseudoPackage is the import path of the pseudo-package a cgo file selects C declarations through.
const cgoPseudoPackage = "C"

// addSynthesizedName adds one opaque defined type to a synthesized package.
//
// Only exported names are worth fabricating: an unexported one could never be referenced from another package, so
// seeing it means the selector was something else.
//
// "C" is the exception, and it is noticeable: C has no notion of exportedness and its identifiers are conventionally
// lower case, so the rule above discards every single one — C.int, C.size_t, a struct tag.
//
// A cgo file would then parse, and any C type it used in a declaration would resolve to nothing.
// Keeping the C names lets a program that merely BUILDS with cgo be scanned imprecisely
// rather than not at all: a C type is opaque either way, but the Go declarations around it still resolve.
func (ld *loadState) addSynthesizedName(path, name string) {
	if !ast.IsExported(name) && path != cgoPseudoPackage {
		return
	}

	pkg := ld.stub(path)
	if pkg.Scope().Lookup(name) != nil {
		return
	}

	tn := types.NewTypeName(token.NoPos, pkg, name, nil)
	types.NewNamed(tn, types.NewStruct(nil, nil), nil)
	pkg.Scope().Insert(tn)
}
