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

// fileTypeName is the OAS v2 `file` type, spelled as a swagger:type argument.
const fileTypeName = "file"

// Builder constructs OAS v2 response entries for one `swagger:response` declaration.
//
// Embeds *common.Builder for shared state (Ctx, Decl, PostDeclarations, diagnostics, ParseBlocks cache).
type Builder struct {
	*common.Builder

	// inherited carries an embedded field's in: annotation down to the response fields it promotes (go-swagger#2701) —
	// the body/header routing discriminator.
	//
	// Set with save/restore around the embedded- field recursion in buildFromStruct.
	// The mechanism is shared with the schema and parameters builders via common.EmbedInheritance; responses consume only
	// In (OAS2 response headers carry no required).
	inherited common.EmbedInheritance

	// respBase is the cross-ref base pointer for this response — /responses/{name} — set per Build when a provenance
	// sink is wired ("" when off).
	//
	// Header anchors hang at respBase/headers/{h}; the in:body schema under respBase/schema. bodyPath is the live cursor
	// into the body-schema subtree, advanced by descendBody as the responses builder peels its OWN array/map layers
	// (delegated struct/named builds are pathed by the schema builder).
	respBase string
	bodyPath string
}

// NewBuilder constructs an initialized [Builder] bound to ctx and decl.
//
// The embedded common.Builder owns the diagnostic sink, the post-declaration list, and the per-comment-group parse
// cache.
func NewBuilder(ctx *scanner.ScanCtx, decl *scanner.EntityDecl) *Builder {
	return &Builder{
		Builder: common.New(ctx, decl),
	}
}

// ResponseName resolves the spec name of this response declaration from the grammar (grammar.ResponseBlock): the
// explicit `swagger:response {name}` argument when present, else the Go type name (covering the bare `swagger:response`
// and the `swagger:response *` synonym, which both key the response by its type name).
//
// The targeting parse lives in the grammar, not the scanner.
func (r *Builder) ResponseName() string {
	for _, b := range r.ParseBlocks(r.Decl.Comments()) {
		if rb, ok := b.(*grammar.ResponseBlock); ok {
			if rb.Name != "" {
				return rb.Name
			}
			break
		}
	}
	return r.Decl.Name()
}

func (r *Builder) Build(responses map[string]oaispec.Response) error {
	// check if there is a swagger:response tag that is followed by one or more words, these words are the ids of the
	// operations this parameter struct applies to once type name is found convert it to a schema, by looking up the schema
	// in the parameters dictionary that got passed into this parse method.

	name := r.ResponseName()
	response := responses[name]

	// Cross-ref linkage: anchor this response's headers and in:body schema under /responses/{name}.
	// The response name is known here (no deferral, unlike a parameter's array index), so the base path is fixed for the
	// whole build.
	if r.Ctx.OriginEnabled() {
		r.respBase = scanner.JSONPointer("responses", name)
		r.bodyPath = r.respBase + scanner.JSONPointer("schema")
	}

	// analyze doc comment for the model
	r.applyBlockToDecl(&response)

	// analyze struct body for fields etc
	// each exported struct field:
	// - gets a type mapped to a go primitive
	// - perhaps gets a format
	// - has to document the validations that apply for the type and the field
	// - when the struct field points to a model it becomes a ref: #/definitions/ModelName
	// - comments that aren't tags is used as the description
	if err := r.buildFromType(r.Decl.ObjType(), &response, make(map[string]bool)); err != nil {
		return err
	}

	// Carry decl-comment schema keywords (example:, default:, validations) onto a top-level non-struct response body
	// schema. applyBlockToDecl only takes the prose/description; without this, an `example:` on a `swagger:response` whose
	// body is a bare array/scalar type is dropped (go-swagger#3013).
	//
	// Struct responses carry these on their fields, not the decl, and a $ref body must not gain sibling keywords — both
	// skipped.
	if response.Schema != nil && response.Schema.Ref.String() == "" && !underlyingIsStruct(r.Decl.ObjType()) {
		handlers.DispatchSchemaLevel0(
			r.ParseBlock(r.Decl.Comments()), nil, response.Schema, "",
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

// descendBody advances the in:body schema cursor by segs for the duration of a child build, mirroring the schema
// builder's descend.
//
// It keeps bodyPath aligned with the node being filled when the responses builder peels its OWN array/map layers; types
// delegated to the schema sub-builder are pathed there instead.
//
// No-op (and no restore cost) when provenance is off (bodyPath == "").
func (r *Builder) descendBody(segs ...string) func() {
	if r.bodyPath == "" {
		return func() {}
	}
	saved := r.bodyPath
	r.bodyPath = saved + scanner.JSONPointer(segs...)
	return func() { r.bodyPath = saved }
}

// bodyPathFor returns the cross-ref base path to hand a schema sub-build: the live body cursor when the build targets
// the in:body schema, else "" — a header schema anchors at respBase/headers/{h}, not under /schema, so its finer
// nodes resolve to the header anchor rather than emitting a wrong /schema/... pointer.
func (r *Builder) bodyPathFor(typable ifaces.SwaggerTypable) string {
	if typable != nil && typable.In() == inBody {
		return r.bodyPath
	}
	return ""
}

func (r *Builder) buildFromField(fld *types.Var, tpe types.Type, typable ifaces.SwaggerTypable) error {
	switch ftpe := tpe.(type) {
	case *types.Basic:
		return resolvers.SwaggerSchemaForType(ftpe.Name(), typable)
	case *types.Struct:
		return schema.Delegate(r.Builder, schema.OptionFor(ftpe, typable), schema.WithPath(r.bodyPathFor(typable)))
	case *types.Pointer:
		return r.buildFromField(fld, ftpe.Elem(), typable)
	case *types.Interface:
		return schema.Delegate(r.Builder, schema.OptionFor(ftpe, typable), schema.WithPath(r.bodyPathFor(typable)))
	case *types.Array:
		defer r.descendBody("items")()
		return r.buildFromField(fld, ftpe.Elem(), typable.Items())
	case *types.Slice:
		defer r.descendBody("items")()
		return r.buildFromField(fld, ftpe.Elem(), typable.Items())
	case *types.Map:
		return r.buildFromFieldMap(ftpe, typable)
	case *types.Named:
		return r.buildNamedField(ftpe, typable)
	case *types.Alias:
		return r.buildFieldAlias(ftpe, typable)
	default:
		return fmt.Errorf("unknown type for %s: %T: %w", fld.String(), fld.Type(), ErrResponses)
	}
}

func (r *Builder) buildFromFieldMap(ftpe *types.Map, typable ifaces.SwaggerTypable) error {
	// A Go map is only representable under in=body (object + additionalProperties).
	// A response header is an OAS v2 SimpleSchema target with no map representation.
	// Unlike paramTypable, responseTypable.Schema() always returns the *body* schema, so the non-body path would not panic
	// but silently corrupt the response body and leave the header untyped.
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

	// The map value renders at respBase/schema/additionalProperties; advance the body cursor so the value's inline props
	// (if any) anchor there.
	defer r.descendBody("additionalProperties")()
	valTypable := schema.NewTypable(sch, typable.Level()+1, r.Ctx.SkipExtensions())
	return schema.Delegate(r.Builder,
		schema.WithType(ftpe.Elem(), valTypable),
		schema.WithPath(r.bodyPathFor(valTypable)),
	)
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

// namedWrittenRHS reports a declaration whose written right-hand side names a DECLARED type, together with that type.
//
// Only a declared right-hand side redirects the build: a struct literal, a slice or a basic type is already the shape
// the response arm should build, and sending those through the sub-builder would publish the response type as a
// definition.
//
// An alias counts, since it too names a declared type: go1.27 made `encoding/json.RawMessage` an alias of
// jsontext.Value, so `type DefinedRaw json.RawMessage` writes an alias on the right and a gate reading only
// *types.Named stopped redirecting it — the response fell through to the []byte underlying and came out as an array of
// uint8. The alias itself is handed on, so the schema builder applies the alias policy the declaration asks for.
func namedWrittenRHS(ctx *scanner.ScanCtx, o *types.TypeName) (*scanner.EntityDecl, types.Type, bool) {
	decl, found := ctx.DeclForType(o.Type())
	if !found {
		return nil, nil, false
	}
	rhs, ok := writtenRHS(decl)
	if !ok {
		return nil, nil, false
	}
	if _, isNamed := types.Unalias(rhs).(*types.Named); !isNamed {
		return nil, nil, false
	}

	return decl, rhs, true
}

// writtenRHS returns the type a declaration was WRITTEN over — `Stamp` in `type StampResp Stamp` — as opposed to
// the fully peeled underlying that `types.Named.Underlying` yields.
//
// The distinction matters wherever a named layer carries meaning: a stdlib recognizer keys on `time.Time`, which
// peeling discards.
func writtenRHS(decl *scanner.EntityDecl) (types.Type, bool) {
	if decl == nil {
		return nil, false
	}

	return decl.WrittenRHS()
}

func (r *Builder) buildNamedType(tpe *types.Named, resp *oaispec.Response, seen map[string]bool) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) || resolvers.IsStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a responses section definition: %w", o.Name(), ErrResponses)
	}
	resolvers.MustNotBeABuiltinType(o)

	// The canonical recognizers, ahead of the written-RHS redirect and the shape dispatch below — the order the schema
	// builder uses, and for its reason: these answer from the object alone, so subordinating them to a lookup or to a
	// shape test makes a rule that needs nothing depend on one that can fail.
	//
	// The shape test fails here. `time.Time` is a STRUCT underneath, so dispatching on the underlying sends it to
	// the struct arm to have its fields read as response headers — it has none exported, and the response came out with
	// no schema whatsoever. `type Stamp time.Time` escaped that only because the written-RHS redirect below catches it
	// first; the alias spelling `type Stamp = time.Time` arrives here AS time.Time and had nothing to catch it.
	//
	// The refusal above stays in front: `any` and `error` are not responses, and the recognizers would render them
	// instead of refusing them.
	{
		var sch oaispec.Schema
		typable := schema.NewTypable(&sch, 0, r.Ctx.SkipExtensions())
		if schema.ApplyStdlibSpecials(o, typable, r.Ctx.SkipExtensions()) {
			resp.WithSchema(&sch)

			return nil
		}
	}

	// Follow the declaration's WRITTEN right-hand side when that is itself a named type, before dispatching on the
	// underlying shape.
	//
	// `Underlying()` peels every named layer at once, so `type Stamp time.Time` arrives here as time.Time's STRUCT —
	// read as a response struct whose fields become headers, of which time.Time has none, so the response came out with no
	// schema.
	//
	// The schema builder does not have this problem because it builds from `Spec.Type`, where the recognizer sees
	// `time.Time` one level in.
	//
	// Only a NAMED right-hand side redirects: a struct literal, a slice or a basic type is already the shape this arm
	// should build, and sending those through the sub-builder would publish the response type as a definition.
	if decl, rhs, ok := namedWrittenRHS(r.Ctx, o); ok {
		var sch oaispec.Schema
		typable := schema.NewTypable(&sch, 0, r.Ctx.SkipExtensions())
		if err := schema.DelegateAs(r.Builder, decl,
			schema.OptionFor(rhs, typable), schema.WithPath(r.bodyPathFor(typable)),
		); err != nil {
			return err
		}
		resp.WithSchema(&sch)

		return nil
	}

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

			// The declaration's format is applied HERE rather than by the sub-build, and the sub-build is handed the
			// UNDERLYING rather than the declared type — deliberately.
			// A `swagger:response` declares a response, not a model: passing the named type would send it through the $ref
			// machinery and publish it as a definition, which is the one thing this arm must not do.
			//
			// It used to write into `sch` and return WITHOUT the resp.WithSchema below, so a response declared on a named
			// formatted type carried a description and no schema whatsoever.
			//
			// A hand-rolled `IsStdTime` used to sit in front of this, reached through the same declaration lookup. It could
			// never fire: time.Time is a struct underneath, so it takes the struct arm above and never arrives here. The
			// canonical set at the top of this function answers for it now, and for every other recognized
			// type this arm never covered.
			if sfnm, isf := strfmtFromDoc(r.ParseBlocks(decl.Comments())); isf {
				applyDeclFormat(sfnm, tpe.Underlying(), typable)
				resp.WithSchema(&sch)

				return nil
			}

			if err := schema.DelegateAs(r.Builder, decl,
				schema.OptionFor(tpe.Underlying(), typable), schema.WithPath(r.bodyPathFor(typable)),
			); err != nil {
				return err
			}
			resp.WithSchema(&sch)

			return nil
		}
		return fmt.Errorf("responses can only be structs, did you mean for %s to be the response body?: %w", tpe.String(), ErrResponses)
	}
}

// applyDeclFormat writes a declaration's `swagger:strfmt` onto target, honouring the element-driven items-vs-whole rule
// rather than assuming the whole schema.
//
// A byte or rune sequence is string-like and takes the format itself; any other sequence takes it on its items, so a
// `[]string` annotated `email` is a list of email addresses and not one of them.
// The rule itself lives in common, shared with the schema builder's own classifier — this only picks the element to
// hand it.
func applyDeclFormat(format string, underlying types.Type, target ifaces.SwaggerTypable) {
	switch u := underlying.(type) {
	case *types.Slice:
		common.ApplyArrayLikeStrfmt(format, u.Elem(), target)
	case *types.Array:
		common.ApplyArrayLikeStrfmt(format, u.Elem(), target)
	default:
		target.Typed("string", format)
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
	// Neither the alias decl nor any chain link of its backing struct surfaces as a `definitions` entry — the fields of
	// the unaliased target become the response's body / headers.
	//
	// The mode flags only affect alias *use* sites (field / element), not the top-level response-set declaration;
	// TransparentAliases, RefAliases and Default share the same path here.
	//
	// Recursion handles alias chains naturally: buildFromType dispatches back here for any chain link whose RHS is itself
	// an alias.
	// The named-struct target is reached via buildNamedType -> buildFromStruct, the same path a directly-declared
	// swagger:response struct takes.
	return r.buildFromType(tpe.Rhs(), resp, seen)
}

func (r *Builder) buildNamedField(ftpe *types.Named, typable ifaces.SwaggerTypable) error {
	o := ftpe.Obj()

	// The identity recognizers answer from the object alone and so run before the lookup below.
	// This arm had none of them, and the lookup is not a soft gate here: a field typed `error` has no package, so
	// resolving its declaring source dereferenced nil and took the whole scan down.
	if schema.ApplyStdlibSpecials(o, typable, r.Ctx.SkipExtensions()) {
		r.ensureSimpleSchemaTyped(typable, ftpe, o.Name())

		return nil
	}

	decl, found := r.Ctx.DeclForType(o.Type())
	if !found {
		// See the parameters builder's twin: the type is complete even when its declaration is not
		// readable, so it is rendered from its underlying shape rather than costing the document.
		if r.SourcelessFallback(o) {
			return schema.Delegate(r.Builder, schema.WithType(ftpe.Underlying(), typable))
		}

		return fmt.Errorf("unable to find package and source file for: %s: %w", ftpe.String(), ErrResponses)
	}

	// See the parameters builder's twin: the delegation reaches the schema builder's element-aware classifiers, which the
	// local shortcuts that used to sit here were not.
	return schema.DelegateAs(r.Builder, decl,
		schema.OptionFor(decl.ObjType(), typable), schema.WithPath(r.bodyPathFor(typable)),
	)
}

// ensureSimpleSchemaTyped repairs a response header that resolved to no type, and reports the choice
// made on the author's behalf.
//
// The parameters builder's twin: the recognizers answering in this builder's own field arms return
// without entering the schema builder's Build, so the catch-at-exit contract never sees those
// targets. A header cannot spell "any JSON" any more than a query parameter can.
func (r *Builder) ensureSimpleSchemaTyped(typable ifaces.SwaggerTypable, tpe types.Type, goName string) {
	if !schema.EnsureSimpleSchemaTyped(typable, tpe, r.Ctx.SkipExtensions()) {
		return
	}

	r.RecordDiagnostic(grammar.Warnf(
		r.Ctx.PosOf(r.Decl.Pos()),
		grammar.CodeUnderspecifiedInSimpleSchema,
		"a response header typed %s resolved to no type, which OAS v2 does not allow on a header; "+
			"defaulted to {type: string}",
		goName,
	))
}

func (r *Builder) buildFieldAlias(tpe *types.Alias, typable ifaces.SwaggerTypable) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) {
		// e.g. Field interface{} or Field any
		_ = typable.Schema()
		r.ensureSimpleSchemaTyped(typable, tpe, o.Name())

		return nil // an empty schema where the position can hold one
	}

	// Shared with the parameters builder — see schema.BuildFieldAlias.
	// The cross-ref path is the one thing genuinely ours: a header anchors at respBase/headers/{h}, not under /schema.
	return schema.BuildFieldAlias(r.Builder, tpe, typable, func() error {
		return fmt.Errorf("can't find source file for aliased type: %v: %w", tpe, ErrResponses)
	}, schema.WithPath(r.bodyPathFor(typable)))
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
	// An in: annotation on the embed applies to the response fields it promotes (go-swagger#2701) — body/header routing.
	// Thread it through the recursion, restoring afterwards so siblings are unaffected.
	saved := r.inherited
	if afld := resolvers.FindASTFieldFor(decl.File(), fld, r.Ctx.PosOf); afld != nil {
		r.inherited = r.ReadEmbedInheritance(afld.Doc, saved)
	}
	// An embed marked `in: body` IS the response body — the embedded struct becomes the body schema, exactly like a
	// named `Body Foo` field, rather than promoting its members (a response has a single body, so per-field promotion is
	// meaningless). go-swagger#1635. Other in: values still promote the embed's fields (#2701).
	if r.inherited.InSet && r.inherited.In == inBody {
		err := r.buildBodyEmbed(fld, resp)
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

// buildBodyEmbed renders an anonymously-embedded field marked `in: body` as the response body, exactly like a named
// `Body Foo` field: the embedded type drives the body schema (a $ref to a model, or its inline shape) instead of its
// members becoming response headers (go-swagger#1635).
func (r *Builder) buildBodyEmbed(fld *types.Var, resp *oaispec.Response) error {
	var refAttempted bool
	header := oaispec.Header{}
	return r.buildFromField(fld, fld.Type(), responseTypable{
		in:           inBody,
		header:       &header,
		response:     resp,
		skipExt:      r.Ctx.SkipExtensions(),
		refAttempted: &refAttempted,
	})
}

func (r *Builder) processResponseField(fld *types.Var, decl *scanner.EntityDecl, resp *oaispec.Response, seen map[string]bool) error {
	if !fld.Exported() {
		return nil
	}

	afld := resolvers.FindASTFieldFor(decl.File(), fld, r.Ctx.PosOf)
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

	// A `name:` keyword renames the response header (the Headers map key), overriding the json-tag / Go-field derivation
	// — the response-side analogue of the same keyword on a swagger:parameters field.
	//
	// Read it before `name` flows into the Headers key / seen set. (Harmless on a body field: the body path below never
	// consults `name`.)
	if kwName, ok := r.ParseBlock(afld.Doc).GetString(grammar.KwName); ok {
		if kwName = strings.TrimSpace(kwName); kwName != "" {
			name = kwName
		}
	}

	// `in:` is the body/header annotation switch.
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

	// A swagger:name annotation is inert on a response header — the canonical rename keyword is `name:`.
	//
	// Only the header path consults `name` (a body field becomes resp.Schema), so warn there in case the author meant the
	// keyword; the annotation is dropped either way.
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

	// `swagger:file` is body-only ; on a header it would corrupt the body schema.
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
		}); err != nil {
			if errors.Is(err, errUnrepresentableHeader) {
				// The field type has no OAS v2 SimpleSchema representation in this header (non-body) location (e.g. a map).
				// Record a located diagnostic and skip the header instead of corrupting the response body schema.
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
		// Body field: schema-level keywords (example/default/validations, strfmt) belong on the body schema.
		// Non-body fields route them through the header, but body responses discard the header, so a body field's `example:`
		// would be lost (go-swagger#3013, same family as #2942).
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
