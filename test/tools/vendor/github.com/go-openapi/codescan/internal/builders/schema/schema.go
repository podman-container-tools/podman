// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/go-openapi/swag/mangling"

	"github.com/go-openapi/codescan/internal/builders/common"
	"github.com/go-openapi/codescan/internal/builders/handlers"
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// Builder knows how to build a spec schema from go source.
//
// # Details
//
// See the maintainer notes in README.md (sibling file).
type Builder struct {
	options
	*common.Builder

	GoName         string
	Name           string
	annotated      bool
	discovered     []*scanner.EntityDecl
	methodMangler  mangling.NameMangler // see [§method-mangler](./README.md#method-mangler).
	skipExtensions bool
	emitXGoType    bool // stamp x-go-type on definitions; see [§traceability](./README.md#traceability).

	// Embed-recursion depth for ambiguous-embed diagnostics; see
	// [§embed-depth](./README.md#embed-depth).
	embedDepth int

	// embedInherited carries a `required:` annotation on an embedded field down to the properties it
	// promotes (go-swagger#2701).
	//
	// Set with save/restore around the embed recursion in scanEmbeddedFields.
	// The mechanism is shared with the parameters and responses builders via common.EmbedInheritance;
	// the schema builder consumes only Required (it has no `in:` location concept).
	embedInherited common.EmbedInheritance
}

// NewBuilder constructs an initialized [Builder].
func NewBuilder(ctx *scanner.ScanCtx, decl *scanner.EntityDecl) *Builder {
	return &Builder{
		Builder:        common.New(ctx, decl),
		methodMangler:  mangling.NewNameMangler(),
		skipExtensions: ctx.SkipExtensions(),
		emitXGoType:    ctx.EmitXGoType(),
	}
}

// Build a schema spec.
//
// The output is either stored in the passed definitions map (when used with [WithDefinitions]), or
// in the target renderer (when used with [WithType]).
func (s *Builder) Build(opts ...Option) error {
	s.options = optionsWithDefaults(opts) // set options for this build
	if s.definitions == nil && s.inputType == nil || s.definitions != nil && s.inputType != nil {
		panic("dev error: Build must be called with either a map of definitions or a single type")
	}

	if s.definitions != nil {
		// top-level declarations
		s.inferNames()

		// Key the definitions map by the fully-qualified identity key (pkgpath/name), not the bare
		// s.Name: distinct Go types that share a short name must not collide here. s.Name stays the leaf
		// so annotateSchema's x-go-name / x-go-package emission is unchanged.
		// The reduce stage shortens keys back afterwards.
		//
		// §9.1/§12.1.
		defKey := s.Decl.DefKey()

		schema := s.definitions[defKey] // if not named, empty schema
		err := s.buildFromDecl(&schema)
		if err != nil {
			return err
		}

		// Decl-level `swagger:additionalProperties` rides on top of the type-derived schema (lowest
		// priority; object-only).
		// Applied after the Go type is resolved so it can complement a struct, override a map's element
		// schema, or warn-and-drop on a non-object.
		//
		// See classifierAdditionalProperties.
		s.classifierAdditionalProperties(&schema, s.Ctx.PosOf(s.Decl.Ident.Pos()))
		s.classifierPatternProperties(&schema, s.Ctx.PosOf(s.Decl.Ident.Pos()))

		// The decl-comment block is dispatched before the Go type is resolved onto the schema (see
		// buildFromDecl), so the inline checkShape ran against an empty type.
		// Re-gate now that the type is known: strip validations illegal for the resolved type and warn.
		// See [§decl-shape-recheck](./README.md#decl-shape-recheck).
		handlers.RecheckSchemaShape(&schema, s.Ctx.PosOf(s.Decl.Spec.Pos()), s.RecordDiagnostic)

		s.definitions[defKey] = schema

		return nil
	}

	// build from parameter/response schema
	if err := s.buildFromType(s.inputType, s.target); err != nil {
		return err
	}
	if s.simpleSchema {
		s.validateSimpleSchemaOutcome()
	}
	return nil
}

func (s *Builder) SetDiscovered(discovered []*scanner.EntityDecl) {
	s.discovered = discovered
}

// declPos returns the position of the declaration being built, used as a coarse fallback for
// diagnostics raised deep in the type dispatch where no finer AST node is in scope.
//
// Returns the zero Position when unavailable.
func (s *Builder) declPos() token.Position {
	if s.Decl == nil || s.Decl.Ident == nil {
		return token.Position{}
	}

	return s.Ctx.PosOf(s.Decl.Ident.Pos())
}

// warnUnsupportedGoType records a [grammar.CodeUnsupportedGoType] Warning that tpe could not be
// translated to a Swagger 2.0 construct and was dropped from the spec. where names the dispatch
// site (function / switch arm) for triage. tpe is any (a types.Type, *types.TypeName,
// ifaces.Objecter, …); the message renders its dynamic type and value.
func (s *Builder) warnUnsupportedGoType(where string, tpe any) {
	s.RecordDiagnostic(grammar.Warnf(
		s.declPos(),
		grammar.CodeUnsupportedGoType,
		"%s: unsupported Go type %[2]T (%[2]v); skipping", where, tpe,
	))
}

// buildFromDecl emits the schema for a top-level type declaration (named or alias).
//
// # Details
//
// See [§build-entry](./README.md#build-entry), [§special-types](./README.md#special-types) and
// [§user-overrides](./README.md#user-overrides).
func (s *Builder) buildFromDecl(schema *oaispec.Schema) error {
	if s.applyDeclCommentBlock(schema) {
		// swagger:ignore short-circuit.
		return nil
	}
	defer s.annotateSchema(schema)()

	// Intercept stdlib specials before kind-dispatch (catches stdlib decls pulled in via the discovery
	// chain).
	// See [§special-types](./README.md#special-types).
	ps := NewTypable(schema, 0, s.skipExtensions)
	if applyStdlibSpecials(s.Decl.Obj(), ps, s.skipExtensions) {
		return nil
	}

	// Decl-site swagger:type override wins over type-driven default.
	// See [§user-overrides](./README.md#user-overrides) for the (handled, recurse) contract and the
	// Underlying() fallback rationale.
	if handled, recurse := s.classifierNamedTypeOverride(s.Decl.Comments, ps, s.Decl.ObjType(), s.Ctx.PosOf(s.Decl.Ident.Pos())); handled {
		if !recurse {
			return nil
		}
		if named, ok := s.Decl.ObjType().(*types.Named); ok {
			return s.buildFromType(named.Underlying(), ps)
		}
	}

	switch tpe := s.Decl.ObjType().(type) {
	case *types.Named:
		if s.guardDecl(tpe) {
			return nil
		}
		// F1/F4: a swagger:model type carrying a swagger:strfmt or swagger:enum override publishes the
		// FULL override schema on its own definition, not the bare-underlying orphan ({type:string} with
		// the format dropped / no enum values).
		//
		// The override classifiers — the same ones the field site uses — inline that schema onto the
		// definition target here; field sites then $ref the definition (see buildNamedType's
		// swagger:model gate). swagger:type is already applied above (decl-site
		// classifierNamedTypeOverride).
		defTgt := NewTypable(schema, 0, s.skipExtensions)
		switch ut := tpe.Underlying().(type) {
		case *types.Struct:
			if s.classifierNamedStructStrfmt(s.Decl.Comments, defTgt) {
				return nil
			}
		case *types.Basic:
			if s.classifierNamedBasic(s.Decl.Comments, s.Decl.Pkg, ut, defTgt, tpe.Obj().Name()) {
				return nil
			}
		}
		ti := s.Decl.Pkg.TypesInfo.Types[s.Decl.Spec.Type]
		resolvers.MustBeAType(ti) // invariant
		return s.buildFromType(ti.Type, NewTypable(schema, 0, s.skipExtensions))
	case *types.Alias:
		if s.guardDecl(tpe) {
			return nil
		}
		return s.buildDeclAlias(tpe, NewTypable(schema, 0, s.skipExtensions))
	default:
		s.warnUnsupportedGoType("buildFromDecl", tpe)
		return nil
	}
}

// buildDeclAlias emits the schema for a top-level alias declaration.
//
// Three modes — Dissolve / $ref / Expand — selected by TransparentAliases × RefAliases.
//
// # Details
//
// See [§aliases](./README.md#aliases) and [§discovery](./README.md#discovery).
func (s *Builder) buildDeclAlias(tpe *types.Alias, target ifaces.SwaggerTypable) error {
	resolvers.MustHaveRightHandSide(tpe)

	// `swagger:strfmt` user-override on the alias decl.
	// Without this check, an alias decl decorated with `// swagger:strfmt <format>` would fall through
	// to the recognizer (`type X = any` → empty body) or to the underlying primitive (`type X =
	// int64` → `{integer, int64}`), silently dropping the user's annotation.
	//
	// Honouring it here mirrors `classifierNamedTypeOverride`'s decl-entry treatment of
	// `swagger:type`, and matches the strfmt-first cascade order from `classifierNamedBasic`.
	//
	// `swagger:strfmt` on a Named decl is unaffected — that path is covered by
	// `classifierNamedStructStrfmt` (struct underlying) and `classifierNamedBasic` (primitive
	// underlying), both fired from `buildNamedType` after the underlying-kind switch.
	if name, ok := s.findAnnotationArg(s.Decl.Comments, grammar.AnnStrfmt); ok {
		target.Typed("string", name)
		return nil
	}

	rhs := tpe.Rhs()

	// Dissolve: no LHS definition, build directly from the RHS.
	if s.Ctx.TransparentAliases() {
		return s.buildFromType(rhs, target)
	}

	// Register the LHS so the spec's definitions section includes it even when not
	// swagger:model-annotated. s.Decl IS the alias's LHS decl by construction (see
	// [§discovery](./README.md#discovery) for why we skip the GetModel round-trip here).
	s.Ctx.AddDiscoveredModel(s.Decl)
	s.AppendPostDecl(s.Decl)

	// Expand: LHS gets a structural definition mirroring the underlying. tpe.Rhs() peels one alias
	// layer; tpe.Underlying() peels through aliases AND named types to the structural form.
	if !s.Ctx.RefAliases() {
		// Consult the stdlib recognizers before walking Underlying().
		//
		// Without this, aliases of stdlib-special types (time.Time, error, json.RawMessage, any) lose
		// their canonical shape: the structural walk produces {type:object} for time.Time and
		// {type:array, items:{integer,uint8}} for json.RawMessage instead of {type:string,
		// format:date-time} and the open "any JSON" idiom respectively.
		//
		// The TransparentAliases path at line 156 already gets this right via buildFromType(rhs); Expand
		// needs the same recognizer call before its Underlying fallthrough.
		if obj := rhsTypeName(rhs); obj != nil &&
			applyStdlibSpecials(obj, target, s.skipExtensions) {
			return nil
		}
		return s.buildFromType(tpe.Underlying(), target)
	}

	// $ref to the RHS's target.
	switch rtpe := rhs.(type) {
	case *types.Named:
		ro := rtpe.Obj()
		// Consult the stdlib recognizers first.
		// When the RHS is one of time.Time / error / json.RawMessage / any, emit the canonical inline
		// shape on the alias's OWN definition rather than chaining a $ref to a separately-built
		// definition.
		//
		// This matches what the sibling *types.Alias branch already does, what TransparentAliases mode
		// already does for stdlib decls, and eliminates the Q30 noise from stdlib godocs landing on the
		// chain targets (Time / RawMessage) — those targets no longer exist in the spec when the alias
		// inlines the canonical shape directly.
		//
		// For predeclared `error`, this is also the only safe path: it has no package, so the GetModel
		// lookup below would nil-panic on Pkg().Path().
		// User-defined named types (non-stdlib) fall through to the GetModel + MakeRef chain as before.
		if applyStdlibSpecials(ro, target, s.skipExtensions) {
			return nil
		}
		if ro.Pkg() == nil {
			// Not a recognised special AND no package — predeclared but unsupported; degrade to
			// missingSource so the build fails cleanly.
			return missingSource(rtpe)
		}
		rdecl, found := s.Ctx.GetModel(ro.Pkg().Path(), ro.Name())
		if !found {
			return missingSource(rtpe)
		}

		return s.MakeRef(rdecl, target)
	case *types.Alias:
		ro := rtpe.Obj()
		if resolvers.UnsupportedBuiltin(rtpe) {
			s.warnUnsupportedGoType("buildDeclAlias", rtpe)

			return nil
		}

		if applyStdlibSpecials(ro, target, s.skipExtensions) {
			return nil
		}

		resolvers.MustNotBeABuiltinType(ro)

		rdecl, found := s.Ctx.GetModel(ro.Pkg().Path(), ro.Name())
		if !found {
			return missingSource(rtpe)
		}

		return s.MakeRef(rdecl, target)
	default: // alias to anonymous type
		return s.buildFromType(rhs, target)
	}
}

// buildFromType emits the schema for an arbitrary Go type into target.
//
// # Details
//
// See [§dispatch-table](./README.md#dispatch-table) for the named-type pivot;
// [§textmarshal-order](./README.md#textmarshal-order) for the TextMarshaler shortcut.
func (s *Builder) buildFromType(tpe types.Type, target ifaces.SwaggerTypable) error {
	// Aliases first — honour RefAliases / TransparentAliases before any content-based shortcut (a
	// TextMarshaler-implementing alias otherwise gets inlined and loses its alias indirection).
	if titpe, ok := tpe.(*types.Alias); ok {
		return s.buildAlias(titpe, target)
	}

	// TextMarshaler shortcut: only when reachable to a *types.Named by peeling pointers.
	// Anonymous structs that merely satisfy TextMarshaler via embedded-method promotion don't qualify
	// — they need the structural walk to preserve body / allOf shape.
	if hasNamedCore(tpe) && resolvers.IsTextMarshaler(tpe) {
		return s.buildFromTextMarshal(tpe, target)
	}

	switch titpe := tpe.(type) {
	case *types.Basic:
		if resolvers.UnsupportedBuiltinType(titpe) {
			s.warnUnsupportedGoType("buildFromType", tpe)
			return nil
		}
		return resolvers.SwaggerSchemaForType(titpe.String(), target)
	case *types.Pointer:
		return s.buildFromType(titpe.Elem(), target)
	case *types.Struct:
		return s.buildFromStruct(s.Decl, titpe, target.Schema(), make(map[string]propOwner))
	case *types.Interface:
		return s.buildFromInterface(s.Decl, titpe, target.Schema(), make(map[string]propOwner))
	case *types.Slice:
		defer s.descend("items")()
		return s.buildFromType(titpe.Elem(), target.Items())
	case *types.Array:
		defer s.descend("items")()
		return s.buildFromType(titpe.Elem(), target.Items())
	case *types.Map:
		return s.buildFromMap(titpe, target)
	case *types.Named:
		return s.buildNamedType(titpe, target)
	default:
		// Unknown kind (TypeParam, Chan, Signature, Union, future additions).
		// Warn-and-skip — panicking would degrade UX on user code we can't inspect.
		s.warnUnsupportedGoType("buildFromType", tpe)
		return nil
	}
}

// buildAlias emits a schema for an alias reached as a field/element type (not a top-level decl).
//
// TransparentAliases dissolves the alias indirection; otherwise emit a $ref.
//
// # Details
//
// See [§aliases](./README.md#aliases).
func (s *Builder) buildAlias(tpe *types.Alias, target ifaces.SwaggerTypable) error {
	if resolvers.UnsupportedBuiltinType(tpe) {
		s.warnUnsupportedGoType("buildAlias", tpe)
		return nil
	}

	o := tpe.Obj()
	if applyStdlibSpecials(o, target, s.skipExtensions) {
		return nil
	}
	resolvers.MustNotBeABuiltinType(o)

	if s.Ctx.TransparentAliases() {
		return s.buildFromType(tpe.Rhs(), target)
	}

	decl, ok := s.Ctx.GetModel(o.Pkg().Path(), o.Name())
	if !ok {
		return fmt.Errorf("can't find source file for aliased type: %v: %w", tpe, ErrSchema)
	}
	// Unannotated alias at a use site: dissolve to the unaliased target.
	// The annotation is the user's opt-in to exposing the alias as a first-class spec entity; without
	// it the aliasing is a Go implementation detail and the spec carries the underlying name.
	//
	// Matches the dissolve already performed under TransparentAliases at line 313, but applied
	// per-decl based on intent instead of per-mode.
	// See [§aliases](./README.md#aliases).
	if !decl.HasModelAnnotation() {
		return s.buildFromType(tpe.Rhs(), target)
	}
	return s.MakeRef(decl, target)
}

// buildNamedType emits the schema for a *types.Named reached as a field/element/embed type.
//
// # Details
//
// See [§dispatch-table](./README.md#dispatch-table) and
// [§dissolve-named](./README.md#dissolve-named).
func (s *Builder) buildNamedType(titpe *types.Named, target ifaces.SwaggerTypable) error {
	if resolvers.UnsupportedBuiltin(titpe) {
		s.warnUnsupportedGoType("buildNamedType", titpe)

		return nil
	}

	tio := titpe.Obj()
	if applyStdlibSpecials(tio, target, s.skipExtensions) {
		return nil
	}

	// PkgForType-miss catches types we can't anchor to a scanned package (predeclared `comparable`,
	// generic type params, compiler-internal shapes).
	//
	// Complementary to the UnsupportedBuiltin guard above; also yields `pkg` for the Basic classifier
	// below.
	pkg, found := s.Ctx.PkgForType(titpe)
	if !found {
		return nil
	}

	var cmt *ast.CommentGroup
	var isModel bool
	if decl, ok := s.Ctx.DeclForType(titpe); ok && decl != nil {
		cmt = decl.Comments
		isModel = decl.HasModelAnnotation()
	}

	// refModel: reference this type by $ref instead of inlining the override.
	// A swagger:model type carrying an override (strfmt/type/enum) publishes its override schema on
	// its OWN definition (applied at buildFromDecl) and is referenced here by $ref — the inline
	// override classifiers below are skipped (F1/F2/F4).
	//
	// BUT only in full-schema mode: under SimpleSchema (non-body params / response headers) a $ref is
	// illegal in OAS v2, so the override must still inline even for a swagger:model type.
	// Without swagger:model an override type inlines everywhere, as before.
	refModel := isModel && !s.simpleSchema

	if !refModel {
		if handled, recurse := s.classifierNamedTypeOverride(cmt, target, titpe, s.Ctx.PosOf(tio.Pos())); handled {
			if recurse {
				return s.buildFromType(titpe.Underlying(), target)
			}
			return nil
		}
	}

	// Dissolve when there's no source-level TypeSpec to $ref:
	//   - outer decl is alias-syntax (TransparentAliases plumbing);
	//   - generic instantiation (Underlying carries substituted args).
	//
	// See [§dissolve-named](./README.md#dissolve-named).
	if s.Decl.Spec.Assign.IsValid() || (titpe.TypeArgs() != nil && titpe.TypeArgs().Len() > 0) {
		return s.buildFromType(titpe.Underlying(), target)
	}

	// Underlying-shape table.
	// See [§dispatch-table](./README.md#dispatch-table).
	switch utitpe := titpe.Underlying().(type) {
	case *types.Struct:
		if !refModel && s.classifierNamedStructStrfmt(cmt, target) {
			return nil
		}
		return s.resolveRefOr(tio, target, nil)

	case *types.Interface:
		return s.resolveRefOrErr(tio, target, utitpe)

	case *types.Basic:
		if resolvers.UnsupportedBuiltinType(utitpe) {
			s.warnUnsupportedGoType("buildNamedType", tio)
			return nil
		}
		if !refModel && s.classifierNamedBasic(cmt, pkg, utitpe, target, tio.Name()) {
			return nil
		}
		return s.resolveRefOr(tio, target, func() error {
			return resolvers.SwaggerSchemaForType(utitpe.String(), target)
		})

	case *types.Array:
		return s.buildNamedArrayLike(tio, cmt, utitpe.Elem(), target, false, refModel)
	case *types.Slice:
		return s.buildNamedArrayLike(tio, cmt, utitpe.Elem(), target, true, refModel)

	case *types.Map:
		return s.resolveRefOr(tio, target, nil)

	default:
		s.warnUnsupportedGoType("buildNamedType", utitpe)
		return nil
	}
}

// buildNamedArrayLike is the unified Array/Slice arm. forSlice toggles the slice-only
// "bsonobjectid" special case in classifierNamedArrayLike. isModel skips the inline override
// classifier so a swagger:model array/slice type is referenced by $ref (its override schema lives
// on its own definition).
func (s *Builder) buildNamedArrayLike(tio *types.TypeName, cmt *ast.CommentGroup, elem types.Type, tgt ifaces.SwaggerTypable, forSlice, isModel bool) error {
	if !isModel {
		if handled, recurse := s.classifierNamedArrayLike(cmt, tgt, forSlice); handled {
			if recurse {
				defer s.descend("items")()
				return s.buildFromType(elem, tgt.Items())
			}
			return nil
		}
	}

	return s.resolveRefOr(tio, tgt, func() error {
		defer s.descend("items")()
		return s.buildFromType(elem, tgt.Items())
	})
}

// buildFromMap renders a Go map as a Swagger object with additionalProperties.
//
// A map is representable as {type: object, additionalProperties: V} only when its key marshals to a
// JSON string — string kinds, integer/uint kinds, or an encoding.TextMarshaler key
// (resolvers.IsJSONMapKey, mirroring encoding/json).
//
// A key that json.Marshal itself rejects (float, bool, struct without TextMarshaler, interface,
// …) is not silently dropped to a typeless property: it raises a warning and emits no
// additionalProperties (go-swagger#2251, §18).
func (s *Builder) buildFromMap(titpe *types.Map, tgt ifaces.SwaggerTypable) error {
	sch := tgt.Schema()
	if sch == nil {
		return fmt.Errorf("items doesn't support maps: %w", ErrSchema)
	}

	key := titpe.Key()
	if !resolvers.IsJSONMapKey(key) {
		s.RecordDiagnostic(grammar.Warnf(
			s.Ctx.PosOf(s.Decl.Spec.Pos()),
			grammar.CodeUnsupportedType,
			"map key type %s does not marshal to a JSON object key "+
				"(encoding/json supports string, integer kinds, or encoding.TextMarshaler); "+
				"additionalProperties dropped",
			key.String(),
		))
		return nil
	}

	eleProp := NewTypable(sch, tgt.Level(), s.skipExtensions)
	defer s.descend("additionalProperties")()

	return s.buildFromType(titpe.Elem(), eleProp.AdditionalProperties())
}

// annotateSchema returns a deferrable that decorates schema with x-go-name / x-go-package
// traceability extensions, unless the schema is just a $ref, in which case the source origin is the
// target's definition, not the reference site.
//
// When Options.EmitXGoType is set it also stamps x-go-type with the fully-qualified Go type — but
// only if a recognizer hasn't already set it (the special-type cases for `error` / the generic
// fallback carry their own deliberate value, which we must not clobber).
// See [§traceability](./README.md#traceability).
func (s *Builder) annotateSchema(schema *oaispec.Schema) func() {
	return func() {
		if schema.Ref.String() != "" {
			return
		}
		if s.Name != s.GoName {
			resolvers.AddExtension(&schema.VendorExtensible, "x-go-name", s.GoName, s.skipExtensions)
		}
		resolvers.AddExtension(&schema.VendorExtensible, "x-go-package", s.Decl.Obj().Pkg().Path(), s.skipExtensions)
		if s.emitXGoType {
			if _, exists := schema.Extensions["x-go-type"]; !exists {
				resolvers.AddExtension(&schema.VendorExtensible, "x-go-type", s.Decl.Obj().Pkg().Path()+"."+s.GoName, s.skipExtensions)
			}
		}
	}
}

// guardDecl filters known-unsupported builtins (unsafe.Pointer) and asserts the scanner-side
// invariant that o.Pkg() is non-nil before downstream code dereferences it.
//
// Returns true if the caller should skip processing.
func (s *Builder) guardDecl(tpe ifaces.Objecter) (skip bool) {
	if resolvers.UnsupportedBuiltin(tpe) {
		s.warnUnsupportedGoType("guardDecl", tpe)
		return true
	}
	resolvers.MustNotBeABuiltinType(tpe.Obj()) // invariant
	return false
}

// hasNamedCore reports whether tpe is, or peels through pointers to reach, a *types.Named.
//
// Gates content-based checks (notably the TextMarshaler shortcut) to types whose name is
// inspectable.
// Anonymous structural kinds can satisfy TextMarshaler via embedded method promotion but must take
// the structural dispatch instead.
func hasNamedCore(tpe types.Type) bool {
	for {
		switch t := tpe.(type) {
		case *types.Named:
			return true
		case *types.Pointer:
			tpe = t.Elem()
		default:
			return false
		}
	}
}

// rhsTypeName extracts the *types.TypeName from an alias RHS when it is one of the two kinds
// applyStdlibSpecials accepts: *types.Named (the direct stdlib reference, e.g. `Timestamp =
// time.Time`) or *types.Alias (the chained reference, e.g. `Wrap = Timestamp`).
//
// Returns nil for anonymous and other RHS kinds — applyStdlibSpecials has nothing to recognize on
// those.
//
// Used by buildDeclAlias's Expand branch to consult the stdlib recognizers before walking
// Underlying.
func rhsTypeName(rhs types.Type) *types.TypeName {
	switch t := rhs.(type) {
	case *types.Named:
		return t.Obj()
	case *types.Alias:
		return t.Obj()
	default:
		return nil
	}
}
