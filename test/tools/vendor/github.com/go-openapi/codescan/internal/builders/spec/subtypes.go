// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"go/ast"
	"go/types"
	"sort"

	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// subtypeIndex is the reverse `swagger:allOf` index: Go type identity of a base (see typeIdentity) mapped to every
// `swagger:model` declaration that declares that base as an `allOf` member.
//
// It is the inverse of the reference direction the spec carries: a subtype `$ref`s its base, never the other way round,
// so a discriminated base entering the reachable closure can only pull its family in by looking the relation up
// backwards. go-swagger#1913.
type subtypeIndex map[string][]*scanner.EntityDecl

// subtypes returns the reverse `swagger:allOf` index, building it on first use.
//
// The source is ScanCtx.Models() — populated by classification whether or not ScanModels is set, so the index sees
// every annotated subtype even when none of them would be built (the no-`-m` case this feature exists for).
//
// Entries are ordered by definition key so the pull order — and hence the order of the emitted
// scan.discovered-subtype Hints — does not depend on map iteration.
func (s *Builder) subtypes() subtypeIndex {
	if s.subtypeIdx != nil {
		return s.subtypeIdx
	}

	idx := make(subtypeIndex)
	parser := grammar.NewParser(s.ctx.FileSet())
	for _, decl := range s.ctx.Models() {
		for _, base := range allOfBases(decl, parser) {
			idx[base] = append(idx[base], decl)
		}
	}
	for base := range idx {
		decls := idx[base]
		sort.Slice(decls, func(i, j int) bool { return decls[i].DefKey() < decls[j].DefKey() })
	}

	s.subtypeIdx = idx

	return idx
}

// discriminatedSubtypesOf returns the declarations to pull into discovery because decl — whose definition has just
// been built — is a discriminated base.
//
// The gate is the built definition's own `discriminator`, not a source-level re-derivation: it holds uniformly for an
// interface base with a `discriminator: true` member and for a struct base with a `discriminator: true` field, and it
// is the same fact the emitted document exposes.
//
// A base with no discriminator pulls nothing: its `allOf` users are ordinary compositions, not a polymorphic family,
// and inventing definitions for them would over-generate.
//
// Under ScanModels the pass is a no-op: buildModels already builds every annotated model, so there is nothing to pull
// — and pulling anyway would emit Hints whose order follows the model index's map iteration.
// The `-m` case is served by the prune reachability rule instead (subtypeKeysOf), which is where a discriminated family
// can actually be lost.
//
// Subtypes already emitted are skipped; the discovery loop dedups queued ones, and only genuinely new pulls are
// reported as Hints.
func (s *Builder) discriminatedSubtypesOf(decl *scanner.EntityDecl) []*scanner.EntityDecl {
	if s.scanModels {
		return nil
	}

	sch, built := s.definitions[decl.DefKey()]
	if !built || !isDiscriminated(&sch) {
		return nil
	}

	candidates := s.subtypes()[typeIdentity(decl.Obj())]
	if len(candidates) == 0 {
		return nil
	}

	onDiag := s.ctx.OnDiagnostic()
	out := make([]*scanner.EntityDecl, 0, len(candidates))
	for _, sub := range candidates {
		if _, already := s.definitions[sub.DefKey()]; already {
			continue
		}
		out = append(out, sub)
		if onDiag != nil {
			onDiag(grammar.Hintf(s.ctx.PosOf(sub.Pos()), grammar.CodeDiscoveredSubtype,
				"definition %q discovered as a subtype of discriminated base %q",
				leafName(sub.DefKey()), leafName(decl.DefKey())))
		}
	}

	return out
}

// isDiscriminated reports whether a definition DECLARES a discriminator of its own — the gate both hooks share.
//
// Two shapes carry one, depending on how the type is written:
//
//   - a plain base puts it at the top level ({type: object, properties,
//     discriminator});
//   - a MID-LEVEL base — a subtype that is itself a base — renders as
//     `allOf: [{$ref parent}, {own props, discriminator}]`, so its
//     discriminator sits in its own compound member, not at the top.
//     Multi-level hierarchies would otherwise stop at the first level.
//
// Only an inline discriminator counts.
// A leaf subtype's `$ref` member is not followed, so it does not inherit its base's discriminator and does not
// masquerade as a base itself.
func isDiscriminated(sch *oaispec.Schema) bool {
	if sch == nil {
		return false
	}
	if sch.Discriminator != "" {
		return true
	}
	for i := range sch.AllOf {
		if isDiscriminated(&sch.AllOf[i]) {
			return true
		}
	}

	return false
}

// subtypeKeysOf returns the definition keys of the subtypes of the base emitted under defKey.
//
// Used by the prune reachability walk, which works in definition-key space; the bridge back to Go type identity is the
// declIdentity side table, recorded for every definition as it is built.
// Keys are returned whether or not they are (still) present in the definitions map — marking is membership-checked by
// the caller.
func (s *Builder) subtypeKeysOf(defKey string) []string {
	identity, known := s.declIdentity[defKey]
	if !known {
		return nil
	}

	subs := s.subtypes()[identity]
	if len(subs) == 0 {
		return nil
	}

	out := make([]string, 0, len(subs))
	for _, sub := range subs {
		out = append(out, sub.DefKey())
	}

	return out
}

// allOfBases returns the Go type identity of every base that decl declares as an `allOf` member, i.e. each embedded
// field carrying `swagger:allOf`.
//
// Only explicit `swagger:allOf` counts.
// DefaultAllOfForEmbeds — which renders a plain embed AS allOf composition — is deliberately not honoured: it is a
// rendering knob, and letting it decide which definitions exist would make the emitted set depend on an unrelated
// option.
//
// The polymorphic idiom is the explicit annotation.
//
// `swagger:ignore` on the embed drops it here exactly as it does in the schema builder, so the index never claims a
// relation the document does not carry.
// Declarations with no embeddable members (an alias, a named basic type, …) yield nothing.
func allOfBases(decl *scanner.EntityDecl, parser grammar.Parser) []string {
	// No source, no embeds to read: `swagger:allOf` lives in a comment, and the AST field it decorates
	// is the only thing the annotation can be paired with.
	expr, ok := decl.TypeExpr()
	if !ok {
		return nil
	}

	var out []string
	seen := make(map[string]struct{})
	// The two halves of an embed are read from different places and neither can stand in for the
	// other: the annotation lives in the AST field's doc comment, the base it names lives in the
	// declared type's underlying.
	for _, embed := range resolvers.Embeds(embeddableMembers(expr), decl.ObjType().Underlying()) {
		if !isAllOfEmbed(embed.Field, parser) {
			continue
		}
		// Deduped: a struct can reach one base through two embeds (the type and an alias of it), and the index must not list
		// the same subtype twice under one base.
		for _, id := range baseIdentities(embed.Type) {
			if _, dup := seen[id]; dup {
				continue
			}
			seen[id] = struct{}{}
			out = append(out, id)
		}
	}

	return out
}

// embeddableMembers returns the member list of a declaration that can hold an embed: a struct's fields or an
// interface's method list.
//
// Both matter: a struct embeds its base as an anonymous field, and an INTERFACE embeds its base as an anonymous
// interface — which is how a mid-level type in a multi-level hierarchy is written (a subtype of the root that is
// itself a discriminated base).
// In both AST shapes an embed is the entry with no Names.
//
// Returns nothing for any other type shape.
func embeddableMembers(typeExpr ast.Expr) []*ast.Field {
	switch tpe := typeExpr.(type) {
	case *ast.StructType:
		if tpe.Fields == nil {
			return nil
		}
		return tpe.Fields.List
	case *ast.InterfaceType:
		if tpe.Methods == nil {
			return nil
		}
		return tpe.Methods.List
	default:
		return nil
	}
}

// isAllOfEmbed reports whether an embedded field's doc marks it as an `allOf` member.
//
// This mirrors the `IsAllOfMember` / `Ignored` half of the schema builder's field-doc classifier (schema.scanFieldDoc)
// — the index needs those two signals only, and the spec builder owns no field-level walker to borrow them from.
func isAllOfEmbed(afld *ast.Field, parser grammar.Parser) bool {
	if afld.Doc == nil {
		return false
	}

	var isAllOf bool
	for _, b := range parser.ParseAll(afld.Doc) {
		switch b.AnnotationKind() { //nolint:exhaustive // only these two signals matter to the index
		case grammar.AnnIgnore:
			return false
		case grammar.AnnAllOf:
			isAllOf = true
		}
	}

	return isAllOf
}

// baseIdentities returns every type identity an embedded base may be indexed under: the pointer is unwrapped (`*Base`
// composes exactly like `Base`), and an alias contributes BOTH its own identity and that of the type it names.
//
// Both alias identities are kept because which one the emitted `allOf` member `$ref`s depends on RefAliases /
// TransparentAliases: indexing both means the relation is found either way, and since identities are unique per
// declaration there is no risk of matching an unrelated base.
//
// Returns nothing for an unnamed type — an anonymous struct / interface embed has no declaration to index against.
func baseIdentities(t types.Type) []string {
	if ptr, isPtr := t.(*types.Pointer); isPtr {
		t = ptr.Elem()
	}

	var out []string
	if alias, isAlias := t.(*types.Alias); isAlias {
		out = append(out, typeIdentity(alias.Obj()))
	}
	if named, isNamed := types.Unalias(t).(*types.Named); isNamed {
		if id := typeIdentity(named.Obj()); len(out) == 0 || out[0] != id {
			out = append(out, id)
		}
	}

	return out
}

// typeIdentity is the index key for a base: "<pkgpath>.<Name>", the compiler's identity for the declared type.
//
// Both ends of the relation can compute the Go type — not the swagger definition name — without knowing the
// other's annotations: the embed site resolves a type, the base declaration owns one, while either may carry a
// `swagger:model <name>` override.
// A package-less (universe) type falls back to its bare name.
func typeIdentity(obj *types.TypeName) string {
	if obj == nil {
		return ""
	}
	if pkg := obj.Pkg(); pkg != nil {
		return pkg.Path() + "." + obj.Name()
	}

	return obj.Name()
}
