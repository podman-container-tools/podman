// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/ast"
	"go/types"

	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// enterEmbed bumps embedDepth and returns a func that restores it.
//
// Use as `defer s.enterEmbed()()` around any recursive build pass that descends into an embedded
// type.
// Pairs with `applyFieldCarrier`'s ambiguity diagnostic.
//
// # Details
//
// See [§embed-depth](./README.md#embed-depth) — depth-rule semantics and which writes count as
// ambiguous embeds.
func (s *Builder) enterEmbed() func() {
	s.embedDepth++
	return func() { s.embedDepth-- }
}

// buildEmbedded routes a struct's embedded-field type to the appropriate emitter: pointers are
// peeled, named types descend into `buildNamedEmbedded`, aliases resolve transparently and dispatch
// as the unaliased type would (so that `type BaseAlias = Base` embedded in a struct produces the
// same inline shape as `Base` embedded directly — both Go types are identical to the type
// system).
//
// # Details
//
// See [§embedded](./README.md#embedded) — the three-arm dispatch and the contract that embeds
// always inline properties, never $ref unless the embed is `swagger:allOf`-tagged.
func (s *Builder) buildEmbedded(tpe types.Type, schema *oaispec.Schema, nameByJSON map[string]propOwner) error {
	switch ftpe := tpe.(type) {
	case *types.Pointer:
		return s.buildEmbedded(ftpe.Elem(), schema, nameByJSON)
	case *types.Named:
		return s.buildNamedEmbedded(ftpe, schema, nameByJSON)
	case *types.Alias:
		// Aliases are transparent in Go; the spec should reflect that.
		// Recurse on the unaliased type so the dispatch matches what
		// the user would get if they had written the unaliased type
		// directly:
		//   - Alias-to-Named            → buildNamedEmbedded (inline)
		//   - Alias-to-Pointer-to-Named → peel pointer, then inline
		//   - Alias-to-anonymous-struct → falls to default (warn-skip);
		//     embedding an anonymous struct via alias is unusual Go
		//     code that doesn't promote any named fields.
		//   - Alias-to-primitive / interface → also default; embedding
		//     these promotes nothing.
		return s.buildEmbedded(types.Unalias(ftpe), schema, nameByJSON)
	default:
		s.warnUnsupportedGoType("buildEmbedded", ftpe)
		return nil
	}
}

// buildNamedEmbedded inlines an embedded named struct or interface into the outer schema.
//
// The interface arm runs `applyStdlibSpecials` so `error` etc. recognize cleanly; the struct arm
// does not — the asymmetry is intentional, see README §embedded.
//
// # Details
//
// See [§embedded](./README.md#embedded) — `AddDiscoveredModel` pairing, struct-vs-interface
// specials asymmetry, and how `enterEmbed` interacts with `applyFieldCarrier`'s ambiguity
// diagnostic.
func (s *Builder) buildNamedEmbedded(tpe *types.Named, schema *oaispec.Schema, nameByJSON map[string]propOwner) error {
	if resolvers.UnsupportedBuiltin(tpe) {
		s.warnUnsupportedGoType("buildNamedEmbedded", tpe)

		return nil
	}

	switch utpe := tpe.Underlying().(type) {
	case *types.Struct:
		decl, found := s.Ctx.GetModel(tpe.Obj().Pkg().Path(), tpe.Obj().Name())
		if !found {
			return missingSource(tpe)
		}
		s.Ctx.AddDiscoveredModel(decl)

		defer s.enterEmbed()()
		return s.buildFromStruct(decl, utpe, schema, nameByJSON)
	case *types.Interface:
		if utpe.Empty() {
			return nil
		}
		o := tpe.Obj()
		target := NewTypable(schema, 0, s.skipExtensions)
		if applyStdlibSpecials(o, target, s.skipExtensions) {
			return nil
		}

		resolvers.MustNotBeABuiltinType(o)
		decl, found := s.Ctx.GetModel(o.Pkg().Path(), o.Name())
		if !found {
			return missingSource(tpe)
		}
		s.Ctx.AddDiscoveredModel(decl)

		defer s.enterEmbed()()
		return s.buildFromInterface(decl, utpe, schema, nameByJSON)
	default:
		s.warnUnsupportedGoType("buildNamedEmbedded", utpe)
		return nil
	}
}

// processEmbeddedType handles types reached during an interface's `swagger:allOf` walk: named,
// anonymous interface, and alias embeds.
//
// Each non-empty sub-schema becomes an `allOf` member on the outer schema.
//
// # Details
//
// See [§embedded](./README.md#embedded) — interface-side allOf composition rules and the
// `Ref.String() != "" || Properties >0 || AllOf >0` non-empty guard rationale.
func (s *Builder) processEmbeddedType(fld types.Type, flist []*ast.Field, decl *scanner.EntityDecl, schema *oaispec.Schema,
	nameByJSON map[string]propOwner,
) (fieldHasAllOf bool, err error) {
	// Cross-ref linkage: interface-side embeds compose into allOf members (/allOf/{k}/…), an
	// untracked subtree; clear the base path so nothing inside emits a wrong anchor.
	defer s.repath("")()

	switch ftpe := fld.(type) {
	case *types.Named:
		o := ftpe.Obj()
		var dummySchema oaispec.Schema
		ps := NewTypable(&dummySchema, 0, s.skipExtensions)
		if applyStdlibSpecials(o, ps, s.skipExtensions) {
			return false, nil
		}
		return s.buildNamedInterface(ftpe, flist, decl, schema, nameByJSON)
	case *types.Interface:
		var aliasedSchema oaispec.Schema
		ps := NewTypable(&aliasedSchema, 0, s.skipExtensions)
		if err = s.buildAnonymousInterface(ftpe, ps, decl); err != nil {
			return false, err
		}
		if aliasedSchema.Ref.String() != "" || len(aliasedSchema.Properties) > 0 || len(aliasedSchema.AllOf) > 0 {
			fieldHasAllOf = true
			schema.AddToAllOf(aliasedSchema)
		}
	case *types.Alias:
		var aliasedSchema oaispec.Schema
		ps := NewTypable(&aliasedSchema, 0, s.skipExtensions)
		if err = s.buildAlias(ftpe, ps); err != nil {
			return false, err
		}
		if aliasedSchema.Ref.String() != "" || len(aliasedSchema.Properties) > 0 || len(aliasedSchema.AllOf) > 0 {
			fieldHasAllOf = true
			schema.AddToAllOf(aliasedSchema)
		}
	default:
		s.warnUnsupportedGoType("buildNamedInterface.allOf", ftpe)
	}

	return fieldHasAllOf, nil
}
