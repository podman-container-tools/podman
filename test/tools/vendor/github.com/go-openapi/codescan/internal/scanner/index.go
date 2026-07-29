// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"regexp"

	"github.com/go-openapi/codescan/internal/parsers"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
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

// WithAfterDeclComments enables folding a declaration's inside-body leading comment (struct) or
// trailing comment (alias / non-struct type) into the decl's annotation source.
//
// See Options.AfterDeclComments.
func WithAfterDeclComments(enabled bool) TypeIndexOption {
	return func(a *TypeIndex) {
		a.afterDeclComments = enabled
	}
}

// WithOnDiagnostic wires the consumer's diagnostic sink so the index can surface scan-environment
// observations (e.g. a package or route omitted by the caller's own include/exclude rules) as
// informational Hints.
//
// The index is built before the ScanCtx exists, so it reports through the raw callback directly,
// exactly as detectDegradedLoad does.
func WithOnDiagnostic(cb func(grammar.Diagnostic)) TypeIndexOption {
	return func(a *TypeIndex) {
		a.onDiagnostic = cb
	}
}

type TypeIndex struct {
	AllPackages             map[string]*packages.Package
	Models                  map[*ast.Ident]*EntityDecl
	ExtraModels             map[*ast.Ident]*EntityDecl
	Meta                    []*ast.CommentGroup
	Routes                  []parsers.ParsedPathContent
	Operations              []parsers.ParsedPathContent
	Parameters              []*EntityDecl
	ParameterRefs           []*ParameterRef
	Responses               []*EntityDecl
	excludeDeps             bool
	includeTags             map[string]bool
	excludeTags             map[string]bool
	includePkgs             []string
	excludePkgs             []string
	setXNullableForPointers bool
	refAliases              bool
	transparentAliases      bool
	afterDeclComments       bool
	onDiagnostic            func(grammar.Diagnostic)

	// enrichedFields guards the Phase-B AfterDeclComments field rewrite (append Field.Comment onto
	// Field.Doc) so a given field is enriched at most once even if its struct is visited more than
	// once.
	//
	// This is the only place AfterDeclComments mutates the shared AST.
	enrichedFields map[*ast.Field]struct{}
}

func NewTypeIndex(pkgs []*packages.Package, opts ...TypeIndexOption) (*TypeIndex, error) {
	ac := &TypeIndex{
		AllPackages:    make(map[string]*packages.Package),
		Models:         make(map[*ast.Ident]*EntityDecl),
		ExtraModels:    make(map[*ast.Ident]*EntityDecl),
		enrichedFields: make(map[*ast.Field]struct{}),
	}
	for _, apply := range opts {
		apply(ac)
	}

	if err := ac.build(pkgs); err != nil {
		return nil, err
	}
	return ac, nil
}

// emit delivers d to the consumer's diagnostic sink when one is wired.
//
// No-op otherwise.
// The index is built before the ScanCtx exists, so it reports through the raw callback (no dedup),
// exactly as detectNodes-level and detectDegradedLoad observations do.
func (a *TypeIndex) emit(d grammar.Diagnostic) {
	if a.onDiagnostic == nil {
		return
	}
	a.onDiagnostic(d)
}

// emitHintf delivers an informational Hint with no source position (the index observes
// whole-package / whole-route omissions, not a single token).
func (a *TypeIndex) emitHintf(code grammar.Code, format string, args ...any) {
	a.emit(grammar.Hintf(token.Position{}, code, format, args...))
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
		a.emitHintf(grammar.CodeIgnoredByRules,
			"package %q is omitted by the include/exclude package rules", pkg.PkgPath)
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
		a.Meta = append(a.Meta, file.Doc)
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
			a.emitHintf(grammar.CodeIgnoredByTag,
				"operation %s %s is omitted by the include/exclude tag rules", pp.Method, pp.Path)
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
			a.emitHintf(grammar.CodeIgnoredByTag,
				"route %s %s is omitted by the include/exclude tag rules", pp.Method, pp.Path)
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
			// A `swagger:parameters` marker on a func is a reference (it wires shared parameters into an
			// operation / path-item as $refs), never a definition — definitions live on struct types.
			if n&parametersNode != 0 {
				a.collectParameterRef(pkg, file, fd.Doc)
			}
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
			return
		case *ast.ImportSpec:
			return
		case *ast.TypeSpec:
			def, ok := pkg.TypesInfo.Defs[ts.Name]
			if !ok {
				continue
			}
			nt, isNamed := def.Type().(*types.Named)
			at, isAliased := def.Type().(*types.Alias)
			if !isNamed && !isAliased {
				continue
			}

			comments := ts.Doc // type ( /* doc */ Foo struct{} )
			if comments == nil {
				comments = gd.Doc // /* doc */  type ( Foo struct{} )
			}
			// AfterDeclComments (opt-in): also read the swagger annotations that live inside the declaration
			// (a struct's leading body comment) or inlined after it (an alias / non-struct type's trailing
			// comment), so the godoc above stays clean.
			// Folded into a fresh comment group — ts.Doc is never mutated, so this is idempotent.
			if a.afterDeclComments {
				comments = mergeCommentGroups(comments, afterDeclSource(file, ts))
				// Phase B: fold each struct field's trailing comment into its Doc (the field-level inlined
				// form, e.g. `B string // swagger:strfmt date`).
				// Runs AFTER afterDeclSource so leadingBodyComments still sees the original field Docs for its
				// exclusion set.
				a.enrichStructFields(ts)
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
			}
		}
	}
}

// afterDeclSource returns the comment group carrying swagger annotations that
// lives inside / after a type declaration, when AfterDeclComments is set:
//   - struct type: the leading comment groups at the top of the body (before the
//     first field, excluding any field's own Doc), collected in source order;
//   - alias / non-struct type: the trailing line comment (TypeSpec.Comment),
//     e.g. `type X = Y // swagger:model apiType`.
//
// Returns nil when there is nothing to fold.
func afterDeclSource(file *ast.File, ts *ast.TypeSpec) *ast.CommentGroup {
	if st, ok := ts.Type.(*ast.StructType); ok {
		return leadingBodyComments(file, st)
	}
	return ts.Comment
}

// leadingBodyComments collects every comment group positioned at the top of a struct body — after
// the opening brace and before the first field — that is not itself a field's Doc, in source
// order.
//
// Excluding field Docs keeps an adjacent `// swagger:allOf` above the first field attached to that
// field rather than stolen as a type-level annotation.
// Returns nil when there are none.
func leadingBodyComments(file *ast.File, st *ast.StructType) *ast.CommentGroup {
	fields := st.Fields
	if fields == nil {
		return nil
	}
	limit := fields.Closing // empty struct: up to the closing brace
	if len(fields.List) > 0 {
		limit = fields.List[0].Pos()
	}
	docs := make(map[*ast.CommentGroup]struct{}, len(fields.List))
	for _, f := range fields.List {
		if f.Doc != nil {
			docs[f.Doc] = struct{}{}
		}
	}
	var collected []*ast.Comment
	for _, cg := range file.Comments {
		if cg.Pos() <= fields.Opening || cg.Pos() >= limit {
			continue
		}
		if _, isFieldDoc := docs[cg]; isFieldDoc {
			continue
		}
		collected = append(collected, cg.List...)
	}
	if len(collected) == 0 {
		return nil
	}
	return &ast.CommentGroup{List: collected}
}

// enrichStructFields folds each struct field's trailing line comment (Field.Comment) into its
// Field.Doc — the field-level inlined form of AfterDeclComments (`B string // swagger:strfmt
// date`).
//
// This mutates the shared AST (the builders read Field.Doc directly), so the enrichedFields guard
// ensures each field is rewritten at most once.
// Positions stay ascending (Doc above < trailing comment), so the grammar parses the merged group
// unchanged.
//
// No-op for non-struct types and fields without a trailing comment.
func (a *TypeIndex) enrichStructFields(ts *ast.TypeSpec) {
	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return
	}
	for _, f := range st.Fields.List {
		if f.Comment == nil || len(f.Comment.List) == 0 {
			continue
		}
		if _, done := a.enrichedFields[f]; done {
			continue
		}
		f.Doc = mergeCommentGroups(f.Doc, f.Comment)
		a.enrichedFields[f] = struct{}{}
	}
}

// mergeCommentGroups returns a comment group whose List is above ++ extra in source order — above
// is the doc ABOVE the decl, extra lives inside/below it so positions stay ascending and the
// grammar reconstructs a clean blank-line gap.
//
// Returns the non-nil one when the other is nil (nil only when both are).
// The input groups are never mutated.
func mergeCommentGroups(above, extra *ast.CommentGroup) *ast.CommentGroup {
	switch {
	case extra == nil || len(extra.List) == 0:
		return above
	case above == nil || len(above.List) == 0:
		return extra
	default:
		merged := make([]*ast.Comment, 0, len(above.List)+len(extra.List))
		merged = append(merged, above.List...)
		merged = append(merged, extra.List...)
		return &ast.CommentGroup{List: merged}
	}
}

// collectParameterRef records a standalone `swagger:parameters` reference marker found on a func's
// doc comment.
//
// The marker's argument tokens are not parsed here — the grammar does that when a builder
// consumes the ParameterRef; the scanner only classifies the comment group as carrying a reference.
// No-op when doc carries no `swagger:parameters` marker.
func (a *TypeIndex) collectParameterRef(pkg *packages.Package, file *ast.File, doc *ast.CommentGroup) {
	if doc == nil {
		return
	}
	if _, ok := parsers.ParametersOverride(doc); !ok {
		return
	}
	a.ParameterRefs = append(a.ParameterRefs, &ParameterRef{
		Comments: doc,
		File:     file,
		Pkg:      pkg,
	})
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

// detectNodes scans all comment groups in a file and returns a bitmask of detected swagger
// annotation kinds.
//
// # Details
//
// See [§classifier](./README.md#classifier) — bitmask semantics, struct-annotation exclusivity
// rule, and the recognised-but-bitless field-decoration tokens.
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
				a.warnMalformedStructName(annotation, cline.Text)
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
				a.warnMalformedStructName(annotation, cline.Text)
				if err := checkStructConflict(&seenStruct, annotation, cline.Text); err != nil {
					return 0, err
				}
			case "strfmt", "name", "discriminated", "file", "enum", "default", "alias", "type", "additionalProperties", "patternProperties", "title", "description":
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

// warnMalformedStructName emits a Warning diagnostic when a single-name struct marker
// (swagger:model / swagger:response) on line carries a name that is not a plain identifier — e.g.
// a package-qualified "utils.Error" (go-swagger#874).
//
// Such names are JSON labels, not Go-qualified identifiers; the strict override matcher rejects
// them and the marker is ignored.
// The diagnostic gives the author a clue rather than silently dropping it.
//
// The type's package is resolved automatically, so a plain name suffices regardless of which
// package the type lives in.
func (a *TypeIndex) warnMalformedStructName(annotation, line string) {
	switch annotation {
	case "model":
		if bad, ok := parsers.MalformedModelName(line); ok {
			a.emit(grammar.Warnf(token.Position{}, grammar.CodeInvalidAnnotation,
				"swagger:model name %q is not a plain identifier "+
					"(definition names are JSON labels, not Go-qualified); annotation ignored", bad))
		}
	case "response":
		if bad, ok := parsers.MalformedResponseName(line); ok {
			a.emit(grammar.Warnf(token.Position{}, grammar.CodeInvalidAnnotation,
				"swagger:response name %q is not a plain identifier "+
					"(response names are JSON labels, not Go-qualified); annotation ignored", bad))
		}
	}
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
