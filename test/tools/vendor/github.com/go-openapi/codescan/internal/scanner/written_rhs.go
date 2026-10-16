// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"go/ast"
	"go/types"
	"strconv"
)

// WrittenRHS returns the type this declaration was WRITTEN over — `Stamp` in `type StampResp Stamp`
// — as opposed to the fully peeled underlying that types.Named.Underlying yields.
//
// The distinction matters wherever a named layer carries meaning: a stdlib recognizer keys on
// `time.Time`, which peeling discards, and a `swagger:strfmt` may sit one declaration to the right
// of the one being built.
//
// go/types keeps no record of it on a defined type, so the answer comes from the right-hand side
// expression. It is resolved against the type-checker's own scopes rather than through
// types.Info.Types: a package scope is complete whether the package was checked from source or read
// from compiled export data, while the expression records exist only for a source type-check and
// cannot be reconstructed outside go/types.
//
// Returns (nil, false) when the declaration has no syntax, or when the right-hand side is a shape
// this resolution declines to guess at (a generic instantiation, a dot-imported name). A caller
// that only needs a plausible shape should fall back to Underlying.
func (d *EntityDecl) WrittenRHS() (types.Type, bool) {
	if d.spec == nil {
		return nil, false
	}

	// The checker's own record when the package was checked from source: it answers for every
	// expression shape, including the ones resolved below declines.
	if d.pkg != nil && d.pkg.TypesInfo != nil {
		if tv, ok := d.pkg.TypesInfo.Types[d.spec.Type]; ok && tv.Type != nil {
			return tv.Type, true
		}
	}

	return d.resolveTypeExpr(d.spec.Type)
}

// resolveTypeExpr resolves a type expression written in this declaration to the type it denotes,
// using only the scopes go/types exposes.
func (d *EntityDecl) resolveTypeExpr(expr ast.Expr) (types.Type, bool) {
	switch e := expr.(type) {
	case *ast.ParenExpr:
		return d.resolveTypeExpr(e.X)

	case *ast.Ident:
		// A package-level name, or a predeclared one (`type Count int`).
		if tpe, ok := lookupTypeName(d.Obj().Pkg(), e.Name); ok {
			return tpe, true
		}

		return lookupTypeName(nil, e.Name)

	case *ast.SelectorExpr:
		qualifier, ok := e.X.(*ast.Ident)
		if !ok {
			return nil, false
		}
		pkg, ok := d.importedPackage(qualifier.Name)
		if !ok {
			return nil, false
		}

		return lookupTypeName(pkg, e.Sel.Name)

	case *ast.IndexExpr, *ast.IndexListExpr:
		// A generic instantiation. Reconstructing it means instantiating the origin type, which is
		// the checker's job; declining is honest where guessing would not be.
		return nil, false

	default:
		// Every other right-hand side is a type literal — a struct, an interface, a slice, a map, a
		// pointer, a channel, a function. The type a literal denotes is by definition the declared
		// type's underlying, so there is nothing left to resolve.
		return d.ObjType().Underlying(), true
	}
}

// importedPackage resolves the package a file-local qualifier names.
//
// The qualifier is an import alias when the import declares one, and otherwise the imported
// package's own name — which need not be its last path segment, so the two halves have to be read
// together: the file's import declarations for the spelling, the type-checker's import list for the
// package.
func (d *EntityDecl) importedPackage(qualifier string) (*types.Package, bool) {
	self := d.Obj().Pkg()
	if self == nil {
		return nil, false
	}

	byPath := make(map[string]*types.Package, len(self.Imports()))
	for _, imported := range self.Imports() {
		byPath[imported.Path()] = imported
	}

	if d.file == nil {
		// No import declarations to read the spelling from: an unaliased import is still findable by
		// the package's own name, and an aliased one is not.
		return uniqueByName(self.Imports(), qualifier)
	}

	for _, spec := range d.file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		imported, known := byPath[path]

		switch {
		case spec.Name != nil:
			if spec.Name.Name == qualifier {
				return imported, known
			}
		case known && imported.Name() == qualifier:
			return imported, true
		}
	}

	return nil, false
}

// uniqueByName returns the one package in pkgs called name, or nothing when none or several are.
func uniqueByName(pkgs []*types.Package, name string) (*types.Package, bool) {
	var (
		found *types.Package
		count int
	)

	for _, pkg := range pkgs {
		if pkg.Name() != name {
			continue
		}
		found = pkg
		count++
	}

	if count != 1 {
		return nil, false
	}

	return found, true
}

// lookupTypeName returns the type named name in pkg's scope, or in the universe scope when pkg is
// nil.
func lookupTypeName(pkg *types.Package, name string) (types.Type, bool) {
	scope := types.Universe
	if pkg != nil {
		scope = pkg.Scope()
	}

	tn, ok := scope.Lookup(name).(*types.TypeName)
	if !ok {
		return nil, false
	}

	return tn.Type(), true
}
