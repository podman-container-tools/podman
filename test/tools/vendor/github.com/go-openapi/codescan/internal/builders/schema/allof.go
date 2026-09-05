// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// scanEmbeddedFields walks st's anonymous fields and decides whether each embed contributes
// properties to the outer schema directly or becomes an `allOf` compound member.
//
// # Details
//
// See [§allof](./README.md#allof) — embed classification rules, `IsAllOfMember` semantics, and
// how the returned target ties to `buildFromStruct`'s second pass.
//
// Returns:
//   - target — the schema receiving properties; nil if no embed contributed,
//     `schema` itself for plain embeds, a fresh schema when allOf is in play.
//   - hasAllOf — true if at least one allOf member was appended.
func (s *Builder) scanEmbeddedFields(
	decl *scanner.EntityDecl, st *types.Struct, schema *oaispec.Schema, nameByJSON map[string]propOwner,
) (target *oaispec.Schema, hasAllOf bool, err error) {
	// `swagger:omit` written on the DECLARATION (the power form) applies to the embeds walked below;
	// the ergonomic form lives on each embed and is read from its fieldDoc.
	declOmits := s.declOmitTargets(decl.Comments())

	for fld := range st.Fields() {
		if !fld.Anonymous() {
			continue
		}

		afld := resolvers.FindASTFieldFor(decl.File(), fld, s.Ctx.PosOf)
		if afld == nil {
			continue
		}

		fd := s.scanFieldDoc(afld)
		if fd.Ignored {
			continue
		}
		_, ignore, isString, omitEmpty, err := resolvers.ParseFieldTag(afld, fld.Name(), s.Ctx.NameFromTags())
		if err != nil {
			return nil, false, err
		}
		if ignore {
			continue
		}

		// DefaultAllOfForEmbeds promotes a plain (untagged) embed to allOf composition, exactly as a
		// `swagger:allOf` tag would.
		// A json-named embed is a single named property, not a promotion, so it stays inline
		// (go-swagger#2038); interface embeds are out of scope here.
		isAllOf := fd.IsAllOfMember
		if !isAllOf && s.Ctx.DefaultAllOfForEmbeds() && embedNestName(afld, fd) == "" {
			isAllOf = true
		}

		// `swagger:omit` is a PRE-filter: the listed fields are dropped before the embed is walked, so
		// nothing is ever written for them. That makes the annotation mean the same thing whether the
		// embed is inlined or composed into an allOf member — see omit.go.
		// An embed emitted as a $ref cannot have a property subtracted; embedOmitTargets reports that
		// and filters nothing.
		restoreOmits := s.pushOmitted(
			s.embedOmitTargets(fld, afld, fd, declOmits, isAllOf && s.isModelEmbed(fld.Type())),
		)

		if !isAllOf {
			target, err = s.buildPlainEmbed(fld, afld, fd, isString, omitEmpty, schema, target, nameByJSON)
			restoreOmits()
			if err != nil {
				return nil, false, err
			}
			continue
		}

		hasAllOf = true
		s.warnIneffectiveEmbedAnnotations(afld, fd)
		if target == nil {
			target = &oaispec.Schema{}
		}
		var newSch oaispec.Schema
		buildErr := s.buildAllOf(fld.Type(), &newSch)
		restoreOmits()
		if buildErr != nil {
			return nil, false, buildErr
		}

		if fd.AllOfClass != "" {
			schema.AddExtension("x-class", fd.AllOfClass)
		}
		schema.AllOf = append(schema.AllOf, newSch)
	}

	return target, hasAllOf, nil
}

// warnIneffectiveEmbedAnnotations reports `swagger:strfmt` / `swagger:type` written in an EMBEDDED
// field's own comment, which no embed arm consults.
//
// Both are honoured on a regular field, so the same annotation in the same syntactic position — a
// field's doc comment — works one line and does nothing the next. Worse, the scanner reads that
// comment and rejects an unknown annotation in it, so the author gets validation feedback implying
// the annotation is meaningful and no feedback at all that it was discarded.
//
// An embed contributes its embedded type's shape; what that shape is comes from that type's own
// declaration. So the annotation belongs there, and the message says so rather than merely refusing.
func (s *Builder) warnIneffectiveEmbedAnnotations(afld *ast.Field, fd fieldDoc) {
	var annotations []string
	if fd.StrfmtName != "" {
		annotations = append(annotations, "swagger:strfmt")
	}
	if fd.TypeOverride != "" {
		annotations = append(annotations, "swagger:type")
	}
	if len(annotations) == 0 {
		return
	}

	s.RecordDiagnostic(grammar.Warnf(s.Ctx.PosOf(afld.Pos()), grammar.CodeIneffectiveAnnotation,
		"%s on an embedded field has no effect and is ignored; annotate the embedded type's own "+
			"declaration instead", strings.Join(annotations, " and ")))
}

// buildPlainEmbed handles an anonymous embed that carries no `swagger:allOf` annotation, returning
// the (possibly newly-assigned) property target.
//
// Two shapes, mirroring Go's encoding/json:
//
//   - an embed with an explicit json tag name (or a `swagger:name`) is
//     NOT promoted — it nests under that name as a single regular
//     property, built from the embedded type (a $ref when the type is a
//     model). See go-swagger#2038.
//   - otherwise the embedded type's properties are inlined (promoted)
//     into the outer schema, regardless of whether the embed is a direct
//     named type or an alias of one. (The previous `!isAliased` guard
//     silently promoted aliased embeds to allOf composition, violating
//     the contract that allOf is only produced for explicitly-annotated
//     embeds.)
func (s *Builder) buildPlainEmbed(
	fld *types.Var, afld *ast.Field, fd fieldDoc, isString, omitEmpty bool,
	schema, target *oaispec.Schema, nameByJSON map[string]propOwner,
) (*oaispec.Schema, error) {
	if target == nil {
		target = schema
	}

	nestName := embedNestName(afld, fd)
	if nestName == "" && !embedPromotes(fld.Type()) {
		// An embed that promotes nothing is not a promotion at all: Go keeps the value as an ordinary
		// member keyed by the TYPE name, so that is what the schema says. It takes the same path as a
		// json-named embed because it IS the same thing — a single named property built from the
		// embedded type, classifiers included.
		nestName = fld.Name()
	}
	if nestName != "" {
		err := s.applyFieldCarrier(fieldCarrier{
			name:      nestName,
			goName:    fld.Name(),
			propType:  fld.Type(),
			afld:      afld,
			fd:        fd,
			isString:  isString,
			omitEmpty: omitEmpty,
		}, target, nameByJSON)
		return target, err
	}

	// Past this point the embed genuinely promotes, and no arm of the promotion walk consults the
	// embed's own comment — so a classifier written there is dropped and must be reported.
	s.warnIneffectiveEmbedAnnotations(afld, fd)

	// A `required:` annotation on the embed applies to the properties it promotes (go-swagger#2701).
	// Thread it through the recursion, restoring afterwards so sibling fields are unaffected.
	saved := s.embedInherited
	s.embedInherited = s.ReadEmbedInheritance(afld.Doc, saved)
	err := s.buildEmbedded(fld.Type(), target, nameByJSON)
	s.embedInherited = saved

	return target, err
}

// embedNestName returns the explicit name an embed nests under — the json tag name, overridden by
// `swagger:name` — or "" when the embed promotes its properties (Go field promotion).
//
// An embed with a nest name is a single named property, never a promotion or allOf composition
// (go-swagger#2038).
func embedNestName(afld *ast.Field, fd fieldDoc) string {
	if fd.JSONName != "" {
		return fd.JSONName
	}
	return resolvers.ExplicitJSONName(afld)
}

// embedPromotes reports whether embedding tpe contributes PROMOTED members — struct fields or
// interface methods — rather than a single member named after the type.
//
// Anything else (a named type over a basic, slice, array or map) has no member to promote, so Go
// marshals it as an ordinary key named after the type. This used to reach `buildNamedEmbedded`,
// whose switch had struct and interface arms only, and fall to a warn-and-skip default that dropped
// the member from the schema entirely.
//
// # Why a promoted marshaller does not enter into it
//
// A type reaching the false branch may implement encoding.TextMarshaler, which Go promotes to the
// embedding struct and which makes the WHOLE struct marshal as a bare scalar under the DEFAULT
// marshaller — siblings and all. codescan deliberately does not model that: in the convention it
// describes, an embed means composition, and a composed model round-trips through a hand-written
// MarshalJSON/UnmarshalJSON (as go-swagger's generated models do) rather than the default one. A
// promoted marshaller in the source is therefore not evidence about the wire. Detecting it would
// also require deciding a case that is not decidable from a declaration: a POINTER-receiver
// marshaller squashes for `&v` and not for `v`, and codescan reads the type, not the use site.
//
// See [§embedded](./README.md#embedded).
func embedPromotes(tpe types.Type) bool {
	unaliased := types.Unalias(tpe)
	if ptr, ok := unaliased.(*types.Pointer); ok {
		unaliased = types.Unalias(ptr.Elem())
	}

	switch unaliased.Underlying().(type) {
	case *types.Struct, *types.Interface:
		return true
	default:
		return false
	}
}

// buildAllOf builds the schema for one allOf compound member.
//
// Peels pointers and routes named types and aliases to their dedicated helpers.
//
// # Details
//
// See [§allof](./README.md#allof) — the three-arm dispatch and why non-Named / non-Alias inputs
// are dropped silently with a logger warning rather than an error.
func (s *Builder) buildAllOf(tpe types.Type, schema *oaispec.Schema) error {
	// Cross-ref linkage: an allOf member is an untracked subtree (/allOf/{k}/…); clear the base path
	// so nothing inside emits a wrong anchor.
	// Members that are $refs anchor via their own definition.
	defer s.repath("")()

	switch ftpe := tpe.(type) {
	case *types.Pointer:
		return s.buildAllOf(ftpe.Elem(), schema)
	case *types.Named:
		return s.buildNamedAllOf(ftpe, schema)
	case *types.Alias:
		tgt := NewTypable(schema, 0, s.skipExtensions)
		return s.buildAlias(ftpe, tgt)
	default:
		s.warnUnsupportedGoType("buildAllOf", ftpe)
		return nil
	}
}

// buildNamedAllOf resolves a named type appearing as an allOf member.
//
// Struct and interface underlyings share the same precedence shape: user-classifier first, then
// stdlib specials, then model lookup, then inline build.
//
// # Details
//
// See [§allof](./README.md#allof) — arm symmetry rationale and why `classifierAliasTargetStrfmt`
// is preferred over a comment-group-keyed variant.
func (s *Builder) buildNamedAllOf(ftpe *types.Named, schema *oaispec.Schema) error {
	tgt := NewTypable(schema, 0, s.skipExtensions)
	tio := ftpe.Obj()

	if ApplyStdlibSpecials(tio, tgt, s.skipExtensions) {
		return nil
	}

	decl, found := s.Ctx.GetModel(tio.Pkg().Path(), tio.Name())
	if !found {
		return fmt.Errorf("can't find source for named allOf member %s: %w", ftpe.String(), ErrSchema)
	}

	// A `swagger:model` member is referenced, and its classifiers ride on its own definition — the
	// same gate buildNamedType applies before inlining an override.
	if decl.HasModelAnnotation() {
		return s.MakeRef(decl, tgt)
	}

	// The author's classifiers, in the precedence the field dispatch uses: `swagger:type` decides the
	// type axis outright, then the shape-aware classifiers.
	//
	// This arm used to run one shape-BLIND strfmt check and no type override at all, so a member
	// carrying `swagger:type` or `swagger:enum` came out as an EMPTY schema (its basic underlying then
	// fell to the warn-and-skip default below), and a format on a string sequence landed on the whole
	// member instead of on its items.
	if handled, recurse := s.classifierNamedTypeOverride(
		decl.Comments(), tgt, ftpe, s.Ctx.PosOf(tio.Pos()),
	); handled {
		if recurse {
			return s.buildFromType(ftpe.Underlying(), tgt)
		}

		return nil
	}
	if handled, recurse := s.applyNamedShapeClassifier(decl.Comments(), ftpe, tgt); handled {
		if recurse != nil {
			return recurse()
		}

		return nil
	}

	// Shape dispatch. Unlike the field arm this INLINES rather than emitting a $ref: a member that is
	// not a `swagger:model` has no definition to point at, and publishing one would put types in the
	// spec their author never asked to expose.
	switch utpe := ftpe.Underlying().(type) {
	case *types.Struct:
		return s.buildFromStruct(decl, utpe, schema, make(map[string]propOwner))
	case *types.Interface:
		return s.buildFromInterface(decl, utpe, schema, make(map[string]propOwner))
	default:
		s.warnUnsupportedGoType("buildNamedAllOf", utpe)
		return nil
	}
}
