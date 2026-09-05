// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
	oaispec "github.com/go-openapi/spec"
)

// omitPathSep separates the segments of a qualified `swagger:omit` target (`Base.ID`).
const omitPathSep = "."

// parseOmitTargets splits a `swagger:omit` argument into its targets.
//
// The lexer captures the whole remainder verbatim (the list may carry spaces after commas), so the
// split happens here: comma-separated, each target trimmed, empties dropped.
func parseOmitTargets(arg string) []string {
	var out []string
	for raw := range strings.SplitSeq(arg, ",") {
		if t := strings.TrimSpace(raw); t != "" {
			out = append(out, t)
		}
	}

	return out
}

// resolveOmitTargets resolves each target against the embedded type, returning the field objects to
// filter out of the promotion walk.
//
// Resolution runs against the TYPE, not the walk: a target either names a (promoted) field of the
// embedded type or it does not, and that is known before a single property is written. The scope
// then carries field OBJECTS, so the pre-filter is pointer identity — no name matching, no
// bookkeeping of what was "actually omitted", and no collision between same-named fields at
// different depths.
//
// [types.LookupFieldOrMethod] is Go's own promoted-field lookup, so depth and ambiguity rules come
// for free: a target that Go itself could not resolve unambiguously resolves to nothing here and is
// reported.
//
// Unresolved targets raise a scan.omit-unresolved Hint (see [grammar.CodeOmitUnresolved]) — this is
// what stops the annotation rotting silently when a field is renamed upstream.
func (s *Builder) resolveOmitTargets(embedded types.Type, targets []string, pos token.Position) []*types.Var {
	if len(targets) == 0 {
		return nil
	}

	out := make([]*types.Var, 0, len(targets))
	for _, target := range targets {
		fld, ok := lookupOmitPath(embedded, strings.Split(target, omitPathSep))
		if !ok {
			s.hintf(pos, grammar.CodeOmitUnresolved,
				"swagger:omit: %s has no field %q; the target is ignored", typeLabel(embedded), target)

			continue
		}
		out = append(out, fld)
	}

	return out
}

// lookupOmitPath walks a dotted target (`Inner.Deep`) through the embed chain of tpe and returns the
// field the last segment names.
//
// Every segment but the last must name an EMBEDDED field: `swagger:omit` removes promoted content,
// which is the only thing the enclosing schema owns. A segment naming a regular field stops the walk
// (reported as unresolved by the caller) — reaching through a regular field would either edit
// another type's inlined copy or, worse, a `$ref`'d definition.
func lookupOmitPath(tpe types.Type, path []string) (*types.Var, bool) {
	cur := tpe
	for i, seg := range path {
		obj, _, _ := types.LookupFieldOrMethod(cur, true, pkgOf(cur), seg)
		fld, isField := obj.(*types.Var)
		if !isField {
			return nil, false
		}
		if i == len(path)-1 {
			return fld, true
		}
		if !fld.Embedded() {
			return nil, false
		}
		cur = fld.Type()
	}

	return nil, false
}

// pkgOf returns the package a named type belongs to, or nil for an unnamed one.
//
// [types.LookupFieldOrMethod] needs it only to decide whether unexported names are visible; a nil
// package restricts the lookup to exported fields, which is the right default for an unnamed type.
func pkgOf(tpe types.Type) *types.Package {
	switch t := types.Unalias(tpe).(type) {
	case *types.Pointer:
		return pkgOf(t.Elem())
	case *types.Named:
		if obj := t.Obj(); obj != nil {
			return obj.Pkg()
		}
	}

	return nil
}

// typeLabel names a type for a diagnostic — the declared name when there is one, else the type
// string.
func typeLabel(tpe types.Type) string {
	if named, ok := types.Unalias(tpe).(*types.Named); ok {
		return named.Obj().Name()
	}

	return tpe.String()
}

// pushOmitted adds fields to the active omit scope and returns the function restoring it.
//
// The scope is a set of field objects in force for the subtree currently being walked; nesting
// accumulates, and each level removes only what it added, so sibling embeds never leak into one
// another.
func (s *Builder) pushOmitted(fields []*types.Var) func() {
	if len(fields) == 0 {
		return func() {}
	}
	if s.omitted == nil {
		s.omitted = make(map[*types.Var]struct{}, len(fields))
	}

	added := make([]*types.Var, 0, len(fields))
	for _, fld := range fields {
		if _, dup := s.omitted[fld]; dup {
			continue
		}
		s.omitted[fld] = struct{}{}
		added = append(added, fld)
	}

	return func() {
		for _, fld := range added {
			delete(s.omitted, fld)
		}
	}
}

// isOmitted reports whether fld is filtered out by the active `swagger:omit` scope.
//
// This is the whole enforcement: a pre-filter on the promotion walk, so the property is never
// written. Because it runs before any name is computed, it is indifferent to json tags, to
// `NameFromTags`, and to whether the enclosing schema is inlined or composed with allOf.
func (s *Builder) isOmitted(fld *types.Var) bool {
	if len(s.omitted) == 0 {
		return false
	}
	_, omitted := s.omitted[fld]

	return omitted
}

// embedOmitTargets collects the `swagger:omit` targets that apply to one embed and resolves them
// against the embedded type.
//
// Sources: the embed's own annotation (the ergonomic form — targets are plain field names of that
// type) and the enclosing declaration's annotation (the power form — a bare name resolved against
// the promoted set, or a dotted path whose head names this embed).
//
// isAllOfRef marks an embed that will be emitted as a `$ref` rather than inlined; the omission
// cannot be expressed there, so it is dropped with a scan.omit-behind-ref Hint instead of silently
// forking the referenced definition.
func (s *Builder) embedOmitTargets(
	fld *types.Var, afld *ast.Field, fd fieldDoc, declOmits []string, isAllOfRef bool,
) []*types.Var {
	targets := append([]string(nil), fd.OmitTargets...)
	targets = append(targets, declOmitsFor(fld, declOmits)...)
	if len(targets) == 0 {
		return nil
	}

	pos := s.Ctx.PosOf(afld.Pos())
	if isAllOfRef {
		s.hintf(pos, grammar.CodeOmitBehindRef,
			"swagger:omit: %s is composed as a $ref; a referenced definition cannot have a property "+
				"subtracted, so the omission is dropped", typeLabel(fld.Type()))

		return nil
	}

	return s.resolveOmitTargets(fld.Type(), targets, pos)
}

// declOmitsFor selects the declaration-level targets that apply to one embed: a dotted path whose
// head names this embed (with the head consumed), and every bare name — a bare name is resolved
// against the promoted set of each embed in turn, exactly as Go resolves a promoted field.
//
// A bare name that matches no embed is reported once per embed it was tried against; that is the
// intended behaviour for a typo and is silent for the (redundant, harmless) case where the field was
// already excluded for another reason.
func declOmitsFor(fld *types.Var, declOmits []string) []string {
	if len(declOmits) == 0 {
		return nil
	}

	var out []string
	for _, target := range declOmits {
		head, tail, qualified := strings.Cut(target, omitPathSep)
		if !qualified {
			out = append(out, target)

			continue
		}
		if head == fld.Name() {
			out = append(out, tail)
		}
	}

	return out
}

// hintf emits an informational diagnostic through the builder's sink.
func (s *Builder) hintf(pos token.Position, code grammar.Code, format string, args ...any) {
	if onDiag := s.Ctx.OnDiagnostic(); onDiag != nil {
		onDiag(grammar.Hintf(pos, code, format, args...))
	}
}

// declOmitTargets reads the declaration-level `swagger:omit` targets off a type's doc comment.
//
// These are the power form, for embeds the author cannot annotate directly (a type they do not own,
// or one nested deeper than their own source).
func (s *Builder) declOmitTargets(cg *ast.CommentGroup) []string {
	if cg == nil {
		return nil
	}

	var out []string
	for _, b := range s.ParseBlocks(cg) {
		if b.AnnotationKind() != grammar.AnnOmit {
			continue
		}
		if arg, ok := b.AnnotationArg(); ok {
			out = append(out, parseOmitTargets(arg)...)
		}
	}

	return out
}

// isModelEmbed reports whether an embedded type is a `swagger:model`, i.e. whether composing it into
// an allOf member emits a `$ref` rather than an inline copy (see buildNamedAllOf).
//
// A `$ref` member is where `swagger:omit` cannot be expressed at all.
func (s *Builder) isModelEmbed(tpe types.Type) bool {
	named, ok := types.Unalias(derefType(tpe)).(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	decl, found := s.Ctx.GetModel(obj.Pkg().Path(), obj.Name())

	return found && decl.HasModelAnnotation()
}

// derefType peels a pointer, so `*Base` classifies exactly like `Base`.
func derefType(tpe types.Type) types.Type {
	if ptr, ok := types.Unalias(tpe).(*types.Pointer); ok {
		return ptr.Elem()
	}

	return tpe
}

// warnShadowedByJSONDash reports a field re-declared with `json:"-"` that carries the same Go name as
// a field promoted from an embed.
//
// encoding/json ignores a `-` field ENTIRELY — it never enters the name set, so it cannot shadow the
// promoted field and Go keeps marshalling the embedded one. The author almost certainly meant
// `swagger:omit`, which removes it from the schema for real.
//
// See [grammar.CodeShadowedEmbedField].
func (s *Builder) warnShadowedByJSONDash(fld *types.Var, afld *ast.Field, target *oaispec.Schema, nameByJSON map[string]propOwner) {
	if afld == nil || len(nameByJSON) == 0 {
		return
	}
	for jsonName, prior := range nameByJSON {
		if prior.goName != fld.Name() || prior.depth == 0 {
			continue
		}
		if _, written := target.Properties[jsonName]; !written {
			continue
		}
		s.hintf(s.Ctx.PosOf(afld.Pos()), grammar.CodeShadowedEmbedField,
			"field %s is re-declared with `json:\"-\"`, but encoding/json ignores such a field entirely: "+
				"it does not shadow the %q promoted from an embed, which Go still marshals. "+
				"Use swagger:omit on the embed to drop it from the schema", fld.Name(), jsonName)

		return
	}
}
