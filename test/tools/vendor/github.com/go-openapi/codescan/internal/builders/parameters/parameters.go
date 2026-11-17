// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parameters

import (
	"errors"
	"fmt"
	"go/types"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/common"
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/builders/schema"
	"github.com/go-openapi/codescan/internal/builders/validations"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

const inBody = "body"

// fileTypeName is the OAS v2 `file` type, spelled as a swagger:type argument.
const fileTypeName = "file"

// Builder constructs OAS v2 parameter entries for one `swagger:parameters` declaration and writes them onto the
// matching operations.
//
// Embeds *common.Builder for shared state (Ctx, Decl, PostDeclarations, diagnostics, ParseBlocks cache).
type Builder struct {
	*common.Builder

	// inherited carries an embedded field's in:/required: annotation down into the parameters it promotes
	// (go-swagger#2701).
	//
	// The zero value means no inheritance (top-level / non-embedded path).
	// Set with save/restore around the embedded-field recursion in buildFromStruct.
	// The mechanism is shared with the schema and responses builders via common.EmbedInheritance.
	inherited common.EmbedInheritance

	// currentOpID is the operation id whose parameter set is being built.
	//
	// Set per-iteration in Build and read by processParamField to key the deferred cross-ref anchor capture.
	// The same swagger:parameters struct may apply to several operations, so the capture runs once per op id.
	//
	// Empty while building a shared (`*`) parameter set — there is no operation context.
	currentOpID string

	// shared accumulates the parameters this struct registers at the spec top level (`swagger:parameters *`), keyed by
	// resolved parameter name. nil unless a shared marker was built.
	//
	// Exposed via SharedParameters.
	shared map[string]oaispec.Parameter

	// sharedRefOps are the operation ids a `swagger:parameters * opid …` marker references the struct's shared
	// parameters into.
	//
	// The $ref wiring is applied by the spec builder.
	// Exposed via SharedRefOperations.
	sharedRefOps []string

	// pathItems holds the parameters a `swagger:parameters /path` marker inlines into a path-item, keyed by exact path.
	// nil unless a path marker was built.
	//
	// Exposed via PathItemParameters.
	pathItems map[string][]oaispec.Parameter
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

func (p *Builder) Build(operations map[string]*oaispec.Operation) error {
	// A swagger:parameters struct may carry several markers, each parsed by the grammar into a ParametersBlock
	// with a target (grammar owns the targeting parse).
	//
	// Dispatch per target:
	//
	//   - operations: inline the struct's fields into each named operation (the historical behaviour).
	//   - shared (`*`): register the fields at the spec top level
	//     (#/parameters/{name}); harvested here, merged + conflict-checked by the spec builder via SharedParameters().
	//   - path (`/path`): path-item parameters — handled in a later phase.
	for _, pb := range p.parametersBlocks() {
		p.warnDuplicateTargets(pb)
		switch pb.Target {
		case grammar.ParamTargetOperations:
			if err := p.buildIntoOperations(pb.OperationIDs(), operations); err != nil {
				return err
			}
		case grammar.ParamTargetShared:
			if err := p.buildShared(); err != nil {
				return err
			}
			// A `swagger:parameters * opid …` marker also references the struct's shared parameters into the listed
			// operations; the $ref wiring is applied by the spec builder after the shared map is complete (see
			// SharedRefOperations).
			p.sharedRefOps = append(p.sharedRefOps, pb.Args...)
		case grammar.ParamTargetPath:
			// `swagger:parameters /path` on a struct inlines the struct's fields into the path-item.
			// Harvested here; the spec builder applies them once paths are built (PathItemParameters).
			if err := p.buildPathItem(pb.Path); err != nil {
				return err
			}
		default:
			// ParametersTarget is a closed set produced by the grammar; every member is handled above.
			// A new member must add its case.
		}
	}

	return nil
}

// SharedParameters returns the parameters this struct registers at the spec top level (`swagger:parameters *`), keyed
// by resolved parameter name.
//
// Empty unless the struct carried a shared (`*`) marker.
// The spec builder merges these into the single #/parameters map with keep-first conflict handling.
// Valid after Build.
func (p *Builder) SharedParameters() map[string]oaispec.Parameter {
	return p.shared
}

// SharedRefOperations returns the operation ids that a `swagger:parameters * opid …` marker references this struct's
// shared parameters into (as #/parameters/{name} $refs).
//
// Empty unless such a marker was present.
// The spec builder applies the $ref wiring once the shared map is complete.
// Valid after Build.
func (p *Builder) SharedRefOperations() []string {
	return p.sharedRefOps
}

// PathItemParameters returns the parameters this struct inlines into path-items via `swagger:parameters /path`, keyed
// by exact path, in field order.
//
// Empty unless a path marker was present.
// The spec builder applies them once paths are built.
// Valid after Build.
func (p *Builder) PathItemParameters() map[string][]oaispec.Parameter {
	return p.pathItems
}

// warnDuplicateTargets emits a duplicate-target warning (C1) for each argument token the grammar dropped as a duplicate
// on a definition marker (operations / shared targets).
func (p *Builder) warnDuplicateTargets(pb *grammar.ParametersBlock) {
	for _, dup := range pb.Dups {
		p.RecordDiagnostic(grammar.Warnf(pb.Pos(), grammar.CodeDuplicateTarget,
			"swagger:parameters: duplicate target %q dropped", dup))
	}
}

// parametersBlocks returns every grammar.ParametersBlock attached to the decl, in source order
// (a struct may carry several swagger:parameters lines).
func (p *Builder) parametersBlocks() []*grammar.ParametersBlock {
	var out []*grammar.ParametersBlock
	for _, b := range p.ParseBlocks(p.Decl.Comments()) {
		if pb, ok := b.(*grammar.ParametersBlock); ok {
			out = append(out, pb)
		}
	}
	return out
}

// buildIntoOperations inlines the struct's fields into each named operation, creating the operation entry when absent.
func (p *Builder) buildIntoOperations(opids []string, operations map[string]*oaispec.Operation) error {
	for _, opid := range opids {
		operation, ok := operations[opid]
		if !ok {
			operation = new(oaispec.Operation)
			operations[opid] = operation
			operation.ID = opid
		}
		p.currentOpID = opid

		// analyze struct body for fields etc
		// each exported struct field:
		// - gets a type mapped to a go primitive
		// - perhaps gets a format
		// - has to document the validations that apply for the type and the field
		// - when the struct field points to a model it becomes a ref: #/definitions/ModelName
		// - comments that aren't tags is used as the description
		if err := p.buildFromType(p.Decl.ObjType(), operation, make(map[string]oaispec.Parameter)); err != nil {
			return err
		}
	}

	return nil
}

// buildShared builds the struct's fields as a free-standing parameter set and harvests them, keyed by the resolved
// parameter name (the overridden name when `name:` / NameFromTags applies — C3), for top-level registration.
//
// Reuses the full field-building path by building into a throwaway operation. currentOpID is left empty so no
// operation-path cross-ref anchor is recorded (there is no operation here).
func (p *Builder) buildShared() error {
	p.currentOpID = ""
	tmp := new(oaispec.Operation)
	if err := p.buildFromType(p.Decl.ObjType(), tmp, make(map[string]oaispec.Parameter)); err != nil {
		return err
	}

	if p.shared == nil {
		p.shared = make(map[string]oaispec.Parameter, len(tmp.Parameters))
	}
	for _, prm := range tmp.Parameters {
		p.shared[prm.Name] = prm
	}

	return nil
}

// buildPathItem builds the struct's fields as an ordered parameter set for inlining into the given path-item.
//
// Reuses the full field-building path by building into a throwaway operation; currentOpID is left empty (no
// operation-path cross-ref anchor — path-item parameters are not under an operation).
func (p *Builder) buildPathItem(path string) error {
	p.currentOpID = ""
	tmp := new(oaispec.Operation)
	if err := p.buildFromType(p.Decl.ObjType(), tmp, make(map[string]oaispec.Parameter)); err != nil {
		return err
	}

	if p.pathItems == nil {
		p.pathItems = make(map[string][]oaispec.Parameter)
	}
	p.pathItems[path] = append(p.pathItems[path], tmp.Parameters...)

	return nil
}

func (p *Builder) buildFromType(otpe types.Type, op *oaispec.Operation, seen map[string]oaispec.Parameter) error {
	switch tpe := otpe.(type) {
	case *types.Pointer:
		return p.buildFromType(tpe.Elem(), op, seen)
	case *types.Named:
		return p.buildNamedType(tpe, op, seen)
	case *types.Alias:
		return p.buildAlias(tpe, op, seen)
	default:
		return fmt.Errorf("unhandled type (%T): %s: %w", otpe, tpe.String(), ErrParameters)
	}
}

func (p *Builder) buildNamedType(tpe *types.Named, op *oaispec.Operation, seen map[string]oaispec.Parameter) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) || resolvers.IsStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a parameters section definition: %w", o.Name(), ErrParameters)
	}
	resolvers.MustNotBeABuiltinType(o)

	switch stpe := o.Type().Underlying().(type) {
	case *types.Struct:
		if decl, found := p.Ctx.DeclForType(o.Type()); found {
			return p.buildFromStruct(decl, stpe, op, seen)
		}

		return p.buildFromStruct(p.Decl, stpe, op, seen)
	default:
		return fmt.Errorf("unhandled type (%T): %s: %w", stpe, o.Type().Underlying().String(), ErrParameters)
	}
}

func (p *Builder) buildAlias(tpe *types.Alias, op *oaispec.Operation, seen map[string]oaispec.Parameter) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) || resolvers.IsStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a parameters section definition: %w", o.Name(), ErrParameters)
	}
	resolvers.MustNotBeABuiltinType(o)
	resolvers.MustHaveRightHandSide(tpe)

	// `swagger:parameters` declares a parameter SET, not a model.
	// Neither the alias decl nor any chain link of its target surfaces as a `definitions` entry — the fields of the
	// unaliased target become the operation's parameters.
	//
	// There is no mode-specific behaviour for this case: TransparentAliases takes the same path as Default and RefAliases.
	// The mode flags only affect alias *use* sites (field / element), not the top-level parameter-set declaration.
	//
	// Recursion handles alias chains naturally: buildFromType dispatches back here for any chain link whose RHS is itself
	// an alias.
	return p.buildFromType(tpe.Rhs(), op, seen)
}

func (p *Builder) buildFromField(fld *types.Var, tpe types.Type, typable ifaces.SwaggerTypable) error {
	switch ftpe := tpe.(type) {
	case *types.Basic:
		return resolvers.SwaggerSchemaForType(ftpe.Name(), typable)
	case *types.Struct:
		return schema.Delegate(p.Builder, schema.OptionFor(ftpe, typable))
	case *types.Pointer:
		return p.buildFromField(fld, ftpe.Elem(), typable)
	case *types.Interface:
		return schema.Delegate(p.Builder, schema.OptionFor(ftpe, typable))
	case *types.Array:
		return p.buildFromField(fld, ftpe.Elem(), typable.Items())
	case *types.Slice:
		return p.buildFromField(fld, ftpe.Elem(), typable.Items())
	case *types.Map:
		return p.buildFromFieldMap(ftpe, typable)
	case *types.Named:
		return p.buildNamedField(ftpe, typable)
	case *types.Alias:
		return p.buildFieldAlias(ftpe, typable)
	default:
		return fmt.Errorf("unknown type for %s: %T: %w", fld.String(), fld.Type(), ErrParameters)
	}
}

func (p *Builder) buildFromFieldMap(ftpe *types.Map, typable ifaces.SwaggerTypable) error {
	// A Go map is only representable under in=body (object + additionalProperties).
	// In any OAS v2 SimpleSchema location (query/formData/path/header) it has no representation: paramTypable (and
	// ItemsTypable) return a nil schema there, so dereferencing it would panic (go-swagger#2804).
	//
	// Signal the field-level caller to skip the field with a diagnostic instead.
	// Same rule as responses.buildFromFieldMap for SimpleSchema response headers.
	if typable.In() != inBody {
		return errUnrepresentableParam
	}

	sch := new(oaispec.Schema)
	typable.Schema().Typed("object", "").AdditionalProperties = &oaispec.SchemaOrBool{
		Schema: sch,
	}

	return schema.Delegate(p.Builder, schema.WithType(
		ftpe.Elem(),
		schema.NewTypable(sch, typable.Level()+1, p.Ctx.SkipExtensions()),
	))
}

func (p *Builder) buildNamedField(ftpe *types.Named, typable ifaces.SwaggerTypable) error {
	o := ftpe.Obj()
	if resolvers.IsStdErrorType(ftpe) {
		// An `error` has no meaning as a parameter, and the schema builder's rendering of it (`{type: string}`) would be a
		// lie about what a client should send.
		// Dropping the field is the right outcome — a struct shared between a parameter set and a response should lose it
		// on the parameter side rather than carry it.
		//
		// It used to abort the whole scan.
		// Skip-with-a-diagnostic is the house rule, and its sibling two arms down already follows it for a Go type with no
		// SimpleSchema form (go-swagger#2804); this guard predates that and never got the same treatment.
		//
		// This is the ONE recognizer where a parameter must not answer as the other two builders do, so it sits above the
		// canonical set rather than inside it.
		return errNotAParameter
	}
	resolvers.MustNotBeABuiltinType(o)

	// The rest of the identity recognizers answer from the object alone, so they run before the lookup rather than after
	// it.
	// The subset hand-rolled here was `any` only — which is unreachable in this arm, since the predeclared `any` is an
	// alias and lands in the one below.
	if schema.ApplyStdlibSpecials(o, typable, p.Ctx.SkipExtensions()) {
		p.ensureSimpleSchemaTyped(typable, ftpe, o.Name())

		return nil
	}

	decl, found := p.Ctx.DeclForType(o.Type())
	if !found {
		// No declaration to read, but the type is complete: render what it is rather than what it was
		// called. time.Duration lands here and comes out as its underlying int64, which is the same answer
		// a readable declaration produces for a parameter — the declaration was only ever going to add
		// prose.
		if p.SourcelessFallback(o) {
			return schema.Delegate(p.Builder, schema.WithType(ftpe.Underlying(), typable))
		}

		return fmt.Errorf("unable to find package and source file for: %s: %w", ftpe.String(), ErrParameters)
	}

	// No local recognizer or format short-circuit here: the delegation below reaches the schema builder's own, which are
	// element-aware.
	// The shortcuts that used to sit here wrote `Typed("string", format)` unconditionally, so a format on a `[]string`
	// landed on the whole schema instead of on its items — describing a list of email addresses as one email address.
	return schema.DelegateAs(p.Builder, decl, schema.OptionFor(decl.ObjType(), typable))
}

// ensureSimpleSchemaTyped repairs a non-body parameter that resolved to no type, and reports the
// choice made on the author's behalf.
//
// The recognizers answering in this builder's own field arms return without ever entering the schema
// builder's Build, so the catch-at-exit contract never sees those targets. `any` and
// `json.RawMessage` resolve to "any JSON", which OAS v2 gives a non-body parameter no way to spell.
func (p *Builder) ensureSimpleSchemaTyped(typable ifaces.SwaggerTypable, tpe types.Type, goName string) {
	if !schema.EnsureSimpleSchemaTyped(typable, tpe, p.Ctx.SkipExtensions()) {
		return
	}

	p.RecordDiagnostic(grammar.Warnf(
		p.Ctx.PosOf(p.Decl.Pos()),
		grammar.CodeUnderspecifiedInSimpleSchema,
		"a parameter (in=%q) typed %s resolved to no type, which OAS v2 does not allow on a non-body "+
			"parameter; defaulted to {type: string}",
		typable.In(), goName,
	))
}

func (p *Builder) buildFieldAlias(tpe *types.Alias, typable ifaces.SwaggerTypable) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) {
		// e.g. Field interface{} or Field any
		_ = typable.Schema()
		p.ensureSimpleSchemaTyped(typable, tpe, o.Name())

		return nil // an empty schema where the position can hold one
	}
	if resolvers.IsStdErrorType(tpe) {
		// Same refusal as the named arm, and it has to be spelled against the resolved type: an alias's own object is the
		// alias name, so the object-level recognizer that used to sit here could never fire and `type Wrapped = error` was
		// emitted as a parameter while a bare `error` was dropped.
		//
		// It also aborted the entire scan where its twin skips the field.
		return errNotAParameter
	}
	resolvers.MustNotBeABuiltinType(o)

	// The resolution itself is shared with the responses builder: every classifier an alias declaration may carry lives in
	// the schema package, and two copies of the walk drifted apart three times before this.
	return schema.BuildFieldAlias(p.Builder, tpe, typable, func() error {
		return fmt.Errorf("can't find source file for aliased type: %v -> %v: %w", tpe, tpe.Rhs(), ErrParameters)
	})
}

func (p *Builder) buildFromStruct(decl *scanner.EntityDecl, tpe *types.Struct, op *oaispec.Operation, seen map[string]oaispec.Parameter) error {
	numFields := tpe.NumFields()

	if numFields == 0 {
		return nil
	}

	sequence := make([]string, 0, numFields)
	for fld := range tpe.Fields() {
		if fld.Embedded() {
			var err error
			sequence, err = p.buildEmbeddedField(fld, decl, op, sequence, seen)
			if err != nil {
				return nil
			}

			continue
		}

		name, err := p.processParamField(fld, decl, seen)
		if err != nil {
			return err
		}

		if name != "" {
			sequence = append(sequence, name)
		}
	}

	for _, k := range sequence {
		p := seen[k]
		for i, v := range op.Parameters {
			if v.Name == k {
				op.Parameters = append(op.Parameters[:i], op.Parameters[i+1:]...)
				break
			}
		}
		op.Parameters = append(op.Parameters, p)
	}

	return nil
}

func (p *Builder) buildEmbeddedField(fld *types.Var, decl *scanner.EntityDecl, op *oaispec.Operation, sequence []string, seen map[string]oaispec.Parameter) ([]string, error) {
	// An in:/required: annotation on the embed itself applies to the parameters it promotes (go-swagger#2701).
	// Thread it through the recursion as inherited context, restoring afterwards so sibling fields are unaffected.
	saved := p.inherited
	if afld := resolvers.FindASTFieldFor(decl.File(), fld, p.Ctx.PosOf); afld != nil {
		p.inherited = p.ReadEmbedInheritance(afld.Doc, saved)
	}
	// An embed marked `in: body` IS the body parameter — the embedded struct becomes one body param's schema, exactly
	// like a named `Body Foo` field, rather than promoting its members as N separate body params (an operation allows at
	// most one body parameter, so per-field promotion produces an invalid spec). go-swagger#1635; the parameters
	// counterpart of the responses in: body embed.
	//
	// Other in: values still promote the embed's fields (#2701).
	if p.inherited.InSet && p.inherited.In == inBody {
		name, err := p.processParamField(fld, decl, seen)
		p.inherited = saved
		if err != nil {
			return nil, err
		}

		if name != "" {
			sequence = append(sequence, name)
		}

		return sequence, nil
	}

	err := p.buildFromType(fld.Type(), op, seen)
	p.inherited = saved
	if err != nil {
		return nil, err
	}

	return sequence, nil
}

// applyTypeOverride honours a field-level `swagger:type` on a parameter (go-swagger#1499).
//
// The override always produces an inline SimpleSchema and wins outright over the field's Go type.
// Only what a parameter can represent is accepted: a scalar / Go-builtin base, optionally wrapped in `[]` array layers.
//
// `inline`, `file`, and type-name references have no SimpleSchema representation — they (and any unknown token) are
// rejected with a located diagnostic and the caller falls back to Go-type resolution.
//
// Unlike the schema builder's resolveTypeOverride, this never recurses into a Go struct (which would dereference the
// nil SimpleSchema schema of a non-body param), so it is panic-safe for every parameter location.
func (p *Builder) applyTypeOverride(arg string, typable ifaces.SwaggerTypable, fld *types.Var) bool {
	base, depth := stripArrayPrefixes(arg)

	target := typable
	for range depth {
		target.Typed("array", "")
		target = target.Items()
	}

	if err := resolvers.SwaggerSchemaForType(base, target); err != nil {
		p.RecordDiagnostic(grammar.Warnf(
			p.Ctx.PosOf(fld.Pos()),
			grammar.CodeUnsupportedType,
			"swagger:type %q has no SimpleSchema representation on parameter %q; override ignored",
			arg, fld.Name(),
		))
		return false
	}

	return true
}

// stripArrayPrefixes counts leading `[]` prefixes on a swagger:type argument and returns the bare base plus the array
// depth.
//
// `[]string` → ("string", 1), `int64` → ("int64", 0).
//
// Mirrors the schema builder's identically named unexported helper; kept local to avoid widening the schema package
// surface.
func stripArrayPrefixes(arg string) (base string, depth int) {
	base = strings.TrimSpace(arg)
	for strings.HasPrefix(base, "[]") {
		base = strings.TrimSpace(base[2:])
		depth++
	}
	return base, depth
}

// resolveParamType resolves the parameter's type onto pty in precedence order: a formData file field, then a
// field-level swagger:type override (go-swagger#1499), then the field's own Go type.
//
// Returns skip=true (with a recorded diagnostic) when the Go type has no OAS v2 SimpleSchema representation in this
// location and the field should be dropped.
func (p *Builder) resolveParamType(signals fieldDocSignals, fld *types.Var, name, in string, pty ifaces.SwaggerTypable) (skip bool, err error) {
	switch {
	case in == "formData" && signals.file:
		pty.Typed("file", "")
	case signals.swTypeSet && p.applyTypeOverride(signals.swaggerType, pty, fld):
		// A field-level swagger:type overrides the Go type for the parameter (go-swagger#1499) — the SimpleSchema analogue
		// of the schema builder's field-level override.
		// The override wins outright; the Go type is not consulted.
		// A compatible swagger:strfmt then rides as a supplementary format back in processParamField.
	default:
		if err := p.buildFromField(fld, fld.Type(), pty); err != nil {
			// Both sentinels mean "skip the field rather than panic or fail the whole scan"; they differ only in what the author
			// is told.
			switch {
			case errors.Is(err, errUnrepresentableParam):
				// The field type has no OAS v2 SimpleSchema representation in this non-body location (e.g. a map under in=query).
				// Naming the location is the point — the same type is fine in a body.
				//
				// See go-swagger/go-swagger#2804.
				p.RecordDiagnostic(grammar.Warnf(
					p.Ctx.PosOf(fld.Pos()),
					grammar.CodeUnsupportedInSimpleSchema,
					"parameter %q (in=%q) has Go type %s, which has no OAS v2 SimpleSchema representation; parameter skipped",
					name, in, fld.Type().String(),
				))

				return true, nil
			case errors.Is(err, errNotAParameter):
				// Meaningless in every location, so the message deliberately does NOT suggest that another `in:` would work.
				p.RecordDiagnostic(grammar.Warnf(
					p.Ctx.PosOf(fld.Pos()),
					grammar.CodeUnsupportedGoType,
					"parameter %q has Go type %s, which describes an outcome rather than a value a client can send; parameter skipped",
					name, fld.Type().String(),
				))

				return true, nil
			}

			return false, err
		}
	}

	return false, nil
}

// processParamField processes a single non-embedded struct field for parameter building.
//
// Returns the parameter name if the field was processed, or "" if it was skipped.
func (p *Builder) processParamField(fld *types.Var, decl *scanner.EntityDecl, seen map[string]oaispec.Parameter) (string, error) {
	if !fld.Exported() {
		return "", nil
	}

	afld := resolvers.FindASTFieldFor(decl.File(), fld, p.Ctx.PosOf)
	if afld == nil {
		return "", nil
	}

	signals := scanFieldDocSignals(p.ParseBlocks(afld.Doc), afld.Doc)

	if signals.ignored {
		return "", nil
	}

	name, ignore, _, _, err := resolvers.ParseFieldTag(afld, fld.Name(), p.Ctx.NameFromTags())
	if err != nil {
		return "", err
	}
	if ignore {
		return "", nil
	}

	// A `name:` keyword on the field renames the JSON parameter name, overriding the json-tag / Go-field derivation (the
	// parameter-side analogue of swagger:name on a schema field).
	//
	// Read it before `name` flows into the `seen` key, ps.Name, the sequence and the dedup so the rename is applied
	// consistently. applyFieldCarrier-style x-go-name tracking below records the Go field name when it differs.
	if kwName, ok := p.ParseBlock(afld.Doc).GetString(grammar.KwName); ok {
		if kwName = strings.TrimSpace(kwName); kwName != "" {
			name = kwName
		}
	}

	// A swagger:name annotation is inert in a parameter context — the canonical rename keyword here is `name:`
	// (doc-quirk G2).
	// It is dropped rather than applied, so warn in case the author reached for the schema annotation when they meant the
	// keyword.
	for _, b := range p.ParseBlocks(afld.Doc) {
		if b.AnnotationKind() == grammar.AnnName {
			p.RecordDiagnostic(grammar.Warnf(
				p.Ctx.PosOf(afld.Pos()),
				grammar.CodeContextInvalid,
				"swagger:name is ignored on a parameter field; use the `name:` keyword to rename parameter %q",
				name,
			))
			break
		}
	}

	// Cross-ref linkage: capture the field's position keyed by (opid, name) for the spec builder's deferred
	// /paths/.../parameters/{i} anchor pass.
	// Skipped for shared (`*`) parameters (empty opID) — they have no operation path.
	if p.Ctx.OriginEnabled() && p.currentOpID != "" {
		p.Ctx.RecordParamOrigin(p.currentOpID, name, p.Ctx.PosOf(afld.Pos()))
	}

	in := "query"
	switch {
	case signals.inSet:
		in = signals.in
	case p.inherited.InSet:
		// in: from an embedding field (go-swagger#2701).
		in = p.inherited.In
	}

	ps := seen[name]
	ps.In = in
	var pty ifaces.SwaggerTypable = paramTypable{&ps, p.Ctx.SkipExtensions()}
	if in == inBody {
		pty = schema.NewTypable(pty.Schema(), 0, p.Ctx.SkipExtensions())
	}

	if skip, err := p.resolveParamType(signals, fld, name, in, pty); err != nil {
		return "", err
	} else if skip {
		return "", nil
	}

	if signals.strfmtSet {
		if signals.swTypeSet {
			// swagger:type already fixed the type axis (go-swagger#1499); swagger:strfmt is supplementary and applies as a
			// format only when compatible with the resolved type, mirroring the schema builder's swagger:type + swagger:strfmt
			// precedence.
			// An incompatible format is dropped rather than overriding the type.
			if ok, _ := validations.IsFormatCompatible(ps.Type, signals.strfmt); ok {
				ps.Format = signals.strfmt
			}
		} else {
			ps.Typed("string", signals.strfmt)
			ps.Ref = oaispec.Ref{}
			ps.Items = nil
		}
	}

	_, fieldSetRequired := p.ParseBlock(afld.Doc).GetBool(grammar.KwRequired)
	if err := p.applyBlockToField(afld, &ps); err != nil {
		return "", err
	}
	if ps.In == "path" {
		ps.Required = true
	}
	// required: from an embedding field (go-swagger#2701), unless the promoted field set its own required: explicitly.
	if !fieldSetRequired && p.inherited.RequiredSet && p.inherited.Required {
		ps.Required = true
	}

	if ps.Name == "" {
		ps.Name = name
	}

	if name != fld.Name() {
		resolvers.AddExtension(&ps.VendorExtensible, "x-go-name", fld.Name(), p.Ctx.SkipExtensions())
	}

	seen[name] = ps
	return name, nil
}
