// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/token"
	"go/types"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/builders/validations"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// refuseUnrepresentableOverride reports whether a `swagger:type` base must be IGNORED because the
// location cannot carry what it resolves to.
//
// A non-body parameter or a response header is an OAS-2 SimpleSchema, where `type` is mandatory and
// restricted to {string, number, integer, boolean, array} (plus `file`, formData-only). An override
// resolving to an object has nowhere to go there.
//
// Ignoring beats applying: `type` is required, so the Go-derived type must survive. The alternative
// outcomes were both invalid — a bare `object` base reached the target and was reset to `{}` by
// validateSimpleSchemaOutcome, leaving the parameter untyped, while a type-name reference resolved
// into paramTypable.Schema()'s nil and vanished with no diagnostic at all, the check never firing
// because the illegal value never materialised to be checked.
//
// Side-effect free: it inspects, it does not build.
func (s *Builder) refuseUnrepresentableOverride(arg string, ownType types.Type, pos token.Position) bool {
	if !s.simpleSchema {
		return false
	}

	base, depth := stripArrayPrefixes(arg)
	if depth > 0 {
		// `[]T` is an array, which SimpleSchema allows — but its items are a SimpleSchema too, so the
		// element carries the same restriction.
		return s.overrideYieldsObject(base, ownType)
	}
	if !s.overrideYieldsObject(base, ownType) {
		return false
	}

	s.RecordDiagnostic(grammar.Warnf(pos, grammar.CodeUnsupportedInSimpleSchema,
		"swagger:type %s resolves to an object, which an OAS v2 SimpleSchema (non-body parameter or "+
			"response header, in=%q) cannot carry; the annotation is ignored and the Go type is used "+
			"instead, since SimpleSchema requires a type", arg, s.paramIn))

	return true
}

// overrideYieldsObject reports whether a swagger:type base resolves to an object-shaped schema.
//
// The four base kinds are answered without building: the `inline`/`array` keywords inline the
// annotated Go type, a recognised scalar name is never an object except `object` itself, and
// anything else is a reference to a scanned type whose underlying decides.
func (s *Builder) overrideYieldsObject(base string, ownType types.Type) bool {
	switch base {
	case keywordInline, keywordArray:
		return ownType != nil && isObjectLikeUnderlying(ownType)
	case keywordFile:
		return false // refused earlier, with its own diagnostic
	case "object":
		return true
	}

	// A recognised scalar / Go-builtin / OAS-2 name resolves without touching the scanned packages.
	var probe oaispec.Schema
	if err := resolvers.SwaggerSchemaForType(base, NewTypable(&probe, 0, true)); err == nil {
		return len(probe.Type) > 0 && probe.Type[0] == "object"
	}

	// Otherwise a type-name reference: its underlying decides.
	decl, found, ambiguous := s.resolveNamedTypeLeaf(base, token.Position{})
	if ambiguous || !found {
		return false // unknown name — resolveTypeBase reports it
	}
	t := declNamedType(decl)

	return t != nil && isObjectLikeUnderlying(t)
}

// isObjectLikeUnderlying reports whether a Go type renders as a Swagger object.
func isObjectLikeUnderlying(t types.Type) bool {
	switch t.Underlying().(type) {
	case *types.Struct, *types.Map, *types.Interface:
		return true
	default:
		return false
	}
}

// resolveTypeOverride applies a `swagger:type` argument onto tgt, ALWAYS producing an inline schema
// (never a $ref).
//
// It is the single resolution point for the keyword consumed at every swagger:type site (the F3
//
//   - ownType is the annotated field/decl's Go type, consumed by the
//     `inline` / `array` keywords (which expand that type in place). May be
//     nil when the site has no Go type to expand.
//   - pos drives diagnostics.
//
// Returns applied=true when the override was honoured (the caller short-circuits). applied=false
// means the caller should fall through to default Go-type resolution; a diagnostic explaining why
// has already been recorded (unknown type, `file`, or an invalid array element).
//
// Argument grammar (after stripping N leading `[]` → an N-deep array whose innermost items are
// the resolved base):
//
//   - keyword scalars `string`/`integer`/`number`/`boolean`/`object` and the
//     Go-builtin spellings (`int64`, `uint32`, `float64`, …) — via
//     resolvers.SwaggerSchemaForType;
//   - `inline` — expand ownType in place (no $ref; slice → array of inlined
//     items);
//   - `array` — deprecated alias of `inline` for collections (warns,
//     prefer `inline`);
//   - `file` — valid only on a formData parameter or a response body, both handled by the
//     parameters / responses builders; a diagnostic anywhere else;
//   - any other token — a case-sensitive type-name reference, inlined from a
//     known definition; unknown → diagnostic.
func (s *Builder) resolveTypeOverride(arg string, tgt ifaces.SwaggerTypable, ownType types.Type, pos token.Position) (applied bool) {
	if s.refuseUnrepresentableOverride(arg, ownType, pos) {
		return false
	}

	base, depth := stripArrayPrefixes(arg)
	if depth == 0 {
		return s.resolveTypeBase(base, tgt, ownType, pos, false)
	}

	// `[]T …` — build N array layers, then resolve the base into the innermost items.
	// The items form an inline schema like any other.
	tgt.Typed("array", "")
	items := tgt.Items()
	for range depth - 1 {
		items.Typed("array", "")
		items = items.Items()
	}
	// Cross-ref linkage: the base resolves into the innermost items node, so any anchors it emits (an
	// inlined named struct's properties, enum values) must carry the …/items[/items…] pointer, not
	// the parent's.
	defer s.descendItems(depth)()
	return s.resolveTypeBase(base, items, ownType, pos, true)
}

// resolveTypeBase resolves a single (array-stripped) swagger:type base onto target. isElem reports
// whether target is an array-element position, where the own-type keywords (`inline`/`array`) and
// `file` are not meaningful.
func (s *Builder) resolveTypeBase(base string, target ifaces.SwaggerTypable, ownType types.Type, pos token.Position, isElem bool) (applied bool) {
	switch base {
	case keywordInline, keywordArray:
		if isElem {
			s.RecordDiagnostic(grammar.Warnf(pos, grammar.CodeUnsupportedType,
				"swagger:type: %q is not a valid array element type", base))
			return false
		}
		if base == keywordArray {
			s.RecordDiagnostic(grammar.Warnf(pos, grammar.CodeDeprecated,
				`swagger:type: "array" is deprecated, prefer "inline"`))
		}
		if ownType == nil {
			return false
		}
		return s.inlineGoType(ownType, target)
	case keywordFile:
		// `file` is an OAS v2 type, but only in two places: a formData parameter and a response body.
		// Both are owned by the parameters / responses builders, which raise the file signal for this
		// spelling and apply it behind their own location gates — so reaching here means the location
		// is not one of them.
		s.RecordDiagnostic(grammar.Warnf(pos, grammar.CodeUnsupportedType,
			`swagger:type: "file" is only valid on a formData parameter or a response body; ignored here`))

		return false
	}

	// Scalar / Go-builtin / canonical OAS-2 name.
	if err := resolvers.SwaggerSchemaForType(base, target); err == nil {
		return true
	}

	// Otherwise a type-name reference: inline a known definition in place.
	// Resolve the leaf in the builder's own package first, then uniquely across the scanned packages'
	// models (name-identity leaf resolution).
	decl, found, ambiguous := s.resolveNamedTypeLeaf(base, pos)
	if ambiguous {
		return false // diagnostic already recorded
	}
	if found {
		if t := declNamedType(decl); t != nil && s.buildFromType(t.Underlying(), target) == nil {
			return true
		}
	}

	s.RecordDiagnostic(grammar.Warnf(pos, grammar.CodeUnsupportedType,
		"swagger:type: unknown type %q", base))
	return false
}

// applyStrfmtFormat applies a swagger:strfmt format onto a schema whose type has already been fixed
// by swagger:type.
//
// The format rides as a supplementary hint ONLY when it is consistent with that type (string
// accepts any; integer / number accept the numeric width formats — see
// validations.IsFormatCompatible).
//
// An incompatible format is dropped with a shape-mismatch diagnostic rather than silently
// overriding the type.
// This is the swagger:type + swagger:strfmt precedence: type wins, format is advisory.
//
// It does NOT apply to the strfmt-alone path, where strfmt still forces {type: string, format: X}
// (go-swagger#1512).
func (s *Builder) applyStrfmtFormat(ps *oaispec.Schema, format string, pos token.Position) {
	var schemaType string
	if len(ps.Type) > 0 {
		schemaType = ps.Type[0]
	}
	ok, hint := validations.IsFormatCompatible(schemaType, format)
	if ok {
		ps.Format = format
		return
	}
	s.RecordDiagnostic(grammar.Warnf(pos, grammar.CodeShapeMismatch,
		"swagger:strfmt with swagger:type: %s; format ignored", hint))
}

// inlineGoType expands a Go type onto target as an inline schema, never a $ref: pointers are peeled
// and a named/alias type is reduced to its underlying shape (so buildFromType emits the structure
// rather than a reference).
//
// Returns true on success.
func (s *Builder) inlineGoType(t types.Type, target ifaces.SwaggerTypable) bool {
	base := t
	for {
		ptr, ok := base.(*types.Pointer)
		if !ok {
			break
		}
		base = ptr.Elem()
	}
	// Underlying() peels a Named/Alias to its structural type and is a no-op for already-structural
	// types — so buildFromType inlines uniformly without taking the $ref branch in buildNamedType.
	return s.buildFromType(base.Underlying(), target) == nil
}

// resolveNamedTypeLeaf resolves a bare type name written in a type-name keyword (swagger:type /
// swagger:additionalProperties / swagger:patternProperties) to its declaration.
//
// It looks in the builder's own package first — a local type wins (intent) — then, failing
// that, resolves the leaf across the scanned packages' annotated model set (name-identity leaf
// resolution, mirroring routes' resolveDefinitionByLeaf):
//
//   - exactly one model with that leaf -> (decl, true, false);
//   - several -> records an ambiguity diagnostic and returns (nil, false, true);
//   - none -> (nil, false, false), leaving the caller to emit unknown-type.
func (s *Builder) resolveNamedTypeLeaf(name string, pos token.Position) (decl *scanner.EntityDecl, found, ambiguous bool) {
	if s.Decl != nil {
		if d, ok := s.Ctx.FindDecl(s.Decl.PkgPath(), name); ok && d != nil {
			return d, true, false
		}
	}

	matches := s.Ctx.FindModelsByLeaf(name)
	switch len(matches) {
	case 0:
		return nil, false, false
	case 1:
		return matches[0], true, false
	default:
		pkgs := make([]string, 0, len(matches))
		for _, m := range matches {
			pkgs = append(pkgs, m.Obj().Pkg().Path())
		}
		s.RecordDiagnostic(grammar.Warnf(pos, grammar.CodeAmbiguousTypeName,
			"type name %q is ambiguous: declared as a model in %d packages [%s]; "+
				"use a same-package type or a swagger:model override to disambiguate",
			name, len(matches), strings.Join(pkgs, ", ")))
		return nil, false, true
	}
}

// declNamedType returns the named or alias type a decl carries (nil if neither).
//
// The caller decides whether to emit it as a $ref (the named type) or inline it (its Underlying).
func declNamedType(decl *scanner.EntityDecl) types.Type {
	switch {
	case decl.Type != nil:
		return decl.Type
	case decl.Alias != nil:
		return decl.Alias
	default:
		return nil
	}
}

// swagger:type keyword values that are not resolved as scalar/builtin type names (lowercase,
// case-sensitive — a capitalised spelling is a type-name reference instead, e.g. `Inline` vs the
// keyword `inline`).
const (
	keywordInline = "inline"
	keywordArray  = "array"
	keywordFile   = "file"
)

// stripArrayPrefixes counts leading `[]` prefixes on a swagger:type argument and returns the bare
// base plus the array depth.
//
// `[][]string` → ("string", 2), `int64` → ("int64", 0).
func stripArrayPrefixes(arg string) (base string, depth int) {
	base = strings.TrimSpace(arg)
	for strings.HasPrefix(base, "[]") {
		base = strings.TrimSpace(base[2:])
		depth++
	}
	return base, depth
}
