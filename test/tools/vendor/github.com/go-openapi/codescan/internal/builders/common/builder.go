// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package common holds shared per-Builder state every concrete per-decl builder (schema,
// parameters, responses, routes, operations, spec) embeds.
//
// See [./README.md](./README.md) for the long-form maintainer notes on cache scope, diagnostic
// posture, and the post-decl queue's double-dedup design.
package common

import (
	"go/ast"
	"go/token"

	"github.com/go-openapi/codescan/internal/builders/godoclink"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// Builder holds the per-decl state shared across every concrete builder via embedding.
//
// See [§blockcache], [§diagnostics], and [§postdecls] for the cache scope, accumulator posture,
// and discovery queue's dedup design.
//
// [§blockcache]: https://github.com/go-openapi/codescan/blob/master/internal/common/README.md#blockcache
// [§diagnostics]: https://github.com/go-openapi/codescan/blob/master/internal/common/README.md#diagnostics
// [§postdecls]: https://github.com/go-openapi/codescan/blob/master/internal/common/README.md#postdecls
type Builder struct {
	Ctx  *scanner.ScanCtx
	Decl *scanner.EntityDecl

	postDecls   []*scanner.EntityDecl
	postDeclSet map[*ast.Ident]struct{} // dedup index keyed by EntityDecl.Ident
	diagnostics []grammar.Diagnostic
	blockCache  map[*ast.CommentGroup][]grammar.Block
}

// New builds a [Builder] bound to ctx and decl.
//
// The blockCache is pre-allocated empty.
func New(ctx *scanner.ScanCtx, decl *scanner.EntityDecl) *Builder {
	return &Builder{
		Ctx:        ctx,
		Decl:       decl,
		blockCache: make(map[*ast.CommentGroup][]grammar.Block),
	}
}

// PostDeclarations returns the post-decl queue accumulated by this Builder during a Build pass, in
// source order.
//
// See [§postdecls](./README.md#postdecls).
func (s *Builder) PostDeclarations() []*scanner.EntityDecl {
	return s.postDecls
}

// Diagnostics returns every grammar.Diagnostic accumulated by this Builder during a Build pass.
//
// Source order is preserved; no deduplication is applied.
// The slice is owned by the Builder; callers must not mutate it.
// Returns nil before Build is invoked or when no diagnostics were recorded.
//
// # Details
//
// See [§diagnostics](./README.md#diagnostics) — accumulator ordering, dedup posture, and the
// LSP-evolution caveat (the diagnostic surface is expected to widen once IDE integration matures).
func (s *Builder) Diagnostics() []grammar.Diagnostic {
	return s.diagnostics
}

// RecordDiagnostic accumulates one diagnostic on the Builder and fires the consumer's
// [Options.OnDiagnostic] callback when wired.
//
// Walker.Diagnostic is bound to this method, so grammar-level warnings flow through the same
// accumulator as builder-level ones.
func (s *Builder) RecordDiagnostic(d grammar.Diagnostic) {
	s.diagnostics = append(s.diagnostics, d)
	s.Ctx.EmitDiagnostic(d)
}

// WarnStrippedPathRegex records a warning that one or more inline regex path-parameter constraints
// (`{id:[0-9]+}`) were stripped to the bare `{id}` template form.
//
// OpenAPI 2.0 path templating follows RFC 6570 URI Template Level-1 expansion (simple `{name}`
// substitution) only — it cannot express regex/operator constraints — so the route is still
// emitted, with the constraint dropped.
// No-op when params is empty.
//
// Shared by the routes and operations builders.
func (s *Builder) WarnStrippedPathRegex(pos token.Pos, params []string) {
	if len(params) == 0 {
		return
	}
	s.RecordDiagnostic(grammar.Warnf(
		s.Ctx.PosOf(pos),
		grammar.CodeInvalidAnnotation,
		"inline regex constraint on path parameter(s) %v is unsupported: OpenAPI 2.0 path "+
			"templating follows RFC 6570 URI Template Level-1 expansion (bare {name}) only; "+
			"the constraint was stripped",
		params,
	))
}

// ParseBlocks returns the cached grammar.Block slice for cg (one entry per annotation), parsing on
// first access and memoising the result.
//
// Always returns a non-nil slice with at least one Block, so consumers can call
// [Block.AnnotationKind], [Block.AnnotationArg] / etc. unconditionally on the first element.
//
// # Details
//
// See [§blockcache](./README.md#blockcache) — memoisation scope, why ParseAll is preferred over
// Parse, and the per-Builder (single-goroutine) lifetime that obviates synchronisation.
func (s *Builder) ParseBlocks(cg *ast.CommentGroup) []grammar.Block {
	parser := grammar.NewParser(s.Ctx.FileSet(),
		grammar.WithSingleLineCommentAsDescription(s.Ctx.SingleLineCommentAsDescription()))
	if cg == nil {
		return parser.ParseAll(nil)
	}

	bs, ok := s.blockCache[cg]
	if !ok {
		bs = parser.ParseAll(cg)
		s.blockCache[cg] = bs
	}

	return bs
}

// ParseBlock returns the first Block from [Builder.ParseBlocks].
//
// This is the "primary" annotation for callers that don't need multi-annotation
// visibility (title/description, field-level lookups).
//
//nolint:ireturn // grammar.Block is the documented polymorphic return type.
func (s *Builder) ParseBlock(cg *ast.CommentGroup) grammar.Block {
	return s.ParseBlocks(cg)[0]
}

// OverrideValue is an optional swagger:title / swagger:description override harvested from a
// comment group.
//
// Present=false → annotation absent (fall back to the godoc-derived value); Present=true with
// Value=="" → explicit empty, the deliberate godoc-suppression affordance (design D7).
type OverrideValue struct {
	Value   string
	Present bool
	Pos     token.Position
}

// HarvestOverrides scans a comment group's sibling classifier blocks for the swagger:title /
// swagger:description override annotations.
//
// Last occurrence wins.
// This is a pure harvest: the diagnostic policy — the empty-override warning, and the
// context-invalid rejection of swagger:title where a target has no title (responses / headers) —
// is left to each consumer, since it differs per builder.
func (s *Builder) HarvestOverrides(cg *ast.CommentGroup) (title, desc OverrideValue) {
	for _, b := range s.ParseBlocks(cg) {
		switch b.AnnotationKind() { //nolint:exhaustive // only the two override kinds are relevant here
		case grammar.AnnTitle:
			arg, _ := b.AnnotationArg()
			title = OverrideValue{Value: arg, Present: true, Pos: b.Pos()}
		case grammar.AnnDescription:
			arg, _ := b.AnnotationArg()
			desc = OverrideValue{Value: arg, Present: true, Pos: b.Pos()}
		}
	}
	return title, desc
}

// WarnEmptyOverride raises scan.empty-override when an override is present with an empty value.
//
// The empty value is still applied by the caller — empty is the deliberate godoc-suppression
// affordance — but the case is flagged in case the marker was left bare by mistake (design D7).
// Emitted at the consumption point rather than in the parser: sibling classifier blocks are not
// Walk-ed, so a grammar-stored diagnostic would not reach OnDiagnostic.
func (s *Builder) WarnEmptyOverride(kind grammar.AnnotationKind, ov OverrideValue) {
	if !ov.Present || ov.Value != "" {
		return
	}
	s.RecordDiagnostic(grammar.Warnf(ov.Pos, grammar.CodeEmptyOverride,
		"swagger:%s override is empty: the godoc-derived value is suppressed", kind))
}

// CleanGoDoc applies godoc-syntax filtering (Options.CleanGoDoc) to godoc- derived prose, returning
// text unchanged when the option is off.
//
// It MUST be applied only to godoc-derived title / description text — never to author- written
// swagger:title / swagger:description override values, which are deliberate and harvested through
// HarvestOverrides.
//
// Resolvable doc-links are recomposed (via markers; see godoclink) to the referenced schema's
// exposed name; the rest are humanized.
func (s *Builder) CleanGoDoc(text string) string {
	if !s.Ctx.CleanGoDoc() {
		return text
	}

	return godoclink.Clean(text, godoclink.Options{
		Mangler:  s.Ctx.Mangler(),
		Resolver: s.godocResolver(),
	})
}

// CleanGoDocSelf is CleanGoDoc plus leading self-name recomposition: the decl's own
// godoc-convention leading name ("Widget" in "Widget does things") is recomposed to its exposed
// definition name.
//
// Use it for a declaration's title / description; use CleanGoDoc for field / member prose.
func (s *Builder) CleanGoDocSelf(text string) string {
	if !s.Ctx.CleanGoDoc() {
		return text
	}

	return godoclink.Clean(text, godoclink.Options{
		Mangler:  s.Ctx.Mangler(),
		Resolver: s.godocResolver(),
		Self:     s.godocSelf(),
	})
}

// AppendPostDecl marks decl for post-processing by the spec orchestrator's discovery loop.
//
// Idempotent per-Builder: re-appending a decl whose Ident was already seen is a no-op.
// Nil and Ident-less decls are silently ignored.
//
// # Details
//
// See [§postdecls](./README.md#postdecls) — per-Builder dedup index and the second dedup applied
// at consumption time by spec.Builder.buildDiscovered.
func (s *Builder) AppendPostDecl(decl *scanner.EntityDecl) {
	if decl == nil || decl.Ident == nil {
		return
	}
	if s.postDeclSet == nil {
		s.postDeclSet = make(map[*ast.Ident]struct{})
	}
	if _, dup := s.postDeclSet[decl.Ident]; dup {
		return
	}
	s.postDeclSet[decl.Ident] = struct{}{}
	s.postDecls = append(s.postDecls, decl)
}

// ResetPostDeclarations drops every decl this Builder enqueued during the current Build pass.
//
// Used by the SimpleSchema catch-at-exit validator: when a non-body parameter / response-header
// element dissolves an illegal $ref, the decl that MakeRef discovered for that ref is a byproduct
// of the now-removed reference and would otherwise linger as an orphan definition
// (go-swagger#1088).
//
// A single-type Build renders exactly one target, so every queued decl is reachable only through
// it; clearing the whole queue is correct.
// A decl genuinely referenced elsewhere is re-discovered by that other site's Builder and
// deduplicated by the orchestrator.
//
// # Details
//
// See [§postdecls](./README.md#postdecls).
func (s *Builder) ResetPostDeclarations() {
	s.postDecls = nil
	s.postDeclSet = nil
}

// MakeRef writes a `$ref: "#/definitions/<name>"` onto prop and registers decl with the discovery
// loop via AppendPostDecl.
//
// The name comes from decl.Names() (the first entry — top-level decls in this codebase have a
// single name).
// Returns an error only if oaispec.NewRef rejects the JSON pointer.
//
// # Details
//
// See [§makeref](./README.md#makeref) — why the operation lives on the common base and what
// kinds of cross-cutting refinements that shape enables.
func (s *Builder) MakeRef(decl *scanner.EntityDecl, prop ifaces.SwaggerTypable) error {
	// Emit the fully-qualified identity key (pkgpath/name), not the bare short name: this keeps
	// distinct Go types from colliding before the spec.Builder's reduce stage shortens names back.
	// §9.1/§12.1.
	ref, err := oaispec.NewRef("#/definitions/" + decl.DefKey())
	if err != nil {
		return err
	}

	prop.SetRef(ref)
	s.AppendPostDecl(decl)

	return nil
}
