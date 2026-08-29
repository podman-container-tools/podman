// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package responses

import (
	"errors"
	"fmt"
	"go/types"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/common"
	"github.com/go-openapi/codescan/internal/builders/handlers"
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/builders/schema"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

const (
	inBody   = "body"
	inHeader = "header"
)

// Builder constructs OAS v2 response entries for one `swagger:response` declaration.
//
// Embeds *common.Builder for shared state (Ctx, Decl, PostDeclarations, diagnostics, ParseBlocks
// cache).
type Builder struct {
	*common.Builder

	// inherited carries an embedded field's in: annotation down to the response fields it promotes
	// (go-swagger#2701) — the body/header routing discriminator.
	//
	// Set with save/restore around the embedded- field recursion in buildFromStruct.
	// The mechanism is shared with the schema and parameters builders via common.EmbedInheritance;
	// responses consume only In (OAS2 response headers carry no required).
	inherited common.EmbedInheritance

	// respBase is the cross-ref base pointer for this response — /responses/{name} — set per Build
	// when a provenance sink is wired ("" when off).
	//
	// Header anchors hang at respBase/headers/{h}; the in:body schema under respBase/schema. bodyPath
	// is the live cursor into the body-schema subtree, advanced by descendBody as the responses
	// builder peels its OWN array/map layers (delegated struct/named builds are pathed by the schema
	// builder).
	respBase string
	bodyPath string
}

// NewBuilder constructs an initialized [Builder] bound to ctx and decl.
//
// The embedded common.Builder owns the diagnostic sink, the post-declaration list, and the
// per-comment-group parse cache.
func NewBuilder(ctx *scanner.ScanCtx, decl *scanner.EntityDecl) *Builder {
	return &Builder{
		Builder: common.New(ctx, decl),
	}
}

// ResponseName resolves the spec name of this response declaration from the grammar
// (grammar.ResponseBlock): the explicit `swagger:response {name}` argument when present, else the
// Go type name (covering the bare `swagger:response` and the `swagger:response *` synonym, which
// both key the response by its type name).
//
// The targeting parse lives in the grammar, not the scanner.
func (r *Builder) ResponseName() string {
	for _, b := range r.ParseBlocks(r.Decl.Comments) {
		if rb, ok := b.(*grammar.ResponseBlock); ok {
			if rb.Name != "" {
				return rb.Name
			}
			break
		}
	}
	return r.Decl.Ident.Name
}

func (r *Builder) Build(responses map[string]oaispec.Response) error {
	// check if there is a swagger:response tag that is followed by one or more words, these words are
	// the ids of the operations this parameter struct applies to once type name is found convert it to
	// a schema, by looking up the schema in the parameters dictionary that got passed into this parse
	// method.

	name := r.ResponseName()
	response := responses[name]

	// Cross-ref linkage: anchor this response's headers and in:body schema under /responses/{name}.
	// The response name is known here (no deferral, unlike a parameter's array index), so the base
	// path is fixed for the whole build.
	if r.Ctx.OriginEnabled() {
		r.respBase = scanner.JSONPointer("responses", name)
		r.bodyPath = r.respBase + scanner.JSONPointer("schema")
	}

	// analyze doc comment for the model
	r.applyBlockToDecl(&response)

	// analyze struct body for fields etc
	// each exported struct field:
	// * gets a type mapped to a go primitive
	// * perhaps gets a format
	// * has to document the validations that apply for the type and the field
	// * when the struct field points to a model it becomes a ref: #/definitions/ModelName
	// * comments that aren't tags is used as the description
	if err := r.buildFromType(r.Decl.ObjType(), &response, make(map[string]bool)); err != nil {
		return err
	}

	// Carry decl-comment schema keywords (example:, default:, validations) onto a top-level non-struct
	// response body schema. applyBlockToDecl only takes the prose/description; without this, an
	// `example:` on a `swagger:response` whose body is a bare array/scalar type is dropped
	// (go-swagger#3013).
	//
	// Struct responses carry these on their fields, not the decl, and a $ref body must not gain
	// sibling keywords — both skipped.
	if response.Schema != nil && response.Schema.Ref.String() == "" && !underlyingIsStruct(r.Decl.ObjType()) {
		handlers.DispatchSchemaLevel0(
			r.ParseBlock(r.Decl.Comments), nil, response.Schema, "",
			r.RecordDiagnostic, handlers.SchemaOptions{},
		)
	}

	responses[name] = response

	return nil
}

// underlyingIsStruct reports whether t resolves (through named/alias/ pointer layers) to a struct
// — i.e. a struct-bodied response whose fields, not the decl comment, carry schema keywords.
func underlyingIsStruct(t types.Type) bool {
	for {
		switch tt := t.(type) {
		case *types.Named:
			t = tt.Underlying()
		case *types.Alias:
			t = tt.Underlying()
		case *types.Pointer:
			t = tt.Elem()
		case *types.Struct:
			return true
		default:
			return false
		}
	}
}

// descendBody advances the in:body schema cursor by segs for the duration of a child build,
// mirroring the schema builder's descend.
//
// It keeps bodyPath aligned with the node being filled when the responses builder peels its OWN
// array/map layers; types delegated to the schema sub-builder are pathed there instead.
// No-op (and no restore cost) when provenance is off (bodyPath == "").
func (r *Builder) descendBody(segs ...string) func() {
	if r.bodyPath == "" {
		return func() {}
	}
	saved := r.bodyPath
	r.bodyPath = saved + scanner.JSONPointer(segs...)
	return func() { r.bodyPath = saved }
}

// bodyPathFor returns the cross-ref base path to hand a schema sub-build: the live body cursor when
// the build targets the in:body schema, else "" — a header schema anchors at
// respBase/headers/{h}, not under /schema, so its finer nodes resolve to the header anchor rather
// than emitting a wrong /schema/... pointer.
func (r *Builder) bodyPathFor(typable ifaces.SwaggerTypable) string {
	if typable != nil && typable.In() == inBody {
		return r.bodyPath
	}
	return ""
}

func (r *Builder) buildFromField(fld *types.Var, tpe types.Type, typable ifaces.SwaggerTypable, seen map[string]bool) error {
	switch ftpe := tpe.(type) {
	case *types.Basic:
		return resolvers.SwaggerSchemaForType(ftpe.Name(), typable)
	case *types.Struct:
		return r.buildFromFieldStruct(ftpe, typable)
	case *types.Pointer:
		return r.buildFromField(fld, ftpe.Elem(), typable, seen)
	case *types.Interface:
		return r.buildFromFieldInterface(ftpe, typable)
	case *types.Array:
		defer r.descendBody("items")()
		return r.buildFromField(fld, ftpe.Elem(), typable.Items(), seen)
	case *types.Slice:
		defer r.descendBody("items")()
		return r.buildFromField(fld, ftpe.Elem(), typable.Items(), seen)
	case *types.Map:
		return r.buildFromFieldMap(ftpe, typable)
	case *types.Named:
		return r.buildNamedField(ftpe, typable)
	case *types.Alias:
		return r.buildFieldAlias(ftpe, typable, fld, seen)
	default:
		return fmt.Errorf("unknown type for %s: %T: %w", fld.String(), fld.Type(), ErrResponses)
	}
}

func (r *Builder) buildFromFieldStruct(ftpe *types.Struct, typable ifaces.SwaggerTypable) error {
	sb := schema.NewBuilder(r.Ctx, r.Decl)
	if err := sb.Build(schema.OptionFor(ftpe, typable), schema.WithPath(r.bodyPathFor(typable))); err != nil {
		return err
	}

	for _, d := range sb.PostDeclarations() {
		r.AppendPostDecl(d)
	}

	return nil
}

func (r *Builder) buildFromFieldMap(ftpe *types.Map, typable ifaces.SwaggerTypable) error {
	// A Go map is only representable under in=body (object + additionalProperties).
	// A response header is an OAS v2 SimpleSchema target with no map representation.
	// Unlike paramTypable, responseTypable.Schema() always returns the *body* schema, so the non-body
	// path would not panic but silently corrupt the response body and leave the header untyped.
	//
	// Signal the field-level caller to skip the header with a diagnostic instead.
	// Same rule as parameters.buildFromFieldMap.
	// See go-swagger/go-swagger#2804.
	if typable.In() != inBody {
		return errUnrepresentableHeader
	}

	sch := new(oaispec.Schema)
	typable.Schema().Typed("object", "").AdditionalProperties = &oaispec.SchemaOrBool{
		Schema: sch,
	}

	// The map value renders at respBase/schema/additionalProperties; advance the body cursor so the
	// value's inline props (if any) anchor there.
	defer r.descendBody("additionalProperties")()
	valTypable := schema.NewTypable(sch, typable.Level()+1, r.Ctx.SkipExtensions())
	sb := schema.NewBuilder(r.Ctx, r.Decl)
	if err := sb.Build(
		schema.WithType(ftpe.Elem(), valTypable),
		schema.WithPath(r.bodyPathFor(valTypable)),
	); err != nil {
		return err
	}

	for _, d := range sb.PostDeclarations() {
		r.AppendPostDecl(d)
	}

	return nil
}

func (r *Builder) buildFromFieldInterface(tpe *types.Interface, typable ifaces.SwaggerTypable) error {
	sb := schema.NewBuilder(r.Ctx, r.Decl)
	if err := sb.Build(schema.OptionFor(tpe, typable), schema.WithPath(r.bodyPathFor(typable))); err != nil {
		return err
	}

	for _, d := range sb.PostDeclarations() {
		r.AppendPostDecl(d)
	}

	return nil
}

func (r *Builder) buildFromType(otpe types.Type, resp *oaispec.Response, seen map[string]bool) error {
	switch tpe := otpe.(type) {
	case *types.Pointer:
		return r.buildFromType(tpe.Elem(), resp, seen)
	case *types.Named:
		return r.buildNamedType(tpe, resp, seen)
	case *types.Alias:
		return r.buildAlias(tpe, resp, seen)
	default:
		return fmt.Errorf("anonymous types are currently not supported for responses: %w", ErrResponses)
	}
}

func (r *Builder) buildNamedType(tpe *types.Named, resp *oaispec.Response, seen map[string]bool) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) || resolvers.IsStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a responses section definition: %w", o.Name(), ErrResponses)
	}
	resolvers.MustNotBeABuiltinType(o)

	switch stpe := o.Type().Underlying().(type) {
	case *types.Struct:
		if decl, found := r.Ctx.DeclForType(o.Type()); found {
			return r.buildFromStruct(decl, stpe, resp, seen)
		}
		return r.buildFromStruct(r.Decl, stpe, resp, seen)

	default:
		if decl, found := r.Ctx.DeclForType(o.Type()); found {
			var sch oaispec.Schema
			typable := schema.NewTypable(&sch, 0, r.Ctx.SkipExtensions())

			d := decl.Obj()
			if resolvers.IsStdTime(d) {
				typable.Typed("string", "date-time")
				return nil
			}
			if sfnm, isf := strfmtFromDoc(r.ParseBlocks(decl.Comments)); isf {
				typable.Typed("string", sfnm)
				return nil
			}
			sb := schema.NewBuilder(r.Ctx, decl)
			sb.InferNames()
			if err := sb.Build(schema.OptionFor(tpe.Underlying(), typable), schema.WithPath(r.bodyPathFor(typable))); err != nil {
				return err
			}
			resp.WithSchema(&sch)
			for _, d := range sb.PostDeclarations() {
				r.AppendPostDecl(d)
			}
			return nil
		}
		return fmt.Errorf("responses can only be structs, did you mean for %s to be the response body?: %w", tpe.String(), ErrResponses)
	}
}

func (r *Builder) buildAlias(tpe *types.Alias, resp *oaispec.Response, seen map[string]bool) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) || resolvers.IsStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a responses section definition: %w", o.Name(), ErrResponses)
	}
	resolvers.MustNotBeABuiltinType(o)
	resolvers.MustHaveRightHandSide(tpe)

	// `swagger:response` declares a response, not a model.
	// Neither the alias decl nor any chain link of its backing struct surfaces as a `definitions`
	// entry — the fields of the unaliased target become the response's body / headers.
	//
	// The mode flags only affect alias *use* sites (field / element), not the top-level response-set
	// declaration; TransparentAliases, RefAliases and Default share the same path here.
	//
	// Recursion handles alias chains naturally: buildFromType dispatches back here for any chain link
	// whose RHS is itself an alias.
	// The named-struct target is reached via buildNamedType -> buildFromStruct, the same path a
	// directly-declared swagger:response struct takes.
	return r.buildFromType(tpe.Rhs(), resp, seen)
}

func (r *Builder) buildNamedField(ftpe *types.Named, typable ifaces.SwaggerTypable) error {
	decl, found := r.Ctx.DeclForType(ftpe.Obj().Type())
	if !found {
		return fmt.Errorf("unable to find package and source file for: %s: %w", ftpe.String(), ErrResponses)
	}

	d := decl.Obj()
	if resolvers.IsStdTime(d) {
		typable.Typed("string", "date-time")
		return nil
	}

	if sfnm, isf := strfmtFromDoc(r.ParseBlocks(decl.Comments)); isf {
		typable.Typed("string", sfnm)
		return nil
	}

	sb := schema.NewBuilder(r.Ctx, decl)
	sb.InferNames()
	if err := sb.Build(schema.OptionFor(decl.ObjType(), typable), schema.WithPath(r.bodyPathFor(typable))); err != nil {
		return err
	}

	for _, d := range sb.PostDeclarations() {
		r.AppendPostDecl(d)
	}

	return nil
}

func (r *Builder) buildFieldAlias(tpe *types.Alias, typable ifaces.SwaggerTypable, fld *types.Var, seen map[string]bool) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) {
		// e.g. Field interface{} or Field any
		_ = typable.Schema()

		return nil // just leave an empty schema
	}

	// TransparentAliases supersedes annotation at use sites — dissolve to the unaliased target via
	// the schema sub-builder.
	if r.Ctx.TransparentAliases() {
		sb := schema.NewBuilder(r.Ctx, r.Decl)
		if err := sb.Build(schema.OptionFor(tpe.Rhs(), typable), schema.WithPath(r.bodyPathFor(typable))); err != nil {
			return err
		}
		for _, d := range sb.PostDeclarations() {
			r.AppendPostDecl(d)
		}
		return nil
	}

	// Non-body fields are SimpleSchema targets and cannot carry $ref — always expand the alias to
	// its unaliased target regardless of annotation. types.Unalias collapses chains in one step.
	if typable.In() != inBody {
		return r.buildFromField(fld, types.Unalias(tpe), typable, seen)
	}

	decl, ok := r.Ctx.GetModel(o.Pkg().Path(), o.Name())
	if !ok {
		return fmt.Errorf("can't find source file for aliased type: %v: %w", tpe, ErrResponses)
	}

	// Body field: annotation gates first-class identity at the use site.
	// See [§alias-handling](./README.md#alias-handling) for the cross-builder rule.
	//
	//   - annotated   alias → $ref preserves the alias name; the alias
	//     gets its own definition via MakeRef's AppendPostDecl side
	//     effect.
	//   - unannotated alias → dissolve fully to the unaliased target;
	//     the alias produces no definition entry.
	//
	// The mode flag (RefAliases vs Default) only affects the shape of the alias decl's OWN definition
	// downstream — it does not change the field-site $ref target, which is gated entirely by
	// annotation.
	if decl.HasModelAnnotation() {
		return r.MakeRef(decl, typable)
	}

	return r.buildFromField(fld, types.Unalias(tpe), typable, seen)
}

func (r *Builder) buildFromStruct(decl *scanner.EntityDecl, tpe *types.Struct, resp *oaispec.Response, seen map[string]bool) error {
	if tpe.NumFields() == 0 {
		return nil
	}

	for fld := range tpe.Fields() {
		if fld.Embedded() {
			err := r.buildEmbeddedField(fld, decl, resp, seen)
			if err != nil {
				return nil
			}

			continue
		}

		if fld.Anonymous() {
			continue
		}

		if err := r.processResponseField(fld, decl, resp, seen); err != nil {
			return err
		}
	}

	for k := range resp.Headers {
		if !seen[k] {
			delete(resp.Headers, k)
		}
	}

	return nil
}

func (r *Builder) buildEmbeddedField(fld *types.Var, decl *scanner.EntityDecl, resp *oaispec.Response, seen map[string]bool) error {
	// An in: annotation on the embed applies to the response fields it promotes (go-swagger#2701) —
	// body/header routing.
	// Thread it through the recursion, restoring afterwards so siblings are unaffected.
	saved := r.inherited
	if afld := resolvers.FindASTField(decl.File, fld.Pos()); afld != nil {
		r.inherited = r.ReadEmbedInheritance(afld.Doc, saved)
	}
	// An embed marked `in: body` IS the response body — the embedded struct becomes the body schema,
	// exactly like a named `Body Foo` field, rather than promoting its members (a response has a
	// single body, so per-field promotion is meaningless). go-swagger#1635. Other in: values still
	// promote the embed's fields (#2701).
	if r.inherited.InSet && r.inherited.In == inBody {
		err := r.buildBodyEmbed(fld, resp, seen)
		r.inherited = saved
		if err != nil {
			return err
		}

		return nil
	}

	err := r.buildFromType(fld.Type(), resp, seen)
	r.inherited = saved
	if err != nil {
		return err
	}

	return nil
}

// buildBodyEmbed renders an anonymously-embedded field marked `in: body` as the response body,
// exactly like a named `Body Foo` field: the embedded type drives the body schema (a $ref to a
// model, or its inline shape) instead of its members becoming response headers (go-swagger#1635).
func (r *Builder) buildBodyEmbed(fld *types.Var, resp *oaispec.Response, seen map[string]bool) error {
	var refAttempted bool
	header := oaispec.Header{}
	return r.buildFromField(fld, fld.Type(), responseTypable{
		in:           inBody,
		header:       &header,
		response:     resp,
		skipExt:      r.Ctx.SkipExtensions(),
		refAttempted: &refAttempted,
	}, seen)
}

func (r *Builder) processResponseField(fld *types.Var, decl *scanner.EntityDecl, resp *oaispec.Response, seen map[string]bool) error {
	if !fld.Exported() {
		return nil
	}

	afld := resolvers.FindASTField(decl.File, fld.Pos())
	if afld == nil {
		return nil
	}

	signals := scanFieldDocSignals(r.ParseBlocks(afld.Doc), afld.Doc)

	if signals.ignored {
		return nil
	}

	name, ignore, _, _, err := resolvers.ParseFieldTag(afld, fld.Name(), r.Ctx.NameFromTags())
	if err != nil {
		return err
	}
	if ignore {
		return nil
	}

	// A `name:` keyword renames the response header (the Headers map key), overriding the json-tag /
	// Go-field derivation — the response-side analogue of the same keyword on a swagger:parameters
	// field.
	//
	// Read it before `name` flows into the Headers key / seen set. (Harmless on a body field: the body
	// path below never consults `name`.)
	if kwName, ok := r.ParseBlock(afld.Doc).GetString(grammar.KwName); ok {
		if kwName = strings.TrimSpace(kwName); kwName != "" {
			name = kwName
		}
	}

	// `in:` is the body/header annotation switch (Q1, default header).
	//
	// A field's own in: wins; otherwise an enclosing embed's inherited in: applies (go-swagger#2701);
	// otherwise default header.
	// See [§in-discriminator](./README.md#in-discriminator).
	var in string
	switch {
	case signals.inSet:
		in = signals.in
	case r.inherited.InSet:
		in = r.inherited.In
	default:
		in = inHeader
	}
	if signals.invalidIn != "" {
		r.RecordDiagnostic(grammar.Warnf(
			r.Ctx.PosOf(afld.Pos()),
			grammar.CodeInvalidAnnotation,
			"unrecognised `in: %s` on response field %q (vocabulary: query/path/header/body/formData); defaulting to header",
			signals.invalidIn, name,
		))
	}

	// A swagger:name annotation is inert on a response header — the canonical rename keyword is
	// `name:` (doc-quirk G2).
	// Only the header path consults `name` (a body field becomes resp.Schema), so warn there in case
	// the author meant the keyword; the annotation is dropped either way.
	if in == inHeader {
		for _, b := range r.ParseBlocks(afld.Doc) {
			if b.AnnotationKind() == grammar.AnnName {
				r.RecordDiagnostic(grammar.Warnf(
					r.Ctx.PosOf(afld.Pos()),
					grammar.CodeContextInvalid,
					"swagger:name is ignored on a response header field; use the `name:` keyword to rename header %q",
					name,
				))
				break
			}
		}
	}
	ps := resp.Headers[name]

	// `swagger:file` is body-only (Q3); on a header it would corrupt the body schema.
	// See [§file-body](./README.md#file-body).
	useFileBody := signals.file && in == inBody
	if signals.file && !useFileBody {
		r.RecordDiagnostic(grammar.Warnf(
			r.Ctx.PosOf(afld.Pos()),
			grammar.CodeUnsupportedInSimpleSchema,
			"`swagger:file` is only valid on a body response field (in: body); ignored on response field %q (in=%q). Allowed header types: string/number/integer/boolean/array.",
			name, in,
		))
	}

	if useFileBody {
		resp.Schema = &oaispec.Schema{}
		resp.Schema.Typed("file", "")
	} else {
		var refAttempted bool
		if err := r.buildFromField(fld, fld.Type(), responseTypable{
			in:           in,
			header:       &ps,
			response:     resp,
			skipExt:      r.Ctx.SkipExtensions(),
			refAttempted: &refAttempted,
		}, seen); err != nil {
			if errors.Is(err, errUnrepresentableHeader) {
				// The field type has no OAS v2 SimpleSchema representation in this header (non-body) location
				// (e.g. a map).
				// Record a located diagnostic and skip the header instead of corrupting the response body
				// schema.
				//
				// See go-swagger/go-swagger#2804.
				r.RecordDiagnostic(grammar.Warnf(
					r.Ctx.PosOf(afld.Pos()),
					grammar.CodeUnsupportedInSimpleSchema,
					"response header %q (in=%q) has Go type %s, which has no OAS v2 SimpleSchema representation; header skipped",
					name, in, fld.Type().String(),
				))
				return nil
			}
			return err
		}
	}

	if in == inBody {
		// Body field: schema-level keywords (example/default/validations, strfmt) belong on the body
		// schema.
		// Non-body fields route them through the header, but body responses discard the header, so a body
		// field's `example:` would be lost (go-swagger#3013, same family as #2942).
		//
		// Skip a $ref body — siblings on a $ref are invalid.
		if resp.Schema != nil && resp.Schema.Ref.String() == "" {
			if signals.strfmtSet {
				resp.Schema.Typed("string", signals.strfmt)
			}
			handlers.DispatchSchemaLevel0(
				r.ParseBlock(afld.Doc), nil, resp.Schema, "",
				r.RecordDiagnostic, handlers.SchemaOptions{},
			)
		}
		return nil
	}

	if signals.strfmtSet {
		ps.Typed("string", signals.strfmt)
	}

	r.applyBlockToHeader(afld, &ps)

	seen[name] = true
	if resp.Headers == nil {
		resp.Headers = make(map[string]oaispec.Header)
	}
	resp.Headers[name] = ps

	// Cross-ref linkage: anchor the header to its struct field.
	// The response name is known (respBase set), so this is direct — no deferral.
	// Finer header nodes (validations) resolve to this anchor.
	if r.respBase != "" {
		r.Ctx.RecordOrigin(r.respBase+scanner.JSONPointer("headers", name), r.Ctx.PosOf(afld.Pos()))
	}

	return nil
}
