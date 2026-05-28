// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"fmt"
	"go/ast"
	"go/types"
	"regexp"

	"github.com/go-openapi/codescan/internal/logger"
	"github.com/go-openapi/codescan/internal/parsers"
	"golang.org/x/tools/go/packages"
)

type TypeIndexOption func(*TypeIndex)

func WithExcludeDeps(excluded bool) TypeIndexOption {
	return func(a *TypeIndex) {
		a.excludeDeps = excluded
	}
}

func WithIncludeTags(included map[string]bool) TypeIndexOption {
	return func(a *TypeIndex) {
		a.includeTags = included
	}
}

func WithExcludeTags(excluded map[string]bool) TypeIndexOption {
	return func(a *TypeIndex) {
		a.excludeTags = excluded
	}
}

func WithIncludePkgs(included []string) TypeIndexOption {
	return func(a *TypeIndex) {
		a.includePkgs = included
	}
}

func WithExcludePkgs(excluded []string) TypeIndexOption {
	return func(a *TypeIndex) {
		a.excludePkgs = excluded
	}
}

func WithXNullableForPointers(enabled bool) TypeIndexOption {
	return func(a *TypeIndex) {
		a.setXNullableForPointers = enabled
	}
}

func WithRefAliases(enabled bool) TypeIndexOption {
	return func(a *TypeIndex) {
		a.refAliases = enabled
	}
}

func WithTransparentAliases(enabled bool) TypeIndexOption {
	return func(a *TypeIndex) {
		a.transparentAliases = enabled
	}
}

func WithDebug(enabled bool) TypeIndexOption {
	return func(a *TypeIndex) {
		a.debug = enabled
	}
}

type TypeIndex struct {
	AllPackages             map[string]*packages.Package
	Models                  map[*ast.Ident]*EntityDecl
	ExtraModels             map[*ast.Ident]*EntityDecl
	Meta                    []parsers.MetaSection
	Routes                  []parsers.ParsedPathContent
	Operations              []parsers.ParsedPathContent
	Parameters              []*EntityDecl
	Responses               []*EntityDecl
	excludeDeps             bool
	includeTags             map[string]bool
	excludeTags             map[string]bool
	includePkgs             []string
	excludePkgs             []string
	setXNullableForPointers bool
	refAliases              bool
	transparentAliases      bool
	debug                   bool
}

func NewTypeIndex(pkgs []*packages.Package, opts ...TypeIndexOption) (*TypeIndex, error) {
	ac := &TypeIndex{
		AllPackages: make(map[string]*packages.Package),
		Models:      make(map[*ast.Ident]*EntityDecl),
		ExtraModels: make(map[*ast.Ident]*EntityDecl),
	}
	for _, apply := range opts {
		apply(ac)
	}

	if err := ac.build(pkgs); err != nil {
		return nil, err
	}
	return ac, nil
}

func (a *TypeIndex) build(pkgs []*packages.Package) error {
	for _, pkg := range pkgs {
		if _, known := a.AllPackages[pkg.PkgPath]; known {
			continue
		}
		a.AllPackages[pkg.PkgPath] = pkg
		if err := a.processPackage(pkg); err != nil {
			return err
		}
		if err := a.walkImports(pkg); err != nil {
			return err
		}
	}

	return nil
}

func (a *TypeIndex) processPackage(pkg *packages.Package) error {
	if !shouldAcceptPkg(pkg.PkgPath, a.includePkgs, a.excludePkgs) {
		logger.DebugLogf(a.debug, "package %s is ignored due to rules", pkg.Name)
		return nil
	}

	for _, file := range pkg.Syntax {
		if err := a.processFile(pkg, file); err != nil {
			return err
		}
	}

	return nil
}

func (a *TypeIndex) processFile(pkg *packages.Package, file *ast.File) error {
	n, err := a.detectNodes(file)
	if err != nil {
		return err
	}

	if n&metaNode != 0 {
		a.Meta = append(a.Meta, parsers.MetaSection{Comments: file.Doc})
	}

	if n&operationNode != 0 {
		a.Operations = a.collectOperationPathAnnotations(file.Comments, a.Operations)
	}

	if n&routeNode != 0 {
		a.Routes = a.collectRoutePathAnnotations(file.Comments, a.Routes)
	}

	a.processFileDecls(pkg, file, n)

	return nil
}

func (a *TypeIndex) collectOperationPathAnnotations(comments []*ast.CommentGroup, dst []parsers.ParsedPathContent) []parsers.ParsedPathContent {
	for _, cmts := range comments {
		pp := parsers.ParseOperationPathAnnotation(cmts.List)
		if pp.Method == "" {
			continue
		}

		if !shouldAcceptTag(pp.Tags, a.includeTags, a.excludeTags) {
			logger.DebugLogf(a.debug, "operation %s %s is ignored due to tag rules", pp.Method, pp.Path)
			continue
		}
		dst = append(dst, pp)
	}

	return dst
}

func (a *TypeIndex) collectRoutePathAnnotations(comments []*ast.CommentGroup, dst []parsers.ParsedPathContent) []parsers.ParsedPathContent {
	for _, cmts := range comments {
		pp := parsers.ParseRoutePathAnnotation(cmts.List)
		if pp.Method == "" {
			continue
		}

		if !shouldAcceptTag(pp.Tags, a.includeTags, a.excludeTags) {
			logger.DebugLogf(a.debug, "operation %s %s is ignored due to tag rules", pp.Method, pp.Path)
			continue
		}
		dst = append(dst, pp)
	}

	return dst
}

func (a *TypeIndex) processFileDecls(pkg *packages.Package, file *ast.File, n node) {
	for _, dt := range file.Decls {
		switch fd := dt.(type) {
		case *ast.BadDecl:
			continue
		case *ast.FuncDecl:
			if fd.Body == nil {
				continue
			}
			for _, stmt := range fd.Body.List {
				if dstm, ok := stmt.(*ast.DeclStmt); ok {
					if gd, isGD := dstm.Decl.(*ast.GenDecl); isGD {
						a.processDecl(pkg, file, n, gd)
					}
				}
			}
		case *ast.GenDecl:
			a.processDecl(pkg, file, n, fd)
		}
	}
}

func (a *TypeIndex) processDecl(pkg *packages.Package, file *ast.File, n node, gd *ast.GenDecl) {
	for _, sp := range gd.Specs {
		switch ts := sp.(type) {
		case *ast.ValueSpec:
			logger.DebugLogf(a.debug, "saw value spec: %v", ts.Names)
			return
		case *ast.ImportSpec:
			logger.DebugLogf(a.debug, "saw import spec: %v", ts.Name)
			return
		case *ast.TypeSpec:
			def, ok := pkg.TypesInfo.Defs[ts.Name]
			if !ok {
				logger.DebugLogf(a.debug, "couldn't find type info for %s", ts.Name)
				continue
			}
			nt, isNamed := def.Type().(*types.Named)
			at, isAliased := def.Type().(*types.Alias)
			if !isNamed && !isAliased {
				logger.DebugLogf(a.debug, "%s is not a named or aliased type but a %T", ts.Name, def.Type())

				continue
			}

			comments := ts.Doc // type ( /* doc */ Foo struct{} )
			if comments == nil {
				comments = gd.Doc // /* doc */  type ( Foo struct{} )
			}

			decl := &EntityDecl{
				Comments: comments,
				Type:     nt,
				Alias:    at,
				Ident:    ts.Name,
				Spec:     ts,
				File:     file,
				Pkg:      pkg,
			}
			key := ts.Name
			switch {
			case n&modelNode != 0 && decl.HasModelAnnotation():
				a.Models[key] = decl
			case n&parametersNode != 0 && decl.HasParameterAnnotation():
				a.Parameters = append(a.Parameters, decl)
			case n&responseNode != 0 && decl.HasResponseAnnotation():
				a.Responses = append(a.Responses, decl)
			default:
				logger.DebugLogf(a.debug,
					"type %q skipped because it is not tagged as a model, a parameter or a response. %s",
					decl.Obj().Name(),
					"It may reenter the scope because it is a discovered dependency",
				)
			}
		}
	}
}

func (a *TypeIndex) walkImports(pkg *packages.Package) error {
	if a.excludeDeps {
		return nil
	}
	for _, v := range pkg.Imports {
		if _, known := a.AllPackages[v.PkgPath]; known {
			continue
		}

		a.AllPackages[v.PkgPath] = v
		if err := a.processPackage(v); err != nil {
			return err
		}
		if err := a.walkImports(v); err != nil {
			return err
		}
	}

	return nil
}

// detectNodes scans all comment groups in a file and returns a bitmask of
// detected swagger annotation types. Node types like route, operation, and
// meta accumulate freely across comment groups. Struct-level annotations
// (model, parameters, response) are mutually exclusive Within a single
// comment group — mixing them is an error.
func (a *TypeIndex) detectNodes(file *ast.File) (node, error) {
	var n node
	for _, comments := range file.Comments {
		var seenStruct string // tracks the struct annotation for this comment group
		for _, cline := range comments.List {
			if cline == nil {
				continue
			}
		}

		for _, cline := range comments.List {
			if cline == nil {
				continue
			}

			annotation, ok := parsers.ExtractAnnotation(cline.Text)
			if !ok {
				continue
			}

			switch annotation {
			case "route":
				n |= routeNode
			case "operation":
				n |= operationNode
			case "model": // annotation keyword matched from swagger comment.
				n |= modelNode
				if err := checkStructConflict(&seenStruct, annotation, cline.Text); err != nil {
					return 0, err
				}
			case "meta":
				n |= metaNode
			case "parameters":
				n |= parametersNode
				if err := checkStructConflict(&seenStruct, annotation, cline.Text); err != nil {
					return 0, err
				}
			case "response":
				n |= responseNode
				if err := checkStructConflict(&seenStruct, annotation, cline.Text); err != nil {
					return 0, err
				}
			case "strfmt", "name", "discriminated", "file", "enum", "default", "alias", "type":
				// Proposal for enhancement: perhaps collect these and pass along to avoid lookups later on
			case "allOf":
			case "ignore":
			default:
				return 0, fmt.Errorf("classifier: unknown swagger annotation %q: %w", annotation, ErrScanner)
			}
		}
	}

	return n, nil
}

func checkStructConflict(seenStruct *string, annotation string, text string) error {
	if *seenStruct != "" && *seenStruct != annotation {
		return fmt.Errorf("classifier: already annotated as %s, can't also be %q - %s: %w", *seenStruct, annotation, text, ErrScanner)
	}
	*seenStruct = annotation
	return nil
}

func shouldAcceptTag(tags []string, includeTags map[string]bool, excludeTags map[string]bool) bool {
	for _, tag := range tags {
		if len(includeTags) > 0 {
			if includeTags[tag] {
				return true
			}
		} else if len(excludeTags) > 0 {
			if excludeTags[tag] {
				return false
			}
		}
	}

	return len(includeTags) == 0
}

func shouldAcceptPkg(path string, includePkgs, excludePkgs []string) bool {
	if len(includePkgs) == 0 && len(excludePkgs) == 0 {
		return true
	}

	for _, pkgName := range includePkgs {
		matched, _ := regexp.MatchString(pkgName, path)
		if matched {
			return true
		}
	}

	for _, pkgName := range excludePkgs {
		matched, _ := regexp.MatchString(pkgName, path)
		if matched {
			return false
		}
	}

	return len(includePkgs) == 0
}
