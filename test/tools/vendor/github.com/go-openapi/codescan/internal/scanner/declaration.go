// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/go-openapi/codescan/internal/parsers"
	"golang.org/x/tools/go/packages"
)

// ParameterRef is a standalone `swagger:parameters` marker hosted by a func (or other non-struct
// declaration) rather than a struct definition.
//
// Per the disambiguation rule, such a marker is a *reference*: it wires existing shared parameters
// into an operation or path-item as `$ref`s — its first argument token is the target (an
// operation id or a `/path`) and the remaining tokens are shared-parameter names.
//
// The scanner only discovers and locates the marker; its target and names are parsed from Comments
// by the grammar (grammar.ParametersBlock) when the shared-parameters builder consumes it.
// §1b.
type ParameterRef struct {
	Comments *ast.CommentGroup
	File     *ast.File
	Pkg      *packages.Package
}

// EntityDecl is a type declaration the scanner classified, in two halves.
//
// The TYPE half — Type / Alias, and the Obj, ObjType, Name, Pos, PkgPath accessors built on them —
// is always available: it comes from the type-checker, and a package served from compiled export
// data carries it in full (positions included: time.Duration reports $GOROOT/src/time/time.go).
//
// The SYNTAX half — the comment group, the declaring identifier, the type spec, the enclosing file
// and the loaded package — comes from parsed source, which a package need not have. It is
// unexported, and reachable only through accessors that make its absence expressible: HasSource
// answers whether there is any, Comments and File answer nil when there is none, and TypeExpr /
// Imports / PkgImport / EnumSourcePkg return a second result the caller has to handle.
//
// Reach for it only for what genuinely needs source; a name, a position or a package path must come
// from the type half, or the declaration stops working the moment its source is not read. See
// [§declaration](./README.md#declaration).
type EntityDecl struct {
	Type  *types.Named
	Alias *types.Alias // added to supplement Named, after go1.22

	// The syntax half. Written as a unit at construction — either every one of these is set, or the
	// declaration has no source at all — which is what makes HasSource a single honest gate.
	comments *ast.CommentGroup
	ident    *ast.Ident
	spec     *ast.TypeSpec
	file     *ast.File
	pkg      *packages.Package

	hasModelAnnotation      bool
	hasResponseAnnotation   bool
	hasParameterAnnotation  bool
	modelOverrideSuppressed bool
}

// HasSource reports whether this declaration's source was parsed.
//
// False means the type half is all there is: the declaration comes from a package whose types were
// read from compiled export data with no syntax attached. Every syntax accessor then answers
// nothing, and a caller that cannot proceed without source must say so rather than guess from the
// type half — Underlying is not the written right-hand side, and a missing comment is not an absent
// annotation.
func (d *EntityDecl) HasSource() bool { return d.spec != nil }

// Comments returns the declaration's swagger annotations and godoc, or nil when it carries none.
//
// Nil is the ordinary answer for a declaration with no doc comment, so it is also the honest answer
// for one with no source: every consumer already reads nil as "no annotations here".
func (d *EntityDecl) Comments() *ast.CommentGroup { return d.comments }

// File returns the source file the declaration was read from, or nil when it was not read from one.
//
// Nil is a legitimate answer — see [resolvers.FindASTField], the only consumer, which yields no
// field for it.
func (d *EntityDecl) File() *ast.File { return d.file }

// TypeExpr returns the right-hand side of the declaration AS WRITTEN — the `time.Time` in
// `type Stamp time.Time`, the struct or interface literal in a struct or interface declaration.
//
// Reports false when there is no source. go/types keeps no record of the written form on a defined
// type, so there is nothing to fall back on: Underlying peels exactly the named layer the stdlib
// recognizers key on. A caller that only needs a plausible shape may fall back to it deliberately;
// one that needs the written form has to give up.
func (d *EntityDecl) TypeExpr() (ast.Expr, bool) {
	if d.spec == nil {
		return nil, false
	}

	return d.spec.Type, true
}

// Imports returns the import declarations of the file the type was declared in.
//
// Reports false when there is no source. Imports are how a `pkg.Type` doc-link resolves its
// qualifier to a package path, and only the file records the spelling: an import alias exists
// nowhere else.
func (d *EntityDecl) Imports() ([]*ast.ImportSpec, bool) {
	if d.file == nil {
		return nil, false
	}

	return d.file.Imports, true
}

// PkgImport returns the loaded package the declaring package imports under path.
//
// Reports false when there is no source, and when nothing under that path was loaded. It is the
// other half of Imports: the file supplies the path, this supplies the package's own name — which
// need not be the path's last segment, so an unaliased import cannot be spelled without it.
func (d *EntityDecl) PkgImport(path string) (*packages.Package, bool) {
	if d.pkg == nil {
		return nil, false
	}
	imported, ok := d.pkg.Imports[path]

	return imported, ok
}

// EnumSourcePkg returns the loaded package this declaration's source was read from, for the one
// consumer that needs the package rather than the declaration: collecting a swagger:enum's const
// values.
//
// Reports false when there is no source, and the enum then has no members — [ScanCtx.FindEnumValues]
// reads the const block from the package's own syntax, which is precisely what is missing.
func (d *EntityDecl) EnumSourcePkg() (*packages.Package, bool) {
	if d.pkg == nil {
		return nil, false
	}

	return d.pkg, true
}

// SuppressModelOverride drops this declaration's `swagger:model <name>` override so that Names /
// DefKey fall back to the Go type name.
//
// Used to resolve a same-package duplicate, where two distinct types in one package claim the same
// override name (a user error): the first keeps the name, later ones revert to their Go name.
func (d *EntityDecl) SuppressModelOverride() { d.modelOverrideSuppressed = true }

// ModelOverrideSuppressed reports whether SuppressModelOverride was set.
func (d *EntityDecl) ModelOverrideSuppressed() bool { return d.modelOverrideSuppressed }

// Obj returns the type name for the declaration defining the named type or alias t.
func (d *EntityDecl) Obj() *types.TypeName {
	if d.Type != nil {
		return d.Type.Obj()
	}
	if d.Alias != nil {
		return d.Alias.Obj()
	}

	panic("invalid EntityDecl: Type and Alias are both nil")
}

func (d *EntityDecl) ObjType() types.Type {
	if d.Type != nil {
		return d.Type
	}
	if d.Alias != nil {
		return d.Alias
	}

	panic("invalid EntityDecl: Type and Alias are both nil")
}

// Name returns the Go name of the declared type.
//
// It reads the type-checker's object rather than the declaring identifier, so it answers for a
// declaration whose source was never parsed.
func (d *EntityDecl) Name() string { return d.Obj().Name() }

// Pos returns the position of the declared type's name.
//
// Obj().Pos() is the position the type-checker recorded, which compiled export data carries too —
// so this is the position accessor to reach for, never Ident.Pos() / Spec.Pos(). For a package
// loaded from source the three are the same token: ast.TypeSpec.Pos() is its Name.Pos(), and that
// identifier is the key Defs maps to the object.
func (d *EntityDecl) Pos() token.Pos { return d.Obj().Pos() }

// PkgPath returns the import path of the package declaring the type, or "" for a package-less
// (universe) type.
func (d *EntityDecl) PkgPath() string {
	pkg := d.Obj().Pkg()
	if pkg == nil {
		return ""
	}

	return pkg.Path()
}

// IsAlias reports whether the declaration was written with alias syntax (`type X = Y`).
//
// The type-checker's answer to what the source spells `Spec.Assign.IsValid()`: since go1.22 an
// alias declaration defines a *types.Alias, so the shape is readable without the source.
func (d *EntityDecl) IsAlias() bool { return d.Alias != nil }

func (d *EntityDecl) Names() (name, goName string) {
	goName = d.Name()
	model, ok := parsers.ModelOverride(d.comments)
	if !ok {
		return goName, goName
	}

	d.hasModelAnnotation = true
	if model == "" || d.modelOverrideSuppressed {
		return goName, goName
	}

	return model, goName
}

// DefKey returns the fully-qualified, compiler-unique definition key for this declaration:
// "<pkgpath>/<name>", where <name> is the swagger:model override when present, else the Go type
// name (the first return of Names).
//
// This is the build-time key for the definitions map and for every "#/definitions/" $ref target, so
// two distinct Go types that share a short name can never collide before the spec.Builder's reduce
// stage shortens names back.
//
// §12.1).
//
// Universe / package-less types (no enclosing package) fall back to the bare name; in practice
// those are intercepted as stdlib specials before they ever reach a definition key.
func (d *EntityDecl) DefKey() string {
	name, _ := d.Names()
	if pkgPath := d.PkgPath(); pkgPath != "" {
		return pkgPath + "/" + name
	}

	return name
}

func (d *EntityDecl) HasModelAnnotation() bool {
	if d.hasModelAnnotation {
		return true
	}

	_, ok := parsers.ModelOverride(d.comments)
	if !ok {
		return false
	}

	d.hasModelAnnotation = true

	return true
}

func (d *EntityDecl) HasResponseAnnotation() bool {
	if d.hasResponseAnnotation {
		return true
	}

	_, ok := parsers.ResponseOverride(d.comments)
	if !ok {
		return false
	}

	d.hasResponseAnnotation = true

	return true
}

func (d *EntityDecl) HasParameterAnnotation() bool {
	if d.hasParameterAnnotation {
		return true
	}

	_, ok := parsers.ParametersOverride(d.comments)
	if !ok {
		return false
	}

	d.hasParameterAnnotation = true

	return true
}
