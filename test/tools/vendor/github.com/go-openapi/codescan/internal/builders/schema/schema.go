// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/types"
	"log"
	"reflect"
	"strings"

	"github.com/go-openapi/swag/mangling"
	"golang.org/x/tools/go/packages"

	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/logger"
	"github.com/go-openapi/codescan/internal/parsers"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

type Builder struct {
	ctx        *scanner.ScanCtx
	decl       *scanner.EntityDecl
	GoName     string
	Name       string
	annotated  bool
	discovered []*scanner.EntityDecl
	postDecls  []*scanner.EntityDecl

	// interfaceMethodMangler produces JSON-style property names from Go
	// interface-method names. Interface methods cannot carry struct tags, so
	// codescan can't read a per-field convention — instead it applies the
	// same transform go-swagger uses for tag-less struct fields (acronym-aware
	// lower-first, e.g. `CreatedAt → createdAt`, `ID → id`,
	// `ExternalID → externalId`). `swagger:name` still takes precedence when
	// present. NameMangler is thread-safe per its godoc.
	//
	// Pointer so that the zero value (nil) is safely detected and lazily
	// initialized by interfaceJSONName — a zero mangling.NameMangler value
	// panics on use, and tests that construct &Builder{…} directly bypass
	// NewBuilder.
	interfaceMethodMangler *mangling.NameMangler
}

func NewBuilder(ctx *scanner.ScanCtx, decl *scanner.EntityDecl) *Builder {
	m := mangling.NewNameMangler()
	return &Builder{
		ctx:                    ctx,
		decl:                   decl,
		interfaceMethodMangler: &m,
	}
}

func (s *Builder) Build(definitions map[string]oaispec.Schema) error {
	s.inferNames()

	schema := definitions[s.Name]
	err := s.buildFromDecl(s.decl, &schema)
	if err != nil {
		return err
	}
	definitions[s.Name] = schema

	return nil
}

func (s *Builder) SetDiscovered(discovered []*scanner.EntityDecl) {
	s.discovered = discovered
}

func (s *Builder) PostDeclarations() []*scanner.EntityDecl {
	return s.postDecls
}

func (s *Builder) InferNames() {
	s.inferNames()
}

func (s *Builder) BuildFromType(tpe types.Type, tgt ifaces.SwaggerTypable) error {
	return s.buildFromType(tpe, tgt)
}

func (s *Builder) inferNames() {
	if s.GoName != "" {
		return
	}

	goName := s.decl.Ident.Name
	s.GoName = goName
	s.Name = goName

	override, ok := parsers.ModelOverride(s.decl.Comments)
	if !ok {
		return
	}
	s.annotated = true
	// Why: ModelOverride returns ("", true) for a bare `swagger:model` annotation
	// without a name — in that case the Go identifier is the model name.
	if override != "" {
		s.Name = override
	}
}

// interfaceJSONName maps a Go interface-method name to its JSON property
// name via the Builder's mangler, lazily initializing the mangler on first
// use so a zero-value Builder remains usable.
func (s *Builder) interfaceJSONName(goName string) string {
	if s.interfaceMethodMangler == nil {
		m := mangling.NewNameMangler()
		s.interfaceMethodMangler = &m
	}
	return s.interfaceMethodMangler.ToJSONName(goName)
}

func (s *Builder) buildFromDecl(_ *scanner.EntityDecl, schema *oaispec.Schema) error {
	// analyze doc comment for the model
	// This includes parsing "example", "default" and other validation at the top-level declaration.
	sp := s.createParser("", schema, schema, nil,
		parsers.WithSetTitle(func(lines []string) { schema.Title = parsers.JoinDropLast(lines) }),
		parsers.WithSetDescription(func(lines []string) {
			schema.Description = parsers.JoinDropLast(lines)
			enumDesc := parsers.GetEnumDesc(schema.Extensions)
			if enumDesc != "" {
				schema.Description += "\n" + enumDesc
			}
		}),
	)

	if err := sp.Parse(s.decl.Comments); err != nil {
		return err
	}

	// if the type is marked to ignore, just return
	if sp.Ignored() {
		return nil
	}

	defer func() {
		if schema.Ref.String() == "" {
			// unless this is a $ref, we add traceability of the origin of this schema in source
			if s.Name != s.GoName {
				resolvers.AddExtension(&schema.VendorExtensible, "x-go-name", s.GoName, s.ctx.SkipExtensions())
			}
			resolvers.AddExtension(&schema.VendorExtensible, "x-go-package", s.decl.Obj().Pkg().Path(), s.ctx.SkipExtensions())
		}
	}()

	switch tpe := s.decl.ObjType().(type) {
	case *types.Named:
		logger.DebugLogf(s.ctx.Debug(), "named: %v", tpe)
		return s.buildDeclNamed(tpe, schema)
	case *types.Alias:
		logger.DebugLogf(s.ctx.Debug(), "alias: %v -> %v", tpe, tpe.Rhs())
		tgt := Typable{schema, 0, s.ctx.SkipExtensions()}

		return s.buildDeclAlias(tpe, tgt)
	default:
		logger.UnsupportedTypeKind("buildFromDecl", tpe)
		return nil
	}
}

func (s *Builder) buildDeclNamed(tpe *types.Named, schema *oaispec.Schema) error {
	if resolvers.UnsupportedBuiltin(tpe) {
		log.Printf("WARNING: skipped unsupported builtin type: %v", tpe)

		return nil
	}
	o := tpe.Obj()

	resolvers.MustNotBeABuiltinType(o)

	logger.DebugLogf(s.ctx.Debug(), "got the named type object: %s.%s | isAlias: %t | exported: %t", o.Pkg().Path(), o.Name(), o.IsAlias(), o.Exported())
	if resolvers.IsStdTime(o) {
		schema.Typed("string", "date-time")
		return nil
	}

	ps := Typable{schema, 0, s.ctx.SkipExtensions()}
	ti := s.decl.Pkg.TypesInfo.Types[s.decl.Spec.Type]
	if !ti.IsType() {
		return fmt.Errorf("declaration is not a type: %v: %w", o, ErrSchema)
	}

	return s.buildFromType(ti.Type, ps)
}

// buildFromTextMarshal renders a type that marshals as text as a string.
func (s *Builder) buildFromTextMarshal(tpe types.Type, tgt ifaces.SwaggerTypable) error {
	if typePtr, ok := tpe.(*types.Pointer); ok {
		return s.buildFromTextMarshal(typePtr.Elem(), tgt)
	}

	// An alias surfaced under a pointer (e.g. *Timestamp where
	// Timestamp = time.Time) — route through buildAlias so the alias
	// indirection is honored per RefAliases/TransparentAliases, same as
	// the non-pointer path in buildFromType.
	if typeAlias, ok := tpe.(*types.Alias); ok {
		return s.buildAlias(typeAlias, tgt)
	}

	typeNamed, ok := tpe.(*types.Named)
	if !ok {
		tgt.Typed("string", "")
		return nil
	}

	tio := typeNamed.Obj()
	if resolvers.IsStdError(tio) {
		tgt.AddExtension("x-go-type", tio.Name())
		return resolvers.SwaggerSchemaForType(tio.Name(), tgt)
	}

	logger.DebugLogf(s.ctx.Debug(), "named refined type %s.%s", tio.Pkg().Path(), tio.Name())
	pkg, found := s.ctx.PkgForType(tpe)

	if strings.ToLower(tio.Name()) == "uuid" {
		tgt.Typed("string", "uuid")
		return nil
	}

	if !found {
		// this must be a builtin
		logger.DebugLogf(s.ctx.Debug(), "skipping because package is nil: %v", tpe)
		return nil
	}

	if resolvers.IsStdTime(tio) {
		tgt.Typed("string", "date-time")
		return nil
	}

	if resolvers.IsStdJSONRawMessage(tio) {
		tgt.Typed("object", "") // TODO: this should actually be any type
		return nil
	}

	cmt, hasComments := s.ctx.FindComments(pkg, tio.Name())
	if !hasComments {
		cmt = new(ast.CommentGroup)
	}

	if sfnm, isf := parsers.StrfmtName(cmt); isf {
		tgt.Typed("string", sfnm)
		return nil
	}

	tgt.Typed("string", "")
	tgt.AddExtension("x-go-type", tio.Pkg().Path()+"."+tio.Name())

	return nil
}

// hasNamedCore reports whether tpe is a *types.Named, or resolves to one
// by peeling one or more pointer layers. Used to gate content-based
// shortcuts (like the TextMarshaler check) to types whose name can be
// inspected — anonymous structural kinds cannot yield meaningful output
// from those shortcuts and should take the structural dispatch instead.
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

func (s *Builder) buildFromType(tpe types.Type, tgt ifaces.SwaggerTypable) error {
	logger.DebugLogf(s.ctx.Debug(), "schema buildFromType %v (%T)", tpe, tpe)

	// Aliases are dispatched first, before any content-based shortcut,
	// so the alias indirection is honored consistently with the caller's
	// RefAliases/TransparentAliases intent. Without this, a text-
	// marshalable alias (e.g. `type Timestamp = time.Time`) would be
	// inlined as a plain string — losing both the alias semantics and
	// (because buildFromTextMarshal only unwraps pointers) the target's
	// format.
	if titpe, ok := tpe.(*types.Alias); ok {
		logger.DebugLogf(s.ctx.Debug(), "alias(schema.buildFromType): got alias %v to %v", titpe, titpe.Rhs())
		return s.buildAlias(titpe, tgt)
	}

	// Only shortcut to the TextMarshaler renderer when we can reach a
	// *types.Named by peeling pointers — buildFromTextMarshal uses the
	// name to map to known formats (time/uuid/json.RawMessage/strfmt) and
	// falls back to {string, ""} otherwise. An anonymous struct that only
	// satisfies TextMarshaler by embedding time.Time (method promotion)
	// would otherwise be flattened to {string}, erasing its body and any
	// allOf composition. See Q4 in .claude/plans/observed-quirks.md.
	if hasNamedCore(tpe) && resolvers.IsTextMarshaler(tpe) {
		return s.buildFromTextMarshal(tpe, tgt)
	}

	switch titpe := tpe.(type) {
	case *types.Basic:
		if resolvers.UnsupportedBuiltinType(titpe) {
			log.Printf("WARNING: skipped unsupported builtin type: %v", tpe)
			return nil
		}
		return resolvers.SwaggerSchemaForType(titpe.String(), tgt)
	case *types.Pointer:
		return s.buildFromType(titpe.Elem(), tgt)
	case *types.Struct:
		return s.buildFromStruct(s.decl, titpe, tgt.Schema(), make(map[string]string))
	case *types.Interface:
		return s.buildFromInterface(s.decl, titpe, tgt.Schema(), make(map[string]string))
	case *types.Slice:
		// anonymous slice
		return s.buildFromType(titpe.Elem(), tgt.Items())
	case *types.Array:
		// anonymous array
		return s.buildFromType(titpe.Elem(), tgt.Items())
	case *types.Map:
		return s.buildFromMap(titpe, tgt)
	case *types.Named:
		// a named type, e.g. type X struct {}
		return s.buildNamedType(titpe, tgt)
	default:
		// Warn-and-skip for unsupported kinds (TypeParam, Chan, Signature,
		// Union, or future go/types additions). The scanner runs on user
		// code in uncontrolled environments, so panicking here would be a
		// worse experience than producing a partial spec.
		logger.UnsupportedTypeKind("buildFromType", titpe)
		return nil
	}
}

func (s *Builder) buildNamedType(titpe *types.Named, tgt ifaces.SwaggerTypable) error {
	tio := titpe.Obj()
	if resolvers.UnsupportedBuiltin(titpe) {
		log.Printf("WARNING: skipped unsupported builtin type: %v", titpe)
		return nil
	}

	if resolvers.IsAny(tio) {
		// e.g type X any or type X interface{}
		_ = tgt.Schema()

		return nil
	}

	// special case of the "error" interface.
	if resolvers.IsStdError(tio) {
		tgt.AddExtension("x-go-type", tio.Name())
		return resolvers.SwaggerSchemaForType(tio.Name(), tgt)
	}

	// special case of the "time.Time" type
	if resolvers.IsStdTime(tio) {
		tgt.Typed("string", "date-time")
		return nil
	}

	// special case of the "json.RawMessage" type
	if resolvers.IsStdJSONRawMessage(tio) {
		tgt.Typed("object", "") // TODO: this should actually be any type
		return nil
	}

	pkg, found := s.ctx.PkgForType(titpe)
	logger.DebugLogf(s.ctx.Debug(), "named refined type %s.%s", pkg, tio.Name())
	if !found {
		// this must be a builtin
		//
		// This could happen for example when using unsupported types such as complex64, complex128, uintptr,
		// or type constraints such as comparable.
		logger.DebugLogf(s.ctx.Debug(), "skipping because package is nil (builtin type): %v", tio)

		return nil
	}

	cmt, hasComments := s.ctx.FindComments(pkg, tio.Name())
	if !hasComments {
		cmt = new(ast.CommentGroup)
	}

	if tn, ok := parsers.TypeName(cmt); ok {
		if err := resolvers.SwaggerSchemaForType(tn, tgt); err == nil {
			return nil
		}
		// For unsupported swagger:type values (e.g., "array"), fall through
		// to underlying type resolution so the full schema (including items
		// for slices) is properly built. Build directly from the underlying
		// type to bypass the named-type $ref creation.
		return s.buildFromType(titpe.Underlying(), tgt)
	}

	if s.decl.Spec.Assign.IsValid() {
		logger.DebugLogf(s.ctx.Debug(), "found assignment: %s.%s", tio.Pkg().Path(), tio.Name())
		return s.buildFromType(titpe.Underlying(), tgt)
	}

	if titpe.TypeArgs() != nil && titpe.TypeArgs().Len() > 0 {
		return s.buildFromType(titpe.Underlying(), tgt)
	}

	// invariant: the Underlying cannot be an alias or named type
	switch utitpe := titpe.Underlying().(type) {
	case *types.Struct:
		return s.buildNamedStruct(tio, cmt, tgt)
	case *types.Interface:
		logger.DebugLogf(s.ctx.Debug(), "found interface: %s.%s", tio.Pkg().Path(), tio.Name())

		decl, found := s.ctx.FindModel(tio.Pkg().Path(), tio.Name())
		if !found {
			return fmt.Errorf("can't find source file for type: %v: %w", utitpe, ErrSchema)
		}

		return s.makeRef(decl, tgt)
	case *types.Basic:
		return s.buildNamedBasic(tio, pkg, cmt, utitpe, tgt)
	case *types.Array:
		return s.buildNamedArray(tio, cmt, utitpe.Elem(), tgt)
	case *types.Slice:
		return s.buildNamedSlice(tio, cmt, utitpe.Elem(), tgt)
	case *types.Map:
		logger.DebugLogf(s.ctx.Debug(), "found map type: %s.%s", tio.Pkg().Path(), tio.Name())

		if decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name()); ok {
			return s.makeRef(decl, tgt)
		}
		return nil
	default:
		logger.UnsupportedTypeKind("buildNamedType", utitpe)
		return nil
	}
}

func (s *Builder) buildNamedBasic(tio *types.TypeName, pkg *packages.Package, cmt *ast.CommentGroup, utitpe *types.Basic, tgt ifaces.SwaggerTypable) error {
	if resolvers.UnsupportedBuiltinType(utitpe) {
		log.Printf("WARNING: skipped unsupported builtin type: %v", utitpe)
		return nil
	}

	logger.DebugLogf(s.ctx.Debug(), "found primitive type: %s.%s", tio.Pkg().Path(), tio.Name())

	if sfnm, isf := parsers.StrfmtName(cmt); isf {
		tgt.Typed("string", sfnm)
		return nil
	}

	if enumName, ok := parsers.EnumName(cmt); ok {
		enumValues, enumDesces, _ := s.ctx.FindEnumValues(pkg, enumName)
		if len(enumValues) > 0 {
			tgt.WithEnum(enumValues...)
			enumTypeName := reflect.TypeOf(enumValues[0]).String()
			_ = resolvers.SwaggerSchemaForType(enumTypeName, tgt)
		}

		if len(enumDesces) > 0 {
			tgt.WithEnumDescription(strings.Join(enumDesces, "\n"))
		}

		return nil
	}

	if defaultName, ok := parsers.DefaultName(cmt); ok {
		logger.DebugLogf(s.ctx.Debug(), "default name: %s", defaultName)
		return nil
	}

	if typeName, ok := parsers.TypeName(cmt); ok {
		_ = resolvers.SwaggerSchemaForType(typeName, tgt)
		return nil
	}

	if parsers.IsAliasParam(tgt) || parsers.AliasParam(cmt) {
		err := resolvers.SwaggerSchemaForType(utitpe.Name(), tgt)
		if err == nil {
			return nil
		}
	}

	if decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name()); ok {
		return s.makeRef(decl, tgt)
	}

	return resolvers.SwaggerSchemaForType(utitpe.String(), tgt)
}

// buildNamedStruct emits a $ref to a named struct definition (or a strfmt
// override when `swagger:strfmt` is set on the type's doc).
//
// Preconditions established by the sole caller buildNamedType and not
// re-checked here:
//   - tio is never time.Time — IsStdTime(tio) short-circuits upstream to
//     {string, date-time} before the Underlying() switch runs.
//   - parsers.TypeName(cmt) is never set — the upstream TypeName branch
//     either resolves via SwaggerSchemaForType or delegates to
//     buildFromType(Underlying), so neither outcome reaches this function.
//
// Re-adding either check here would be dead code.
func (s *Builder) buildNamedStruct(tio *types.TypeName, cmt *ast.CommentGroup, tgt ifaces.SwaggerTypable) error {
	logger.DebugLogf(s.ctx.Debug(), "found struct: %s.%s", tio.Pkg().Path(), tio.Name())

	// Run strfmt first, before FindModel, so a `swagger:strfmt` type is
	// inlined as {string, format} *without* registering the struct in
	// ExtraModels — FindModel has a side effect that would otherwise emit
	// the struct as an orphan object definition no field references. See
	// Q10 in .claude/plans/observed-quirks.md.
	//
	// A caveat remains: when the author combines `swagger:strfmt` with
	// `swagger:model` (a "named strfmt" shape), the field still inlines
	// here while the top-level definition body is emitted by walking the
	// underlying struct. That inconsistency is documented in
	// .claude/plans/deferred-quirks.md and left for v2.
	if sfnm, isf := parsers.StrfmtName(cmt); isf {
		tgt.Typed("string", sfnm)
		return nil
	}

	decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name())
	if !ok {
		logger.DebugLogf(s.ctx.Debug(), "could not find model in index: %s.%s", tio.Pkg().Path(), tio.Name())
		return nil
	}

	return s.makeRef(decl, tgt)
}

func (s *Builder) buildNamedArray(tio *types.TypeName, cmt *ast.CommentGroup, elem types.Type, tgt ifaces.SwaggerTypable) error {
	logger.DebugLogf(s.ctx.Debug(), "found array type: %s.%s", tio.Pkg().Path(), tio.Name())

	if sfnm, isf := parsers.StrfmtName(cmt); isf {
		if sfnm == "byte" {
			tgt.Typed("string", sfnm)
			return nil
		}
		if sfnm == "bsonobjectid" {
			tgt.Typed("string", sfnm)
			return nil
		}

		tgt.Items().Typed("string", sfnm)
		return nil
	}
	// When swagger:type is set to an unsupported value (e.g., "array"),
	// skip the $ref and inline the array schema with proper items type.
	if tn, ok := parsers.TypeName(cmt); ok {
		if err := resolvers.SwaggerSchemaForType(tn, tgt); err != nil {
			return s.buildFromType(elem, tgt.Items())
		}
		return nil
	}
	if decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name()); ok {
		return s.makeRef(decl, tgt)
	}
	return s.buildFromType(elem, tgt.Items())
}

func (s *Builder) buildNamedSlice(tio *types.TypeName, cmt *ast.CommentGroup, elem types.Type, tgt ifaces.SwaggerTypable) error {
	logger.DebugLogf(s.ctx.Debug(), "found slice type: %s.%s", tio.Pkg().Path(), tio.Name())

	if sfnm, isf := parsers.StrfmtName(cmt); isf {
		if sfnm == "byte" {
			tgt.Typed("string", sfnm)
			return nil
		}
		tgt.Items().Typed("string", sfnm)
		return nil
	}
	// When swagger:type is set to an unsupported value (e.g., "array"),
	// skip the $ref and inline the slice schema with proper items type.
	// This preserves the field's description that would be lost with $ref.
	if tn, ok := parsers.TypeName(cmt); ok {
		if err := resolvers.SwaggerSchemaForType(tn, tgt); err != nil {
			// Unsupported type name (e.g., "array") — build inline from element type.
			return s.buildFromType(elem, tgt.Items())
		}
		return nil
	}
	if decl, ok := s.ctx.FindModel(tio.Pkg().Path(), tio.Name()); ok {
		return s.makeRef(decl, tgt)
	}
	return s.buildFromType(elem, tgt.Items())
}

// buildDeclAlias builds a top-level alias declaration.
//
// Note on LHS checks NOT performed here: IsAny(o) / IsStdError(o) /
// IsStdTime(o) on o := tpe.Obj() would all be false. o is the user's
// declared name (e.g. "X" in `type X = any`), so o.Pkg() is always the
// user's package and o.Name() is the user's identifier — neither
// matches the predeclared any/error (Pkg()==nil) nor stdlib time.Time
// (pkg "time", name "Time"). The live equivalents IsAny(ro) /
// IsStdError(ro) inside the `case *types.Alias:` branch of the RHS
// switch below do fire: they inspect the alias target, which for
// `type X = any` resolves to the predeclared any TypeName.
func (s *Builder) buildDeclAlias(tpe *types.Alias, tgt ifaces.SwaggerTypable) error {
	if resolvers.UnsupportedBuiltinType(tpe) {
		log.Printf("WARNING: skipped unsupported builtin type: %v", tpe)
		return nil
	}

	o := tpe.Obj()
	resolvers.MustNotBeABuiltinType(o)
	resolvers.MustHaveRightHandSide(tpe)
	rhs := tpe.Rhs()

	// If transparent aliases are enabled, use the underlying type directly without creating a definition
	if s.ctx.TransparentAliases() {
		return s.buildFromType(rhs, tgt)
	}

	decl, ok := s.ctx.FindModel(o.Pkg().Path(), o.Name())
	if !ok {
		return fmt.Errorf("can't find source file for aliased type: %v -> %v: %w", tpe, rhs, ErrSchema)
	}

	s.postDecls = append(s.postDecls, decl) // mark the left-hand side as discovered

	if !s.ctx.RefAliases() {
		// expand alias
		return s.buildFromType(tpe.Underlying(), tgt)
	}

	// resolve alias to named type as $ref
	switch rtpe := rhs.(type) {
	// named declarations: we construct a $ref to the right-hand side target of the alias
	case *types.Named:
		ro := rtpe.Obj()
		rdecl, found := s.ctx.FindModel(ro.Pkg().Path(), ro.Name())
		if !found {
			return fmt.Errorf("can't find source file for target type of alias: %v -> %v: %w", tpe, rtpe, ErrSchema)
		}

		return s.makeRef(rdecl, tgt)
	case *types.Alias:
		ro := rtpe.Obj()
		if resolvers.UnsupportedBuiltin(rtpe) {
			log.Printf("WARNING: skipped unsupported builtin type: %v", rtpe)
			return nil
		}
		if resolvers.IsAny(ro) {
			// e.g. type X = any
			_ = tgt.Schema() // this is mutating tgt to create an empty schema
			return nil
		}
		if resolvers.IsStdError(ro) {
			// e.g. type X = error
			tgt.AddExtension("x-go-type", o.Name())
			return resolvers.SwaggerSchemaForType(o.Name(), tgt)
		}
		resolvers.MustNotBeABuiltinType(ro) // TODO(fred): there are a few other cases

		rdecl, found := s.ctx.FindModel(ro.Pkg().Path(), ro.Name())
		if !found {
			return fmt.Errorf("can't find source file for target type of alias: %v -> %v: %w", tpe, rtpe, ErrSchema)
		}

		return s.makeRef(rdecl, tgt)
	}

	// alias to anonymous type
	return s.buildFromType(rhs, tgt)
}

func (s *Builder) buildAnonymousInterface(it *types.Interface, tgt ifaces.SwaggerTypable, decl *scanner.EntityDecl) error {
	tgt.Typed("object", "")

	for fld := range it.ExplicitMethods() {
		if err := s.processAnonInterfaceMethod(fld, it, decl, tgt.Schema()); err != nil {
			return err
		}
	}

	return nil
}

func (s *Builder) processAnonInterfaceMethod(fld *types.Func, it *types.Interface, decl *scanner.EntityDecl, schema *oaispec.Schema) error {
	if !fld.Exported() {
		return nil
	}
	sig, isSignature := fld.Type().(*types.Signature)
	if !isSignature {
		return nil
	}
	if sig.Params().Len() > 0 {
		return nil
	}
	if sig.Results() == nil || sig.Results().Len() != 1 {
		return nil
	}

	afld := resolvers.FindASTField(decl.File, fld.Pos())
	if afld == nil {
		logger.DebugLogf(s.ctx.Debug(), "can't find source associated with %s for %s", fld.String(), it.String())
		return nil
	}

	if parsers.Ignored(afld.Doc) {
		return nil
	}

	name, ok := parsers.NameOverride(afld.Doc)
	if !ok {
		name = s.interfaceJSONName(fld.Name())
	}

	if schema.Properties == nil {
		schema.Properties = make(map[string]oaispec.Schema)
	}
	ps := schema.Properties[name]
	if err := s.buildFromType(sig.Results().At(0).Type(), Typable{&ps, 0, s.ctx.SkipExtensions()}); err != nil {
		return err
	}
	if sfName, isStrfmt := parsers.StrfmtName(afld.Doc); isStrfmt {
		ps.Typed("string", sfName)
		ps.Ref = oaispec.Ref{}
		ps.Items = nil
	}

	sp := s.createParser(name, schema, &ps, afld)
	if err := sp.Parse(afld.Doc); err != nil {
		return err
	}

	if ps.Ref.String() == "" && name != fld.Name() {
		ps.AddExtension("x-go-name", fld.Name())
	}

	if s.ctx.SetXNullableForPointers() {
		_, isPointer := fld.Type().(*types.Signature).Results().At(0).Type().(*types.Pointer)
		noNullableExt := ps.Extensions == nil ||
			(ps.Extensions["x-nullable"] == nil && ps.Extensions["x-isnullable"] == nil)
		if isPointer && noNullableExt {
			ps.AddExtension("x-nullable", true)
		}
	}

	schema.Properties[name] = ps
	return nil
}

// buildAlias builds a reference to an alias from another type.
func (s *Builder) buildAlias(tpe *types.Alias, tgt ifaces.SwaggerTypable) error {
	if resolvers.UnsupportedBuiltinType(tpe) {
		log.Printf("WARNING: skipped unsupported builtin type: %v", tpe)

		return nil
	}

	o := tpe.Obj()
	if resolvers.IsAny(o) {
		_ = tgt.Schema()
		return nil
	}
	resolvers.MustNotBeABuiltinType(o)

	// If transparent aliases are enabled, use the underlying type directly
	if s.ctx.TransparentAliases() {
		return s.buildFromType(tpe.Rhs(), tgt)
	}

	decl, ok := s.ctx.FindModel(o.Pkg().Path(), o.Name())
	if !ok {
		return fmt.Errorf("can't find source file for aliased type: %v: %w", tpe, ErrSchema)
	}

	return s.makeRef(decl, tgt)
}

func (s *Builder) buildFromMap(titpe *types.Map, tgt ifaces.SwaggerTypable) error {
	// check if key is a string type, or knows how to marshall to text.
	// If not, print a message and skip the map property.
	//
	// Only maps with string keys can go into additional properties

	sch := tgt.Schema()
	if sch == nil {
		return fmt.Errorf("items doesn't support maps: %w", ErrSchema)
	}

	eleProp := Typable{sch, tgt.Level(), s.ctx.SkipExtensions()}
	key := titpe.Key()
	if key.Underlying().String() == "string" || resolvers.IsTextMarshaler(key) {
		return s.buildFromType(titpe.Elem(), eleProp.AdditionalProperties())
	}

	return nil
}

func (s *Builder) buildFromInterface(decl *scanner.EntityDecl, it *types.Interface, schema *oaispec.Schema, seen map[string]string) error {
	if it.Empty() {
		// return an empty schema for empty interfaces
		return nil
	}

	var (
		tgt      *oaispec.Schema
		hasAllOf bool
	)

	var flist []*ast.Field
	if specType, ok := decl.Spec.Type.(*ast.InterfaceType); ok {
		flist = make([]*ast.Field, it.NumEmbeddeds()+it.NumExplicitMethods())
		copy(flist, specType.Methods.List)
	}

	// First collect the embedded interfaces
	// create refs when:
	//
	//   1. the embedded interface is decorated with an allOf annotation
	//   2. the embedded interface is an alias
	for fld := range it.EmbeddedTypes() {
		if tgt == nil {
			tgt = &oaispec.Schema{}
		}

		fieldHasAllOf, err := s.processEmbeddedType(fld, flist, decl, schema, seen)
		if err != nil {
			return err
		}
		hasAllOf = hasAllOf || fieldHasAllOf
	}

	if tgt == nil {
		tgt = schema
	}

	// We can finally build the actual schema for the struct
	if tgt.Properties == nil {
		tgt.Properties = make(map[string]oaispec.Schema)
	}
	tgt.Typed("object", "")

	for fld := range it.ExplicitMethods() {
		if err := s.processInterfaceMethod(fld, it, decl, tgt, seen); err != nil {
			return err
		}
	}

	if tgt == nil {
		return nil
	}
	if hasAllOf && len(tgt.Properties) > 0 {
		schema.AllOf = append(schema.AllOf, *tgt)
	}

	for k := range tgt.Properties {
		if _, ok := seen[k]; !ok {
			delete(tgt.Properties, k)
		}
	}

	return nil
}

func (s *Builder) processEmbeddedType(
	fld types.Type,
	flist []*ast.Field,
	decl *scanner.EntityDecl,
	schema *oaispec.Schema,
	seen map[string]string,
) (fieldHasAllOf bool, err error) {
	logger.DebugLogf(s.ctx.Debug(), "inspecting embedded type in interface: %v", fld)

	switch ftpe := fld.(type) {
	case *types.Named:
		logger.DebugLogf(s.ctx.Debug(), "embedded named type (buildInterface): %v", ftpe)
		o := ftpe.Obj()
		if resolvers.IsAny(o) || resolvers.IsStdError(o) {
			return false, nil
		}
		return s.buildNamedInterface(ftpe, flist, decl, schema, seen)
	case *types.Interface:
		logger.DebugLogf(s.ctx.Debug(), "embedded anonymous interface type (buildInterface): %v", ftpe)
		var aliasedSchema oaispec.Schema
		ps := Typable{schema: &aliasedSchema, skipExt: s.ctx.SkipExtensions()}
		if err = s.buildAnonymousInterface(ftpe, ps, decl); err != nil {
			return false, err
		}
		if aliasedSchema.Ref.String() != "" || len(aliasedSchema.Properties) > 0 || len(aliasedSchema.AllOf) > 0 {
			fieldHasAllOf = true
			schema.AddToAllOf(aliasedSchema)
		}
	case *types.Alias:
		logger.DebugLogf(s.ctx.Debug(), "embedded alias (buildInterface): %v -> %v", ftpe, ftpe.Rhs())
		var aliasedSchema oaispec.Schema
		ps := Typable{schema: &aliasedSchema, skipExt: s.ctx.SkipExtensions()}
		if err = s.buildAlias(ftpe, ps); err != nil {
			return false, err
		}
		if aliasedSchema.Ref.String() != "" || len(aliasedSchema.Properties) > 0 || len(aliasedSchema.AllOf) > 0 {
			fieldHasAllOf = true
			schema.AddToAllOf(aliasedSchema)
		}
	default:
		logger.UnsupportedTypeKind("buildNamedInterface.allOf", ftpe)
	}

	logger.DebugLogf(s.ctx.Debug(), "got embedded interface: %v {%T}, fieldHasAllOf: %t", fld, fld, fieldHasAllOf)
	return fieldHasAllOf, nil
}

func (s *Builder) processInterfaceMethod(fld *types.Func, it *types.Interface, decl *scanner.EntityDecl, tgt *oaispec.Schema, seen map[string]string) error {
	if !fld.Exported() {
		return nil
	}

	sig, isSignature := fld.Type().(*types.Signature)
	if !isSignature {
		return nil
	}

	if sig.Params().Len() > 0 {
		return nil
	}

	if sig.Results() == nil || sig.Results().Len() != 1 {
		return nil
	}

	afld := resolvers.FindASTField(decl.File, fld.Pos())
	if afld == nil {
		logger.DebugLogf(s.ctx.Debug(), "can't find source associated with %s for %s", fld.String(), it.String())
		return nil
	}

	// if the field is annotated with swagger:ignore, ignore it
	if parsers.Ignored(afld.Doc) {
		return nil
	}

	name, ok := parsers.NameOverride(afld.Doc)
	if !ok {
		name = s.interfaceJSONName(fld.Name())
	}

	ps := tgt.Properties[name]
	if err := s.buildFromType(sig.Results().At(0).Type(), Typable{&ps, 0, s.ctx.SkipExtensions()}); err != nil {
		return err
	}

	if sfName, isStrfmt := parsers.StrfmtName(afld.Doc); isStrfmt {
		ps.Typed("string", sfName)
		ps.Ref = oaispec.Ref{}
		ps.Items = nil
	}

	sp := s.createParser(name, tgt, &ps, afld)
	if err := sp.Parse(afld.Doc); err != nil {
		return err
	}

	if ps.Ref.String() == "" && name != fld.Name() {
		ps.AddExtension("x-go-name", fld.Name())
	}

	if s.ctx.SetXNullableForPointers() {
		_, isPointer := fld.Type().(*types.Signature).Results().At(0).Type().(*types.Pointer)
		noNullableExt := ps.Extensions == nil ||
			(ps.Extensions["x-nullable"] == nil && ps.Extensions["x-isnullable"] == nil)
		if isPointer && noNullableExt {
			ps.AddExtension("x-nullable", true)
		}
	}

	seen[name] = fld.Name()
	tgt.Properties[name] = ps

	return nil
}

func (s *Builder) buildNamedInterface(ftpe *types.Named, flist []*ast.Field, decl *scanner.EntityDecl, schema *oaispec.Schema, seen map[string]string) (hasAllOf bool, err error) {
	o := ftpe.Obj()
	var afld *ast.Field

	for _, an := range flist {
		if len(an.Names) != 0 {
			continue
		}

		tpp := decl.Pkg.TypesInfo.Types[an.Type]
		if tpp.Type.String() != o.Type().String() {
			continue
		}

		// decl.
		logger.DebugLogf(s.ctx.Debug(), "maybe interface field %s: %s(%T)", o.Name(), o.Type().String(), o.Type())
		afld = an
		break
	}

	if afld == nil {
		logger.DebugLogf(s.ctx.Debug(), "can't find source associated with %s", ftpe.String())
		return hasAllOf, nil
	}

	// if the field is annotated with swagger:ignore, ignore it
	if parsers.Ignored(afld.Doc) {
		return hasAllOf, nil
	}

	if !parsers.AllOfMember(afld.Doc) {
		var newSch oaispec.Schema
		if err = s.buildEmbedded(o.Type(), &newSch, seen); err != nil {
			return hasAllOf, err
		}
		schema.AllOf = append(schema.AllOf, newSch)
		hasAllOf = true

		return hasAllOf, nil
	}

	hasAllOf = true

	var newSch oaispec.Schema
	// when the embedded struct is annotated with swagger:allOf it will be used as allOf property
	// otherwise the fields will just be included as normal properties
	if err = s.buildAllOf(o.Type(), &newSch); err != nil {
		return hasAllOf, err
	}

	if afld.Doc != nil {
		extractAllOfClass(afld.Doc, schema)
	}

	schema.AllOf = append(schema.AllOf, newSch)

	return hasAllOf, nil
}

func (s *Builder) buildFromStruct(decl *scanner.EntityDecl, st *types.Struct, schema *oaispec.Schema, seen map[string]string) error {
	cmt, hasComments := s.ctx.FindComments(decl.Pkg, decl.Obj().Name())
	if !hasComments {
		cmt = new(ast.CommentGroup)
	}
	name, ok := parsers.TypeName(cmt)
	if ok {
		_ = resolvers.SwaggerSchemaForType(name, Typable{schema: schema, skipExt: s.ctx.SkipExtensions()})
		return nil
	}
	// First pass: scan anonymous/embedded fields for allOf composition.
	// Returns the target schema for properties (may differ from schema when allOf is used).
	tgt, hasAllOf, err := s.scanEmbeddedFields(decl, st, schema, seen)
	if err != nil {
		return err
	}

	if tgt == nil {
		if schema != nil {
			tgt = schema
		} else {
			tgt = &oaispec.Schema{}
		}
	}
	if tgt.Properties == nil {
		tgt.Properties = make(map[string]oaispec.Schema)
	}
	tgt.Typed("object", "")

	// Second pass: build properties from non-embedded exported fields.
	if err := s.buildStructFields(decl, st, tgt, seen); err != nil {
		return err
	}

	if tgt == nil {
		return nil
	}
	if hasAllOf && len(tgt.Properties) > 0 {
		schema.AllOf = append(schema.AllOf, *tgt)
	}
	for k := range tgt.Properties {
		if _, ok := seen[k]; !ok {
			delete(tgt.Properties, k)
		}
	}
	return nil
}

// scanEmbeddedFields iterates over anonymous struct fields to detect allOf composition.
// It returns:
//   - tgt: the schema that should receive properties (nil if no embedded fields were processed,
//     schema itself for plain embeds, or a new schema when allOf is detected)
//   - hasAllOf: whether any allOf member was found
func (s *Builder) scanEmbeddedFields(decl *scanner.EntityDecl, st *types.Struct, schema *oaispec.Schema, seen map[string]string) (tgt *oaispec.Schema, hasAllOf bool, err error) {
	for i := range st.NumFields() {
		fld := st.Field(i)
		if !fld.Anonymous() {
			logger.DebugLogf(s.ctx.Debug(), "skipping field %q for allOf scan because not anonymous", fld.Name())
			continue
		}
		tg := st.Tag(i)

		logger.DebugLogf(s.ctx.Debug(),
			"maybe allof field(%t) %s: %s (%T) [%q](anon: %t, embedded: %t)",
			fld.IsField(), fld.Name(), fld.Type().String(), fld.Type(), tg, fld.Anonymous(), fld.Embedded(),
		)
		afld := resolvers.FindASTField(decl.File, fld.Pos())
		if afld == nil {
			logger.DebugLogf(s.ctx.Debug(), "can't find source associated with %s for %s", fld.String(), st.String())
			continue
		}

		if parsers.Ignored(afld.Doc) {
			continue
		}

		_, ignore, _, _, err := resolvers.ParseJSONTag(afld)
		if err != nil {
			return nil, false, err
		}
		if ignore {
			continue
		}

		_, isAliased := fld.Type().(*types.Alias)

		if !parsers.AllOfMember(afld.Doc) && !isAliased {
			// Plain embed: merge fields into the main schema
			if tgt == nil {
				tgt = schema
			}
			if err := s.buildEmbedded(fld.Type(), tgt, seen); err != nil {
				return nil, false, err
			}
			continue
		}

		if isAliased {
			logger.DebugLogf(s.ctx.Debug(), "alias member in struct: %v", fld)
		}

		// allOf member: fields go into a separate schema, embedded struct becomes an allOf entry
		hasAllOf = true
		if tgt == nil {
			tgt = &oaispec.Schema{}
		}
		var newSch oaispec.Schema
		if err := s.buildAllOf(fld.Type(), &newSch); err != nil {
			return nil, false, err
		}

		extractAllOfClass(afld.Doc, schema)
		schema.AllOf = append(schema.AllOf, newSch)
	}

	return tgt, hasAllOf, nil
}

func (s *Builder) buildStructFields(decl *scanner.EntityDecl, st *types.Struct, tgt *oaispec.Schema, seen map[string]string) error {
	for fld := range st.Fields() {
		if err := s.processStructField(fld, decl, tgt, seen); err != nil {
			return err
		}
	}
	return nil
}

func (s *Builder) processStructField(fld *types.Var, decl *scanner.EntityDecl, tgt *oaispec.Schema, seen map[string]string) error {
	if fld.Embedded() || !fld.Exported() {
		return nil
	}

	afld := resolvers.FindASTField(decl.File, fld.Pos())
	if afld == nil {
		logger.DebugLogf(s.ctx.Debug(), "can't find source associated with %s", fld.String())
		return nil
	}

	if parsers.Ignored(afld.Doc) {
		return nil
	}

	name, ignore, isString, omitEmpty, err := resolvers.ParseJSONTag(afld)
	if err != nil {
		return err
	}

	if ignore {
		for seenTagName, seenFieldName := range seen {
			if seenFieldName == fld.Name() {
				delete(tgt.Properties, seenTagName)
				break
			}
		}
		return nil
	}

	ps := tgt.Properties[name]
	if err = s.buildFromType(fld.Type(), Typable{&ps, 0, s.ctx.SkipExtensions()}); err != nil {
		return err
	}
	if isString {
		ps.Typed("string", ps.Format)
		ps.Ref = oaispec.Ref{}
		ps.Items = nil
	}

	if sfName, isStrfmt := parsers.StrfmtName(afld.Doc); isStrfmt {
		ps.Typed("string", sfName)
		ps.Ref = oaispec.Ref{}
		ps.Items = nil
	}

	sp := s.createParser(name, tgt, &ps, afld)
	if err := sp.Parse(afld.Doc); err != nil {
		return err
	}

	if ps.Ref.String() == "" && name != fld.Name() {
		resolvers.AddExtension(&ps.VendorExtensible, "x-go-name", fld.Name(), s.ctx.SkipExtensions())
	}

	if s.ctx.SetXNullableForPointers() {
		if _, isPointer := fld.Type().(*types.Pointer); isPointer && !omitEmpty &&
			(ps.Extensions == nil || (ps.Extensions["x-nullable"] == nil && ps.Extensions["x-isnullable"] == nil)) {
			ps.AddExtension("x-nullable", true)
		}
	}

	// we have 2 cases:
	// 1. field with different name override tag
	// 2. field with different name removes tag
	// so we need to save both tag&name
	seen[name] = fld.Name()
	tgt.Properties[name] = ps
	return nil
}

func (s *Builder) buildAllOf(tpe types.Type, schema *oaispec.Schema) error {
	logger.DebugLogf(s.ctx.Debug(), "allOf %s", tpe.Underlying())

	switch ftpe := tpe.(type) {
	case *types.Pointer:
		return s.buildAllOf(ftpe.Elem(), schema)
	case *types.Named:
		return s.buildNamedAllOf(ftpe, schema)
	case *types.Alias:
		logger.DebugLogf(s.ctx.Debug(), "allOf member is alias %v => %v", ftpe, ftpe.Rhs())
		tgt := Typable{schema: schema, skipExt: s.ctx.SkipExtensions()}
		return s.buildAlias(ftpe, tgt)
	default:
		logger.UnsupportedTypeKind("buildAllOf", ftpe)
		return nil
	}
}

func (s *Builder) buildNamedAllOf(ftpe *types.Named, schema *oaispec.Schema) error {
	switch utpe := ftpe.Underlying().(type) {
	case *types.Struct:
		tio := ftpe.Obj()

		// Run inlining shortcuts (stdlib time, swagger:strfmt) before
		// FindModel — FindModel registers the type in ExtraModels as a
		// side effect, which would emit an orphan top-level definition
		// for a type whose schema we've already inlined. See Q10 in
		// .claude/plans/observed-quirks.md.
		if resolvers.IsStdTime(tio) {
			schema.Typed("string", "date-time")
			return nil
		}

		if pkg, ok := s.ctx.PkgForType(ftpe); ok {
			if cmt, hasComments := s.ctx.FindComments(pkg, tio.Name()); hasComments {
				if sfnm, isf := parsers.StrfmtName(cmt); isf {
					schema.Typed("string", sfnm)
					return nil
				}
			}
		}

		decl, found := s.ctx.FindModel(tio.Pkg().Path(), tio.Name())
		if !found {
			return fmt.Errorf("can't find source file for struct: %s: %w", ftpe.String(), ErrSchema)
		}

		if decl.HasModelAnnotation() {
			return s.makeRef(decl, Typable{schema, 0, s.ctx.SkipExtensions()})
		}

		return s.buildFromStruct(decl, utpe, schema, make(map[string]string))
	case *types.Interface:
		decl, found := s.ctx.FindModel(ftpe.Obj().Pkg().Path(), ftpe.Obj().Name())
		if !found {
			return fmt.Errorf("can't find source file for interface: %s: %w", ftpe.String(), ErrSchema)
		}

		if sfnm, isf := parsers.StrfmtName(decl.Comments); isf {
			schema.Typed("string", sfnm)
			return nil
		}

		if decl.HasModelAnnotation() {
			return s.makeRef(decl, Typable{schema, 0, s.ctx.SkipExtensions()})
		}

		return s.buildFromInterface(decl, utpe, schema, make(map[string]string))
	default:
		logger.UnsupportedTypeKind("buildNamedAllOf", utpe)
		return nil
	}
}

func (s *Builder) buildEmbedded(tpe types.Type, schema *oaispec.Schema, seen map[string]string) error {
	logger.DebugLogf(s.ctx.Debug(), "embedded %v", tpe.Underlying())

	switch ftpe := tpe.(type) {
	case *types.Pointer:
		return s.buildEmbedded(ftpe.Elem(), schema, seen)
	case *types.Named:
		return s.buildNamedEmbedded(ftpe, schema, seen)
	case *types.Alias:
		logger.DebugLogf(s.ctx.Debug(), "embedded alias %v => %v", ftpe, ftpe.Rhs())
		tgt := Typable{schema, 0, s.ctx.SkipExtensions()}
		return s.buildAlias(ftpe, tgt)
	default:
		logger.UnsupportedTypeKind("buildEmbedded", ftpe)
		return nil
	}
}

func (s *Builder) buildNamedEmbedded(ftpe *types.Named, schema *oaispec.Schema, seen map[string]string) error {
	logger.DebugLogf(s.ctx.Debug(), "embedded named type: %T", ftpe.Underlying())
	if resolvers.UnsupportedBuiltin(ftpe) {
		log.Printf("WARNING: skipped unsupported builtin type: %v", ftpe)

		return nil
	}

	switch utpe := ftpe.Underlying().(type) {
	case *types.Struct:
		decl, found := s.ctx.FindModel(ftpe.Obj().Pkg().Path(), ftpe.Obj().Name())
		if !found {
			return fmt.Errorf("can't find source file for struct: %s: %w", ftpe.String(), ErrSchema)
		}

		return s.buildFromStruct(decl, utpe, schema, seen)
	case *types.Interface:
		if utpe.Empty() {
			return nil
		}
		o := ftpe.Obj()
		if resolvers.IsAny(o) {
			return nil
		}
		if resolvers.IsStdError(o) {
			tgt := Typable{schema: schema, skipExt: s.ctx.SkipExtensions()}
			tgt.AddExtension("x-go-type", o.Name())
			return resolvers.SwaggerSchemaForType(o.Name(), tgt)
		}
		resolvers.MustNotBeABuiltinType(o)

		decl, found := s.ctx.FindModel(o.Pkg().Path(), o.Name())
		if !found {
			return fmt.Errorf("can't find source file for struct: %s: %w", ftpe.String(), ErrSchema)
		}
		return s.buildFromInterface(decl, utpe, schema, seen)
	default:
		logger.UnsupportedTypeKind("buildNamedEmbedded", utpe)
		return nil
	}
}

func (s *Builder) makeRef(decl *scanner.EntityDecl, prop ifaces.SwaggerTypable) error {
	nm, _ := decl.Names()
	ref, err := oaispec.NewRef("#/definitions/" + nm)
	if err != nil {
		return err
	}

	prop.SetRef(ref)
	s.postDecls = append(s.postDecls, decl)

	return nil
}

func (s *Builder) createParser(nm string, schema, ps *oaispec.Schema, fld *ast.Field, opts ...parsers.SectionedParserOption) *parsers.SectionedParser {
	if ps.Ref.String() != "" && !s.ctx.DescWithRef() {
		// if DescWithRef option is enabled, allow the tagged documentation to flow alongside the $ref
		// otherwise behave as expected by jsonschema draft4: $ref predates all sibling keys.
		opts = append(
			opts,
			parsers.WithTaggers(refSchemaTaggers(schema, nm)...),
		)

		return parsers.NewSectionedParser(opts...)
	}

	taggers := schemaTaggers(schema, ps, nm)

	// the parser may be called outside the context of struct field.
	// In that case, just return the outcome of the parsing now.

	if fld != nil {
		// check if this is a primitive, if so parse the validations from the
		// doc comments of the slice declaration.
		if ftped, ok := fld.Type.(*ast.ArrayType); ok {
			var err error
			arrayTaggers, err := parseArrayTypes(taggers, ftped.Elt, ps.Items, 0) // NOTE: swallows error silently
			if err == nil {
				taggers = arrayTaggers
			}
		}
	}

	opts = append(
		opts,
		parsers.WithSetDescription(func(lines []string) {
			ps.Description = parsers.JoinDropLast(lines)
			enumDesc := parsers.GetEnumDesc(ps.Extensions)
			if enumDesc != "" {
				ps.Description += "\n" + enumDesc
			}
		}),
		parsers.WithTaggers(taggers...),
	)

	return parsers.NewSectionedParser(opts...)
}

func schemaVendorExtensibleSetter(meta *oaispec.Schema) func(json.RawMessage) error {
	return func(jsonValue json.RawMessage) error {
		var jsonData oaispec.Extensions
		err := json.Unmarshal(jsonValue, &jsonData)
		if err != nil {
			return err
		}

		for k := range jsonData {
			if !parsers.IsAllowedExtension(k) {
				return fmt.Errorf("invalid schema extension name, should start from `x-`: %s: %w", k, ErrSchema)
			}
		}

		meta.Extensions = jsonData

		return nil
	}
}

func extractAllOfClass(doc *ast.CommentGroup, schema *oaispec.Schema) {
	allOfClass, ok := parsers.AllOfName(doc)
	if !ok {
		return
	}

	schema.AddExtension("x-class", allOfClass)
}
