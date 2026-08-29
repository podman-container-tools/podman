// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"go/ast"
	"strconv"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/godoclink"
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/scanner"
)

// godocResolver builds the doc-link resolver for the declaration currently being built.
//
// It maps a doc-link reference — the bracket content with any leading `*` stripped, e.g.
// "Order.CustName" or "inventory.Ledger" — to the referenced schema's fully-qualified definition
// key plus an exposed field-chain suffix.
//
// A reference resolves only when its leading segment(s) name a scanned model (same-package, or via
// a file import for a `pkg.Type` qualifier); a member segment then resolves to its exposed property
// name.
//
// References that name a non-model (a func registered as an operation, an unknown identifier, a
// non-struct field path) return ok=false, so the caller humanizes the leaf instead.
// Returns nil when there is no usable decl context.
func (s *Builder) godocResolver() godoclink.Resolver {
	decl := s.Decl
	if decl == nil || decl.File == nil {
		return nil
	}
	obj := decl.Obj()
	if obj == nil || obj.Pkg() == nil {
		return nil
	}
	currentPkg := obj.Pkg().Path()
	imports := fileImports(decl)

	return func(ref string) (godoclink.Resolution, bool) {
		parts := strings.Split(ref, ".")
		pkgPath, typeName, fields := currentPkg, parts[0], parts[1:]
		if len(parts) > 1 {
			if p, ok := imports[parts[0]]; ok {
				pkgPath, typeName, fields = p, parts[1], parts[2:]
			}
		}

		target, ok := s.Ctx.GetModel(pkgPath, typeName)
		if !ok {
			return godoclink.Resolution{}, false
		}
		suffix, ok := s.resolveFieldChain(target, fields)
		if !ok {
			return godoclink.Resolution{}, false
		}

		return godoclink.Resolution{DefKey: target.DefKey(), Suffix: suffix}, true
	}
}

// godocSelf describes the declaration being built so godoclink can recompose its leading
// godoc-convention self-name to its own exposed name.
//
// Nil when there is no usable decl.
func (s *Builder) godocSelf() *godoclink.SelfRef {
	decl := s.Decl
	if decl == nil || decl.Ident == nil || decl.Obj() == nil || decl.Obj().Pkg() == nil {
		return nil
	}
	_, goName := decl.Names()

	return &godoclink.SelfRef{Name: goName, DefKey: decl.DefKey()}
}

// resolveFieldChain maps a member chain on decl's struct to its exposed property suffix (e.g.
// [".customer_name"]).
//
// An empty chain is the bare-type case (suffix "").
// Only a single member level is resolved in this phase; deeper chains, non-struct targets, ignored
// / un-named fields return ok=false so the caller humanizes the leaf.
func (s *Builder) resolveFieldChain(decl *scanner.EntityDecl, fields []string) (string, bool) {
	if len(fields) == 0 {
		return "", true
	}
	if len(fields) > 1 {
		return "", false
	}

	st, ok := structAST(decl)
	if !ok {
		return "", false
	}
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name != fields[0] {
				continue
			}
			name, ignore, _, _, err := resolvers.ParseFieldTag(f, n.Name, s.Ctx.NameFromTags())
			if err != nil || ignore || name == "" {
				return "", false
			}

			return "." + name, true
		}
	}

	return "", false
}

// structAST returns decl's struct AST when it is a struct type declaration.
func structAST(decl *scanner.EntityDecl) (*ast.StructType, bool) {
	if decl.Spec == nil {
		return nil, false
	}
	st, ok := decl.Spec.Type.(*ast.StructType)

	return st, ok
}

// fileImports maps each usable import's local name to its package path for the file enclosing decl,
// so a `pkg.Type` doc-link can resolve `pkg` to a path.
//
// Blank, dot and unresolvable imports are skipped.
func fileImports(decl *scanner.EntityDecl) map[string]string {
	out := make(map[string]string)
	for _, imp := range decl.File.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			continue
		}

		name := ""
		switch {
		case imp.Name != nil:
			name = imp.Name.Name
		case decl.Pkg != nil:
			if p, ok := decl.Pkg.Imports[path]; ok {
				name = p.Name
			}
		}
		if name == "" || name == "_" || name == "." {
			continue
		}
		out[name] = path
	}

	return out
}
