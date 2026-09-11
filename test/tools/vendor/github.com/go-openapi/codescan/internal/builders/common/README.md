# common builder — maintainer notes

This document is the long-form companion to the `common.Builder` code.

The source files keep godoc concise; complex invariants, design trade-offs, and intentionally-deferred follow-ups live here.

`common.Builder` is the shared state every per-decl builder embeds
(`schema`, `parameters`, `responses`, `routes`, `operations`, `spec`).

It owns the scanner context, the active declaration, the parsed-block memoisation cache, the diagnostic accumulator,
and the post-decl queue.

---

## Table of contents

- [§blockcache](#blockcache) — `ParseBlock` / `ParseBlocks` memoisation strategy and scope
- [§makeref](#makeref) — why `MakeRef` lives on the common base
- [§diagnostics](#diagnostics) — accumulator ordering, dedup posture, LSP-evolution caveat
- [§postdecls](#postdecls) — per-Builder dedup index + cross-Builder re-dedup in the orchestrator
- [§embed-inheritance](#embed-inheritance) — annotations on an embed flow to promoted members
- [§quirks-open](#quirks-open) — deferred follow-ups

---

## <a id="blockcache"></a>§blockcache — `ParseBlock` / `ParseBlocks` memoisation

`Builder.blockCache` memoises `grammar.NewParser(...).ParseAll(cg)`
results keyed by `*ast.CommentGroup` pointer. Two reasons:

1. **Recursive type descent re-visits the same comment.** A struct
   field whose type is itself a struct triggers a nested
   `buildFromDecl`/`buildFromType` pass; without memoisation each
   level re-lexes and re-parses the same field-doc comment group.
2. **Multi-annotation visibility.** `ParseAll` yields one Block per
   annotation on the comment group (the
   `swagger:type` + `swagger:model` co-decl is the canonical
   double-annotation case). Callers that only need the first
   annotation use `ParseBlock`; callers that need every annotation
   iterate `ParseBlocks`.

The cache is **per-Builder** (one top-level decl build), so no
synchronisation is needed: a Builder is single-goroutine for its
entire lifetime. Crossing a Builder boundary discards the cache,
which is fine — the scanner context owns the FileSet, so a parser
constructed in a sibling Builder still produces position-stable
output.

`ParseBlock(cg)` always returns a non-nil Block (the parser yields
at least one Block even for a nil comment group, conventionally an
`UnboundBlock`). Callers can read `AnnotationKind()`,
`AnnotationArg()`, etc. on the result unconditionally.

## <a id="makeref"></a>§makeref — why `MakeRef` lives on the common base

`MakeRef` writes `$ref: #/definitions/<name>` onto a target via
`SwaggerTypable.SetRef`, then enqueues the referenced declaration on
the Builder's post-decl queue so the spec orchestrator visits it
during the discovery loop.

The name source is `decl.Names()` (first entry — top-level decls in
this codebase have a single name).

The method lives on `common.Builder` rather than per-package because
every builder needs the same operation with the same side effect.
Hoisting also means future cross-cutting refinements — a name-collision
diagnostic, a discovery-loop instrumentation counter, a guard against
emitting `$ref` to an unexported name — are one-place edits.

## <a id="diagnostics"></a>§diagnostics — accumulator + LSP-evolution caveat

`Diagnostics()` returns every accumulated `grammar.Diagnostic` in
source order, **raw — no deduplication.** The build re-processes the
same field/annotation across passes (most visibly a `swagger:parameters`
struct applied to several operation ids, which rebuilds every field once
per id), so the slice can carry the identical diagnostic more than once.

The **`OnDiagnostic` callback stream is deduped**, however:
`ScanCtx.EmitDiagnostic` suppresses exact duplicates — same position,
code and message — for the lifetime of one scan, so a consumer that only
listens on the callback (the TUI, an LSP server) sees each distinct
diagnostic once. This is the "structural deduplication" the caveat below
anticipated; it lives at the single delivery boundary so the raw
accumulator stays available for callers that want every occurrence.

The diagnostic surface is **experimental** and expected to evolve
further once the LSP integration matures. Likely remaining changes:
typed severity classes and per-position provenance. The shape is
conservative today (slice of `grammar.Diagnostic` + a callback hook)
precisely so it can be widened without breaking callers.

`RecordDiagnostic` appends to the slice and delivers through
`Ctx.EmitDiagnostic` (the deduped boundary) when a sink is wired.
Walkers' `Diagnostic` callback points at this method so grammar-level
warnings flow into the same accumulator.

## <a id="postdecls"></a>§postdecls — dedup index + orchestrator re-dedup

`AppendPostDecl(decl)` enqueues decl for post-processing by the spec
orchestrator's discovery loop. The Builder maintains a
per-instance dedup index (`postDeclSet`, keyed by the declared type's
`*types.TypeName`) so a single decl re-discovered N times during one Build
pass only enqueues once. The key is the type-checker's object rather than
the declaring identifier: a package cannot declare one name twice, so the
two are in bijection wherever both exist — and the object answers for a
declaration whose source was never parsed, where the identifier does not.

A SECOND dedup runs in `spec.Builder.buildDiscovered` at consumption
time, because two different per-decl Builders may surface the same
post-decl independently. The double-guard means a discovered decl
never reaches a second Build pass even when sibling Builders race to
register it.

Nil decls are silently ignored — defensive against the scanner emitting
nothing at all during error recovery. A decl carrying neither a named type
nor an alias is not tolerated: `Obj()` panics on it, which is the discipline
the type half already applies everywhere else.

`ResetPostDeclarations()` drops the whole queue for a Build pass. Its
one caller is the schema builder's SimpleSchema catch-at-exit validator
(go-swagger#1088): when a non-body parameter / response-header element
dissolves an illegal `$ref`, the decl `MakeRef` discovered for that ref
is a byproduct of the now-removed reference and must not linger as an
orphan definition. A single-type Build renders exactly one target, so
every queued decl is reachable only through it; clearing the whole queue
is correct, and a decl genuinely referenced elsewhere is re-discovered
by that other site's Builder and deduplicated here and in the
orchestrator.

## <a id="embed-inheritance"></a>§embed-inheritance — annotations on an embed flow to promoted members

`EmbedInheritance` + `ReadEmbedInheritance` are the shared kernel of the
rule "a doc-comment directive on an embedded (anonymous) struct field
applies to the members that embed promotes" (go-swagger#2701). All three
field-walking builders embed `*common.Builder` and use it so the
behaviour is identical:

- **parameters** consume `In` and `Required` (an `in: path` / `required:`
  on the embed flows to the promoted parameters);
- **schema** consumes `Required` (a `required:` on the embed adds the
  promoted properties to the enclosing object's required list); it has no
  `in:` concept;
- **responses** consume `In` (the body/header routing discriminator);
  OAS2 response headers carry no `required`.

Each builder keeps its own struct walk (the output objects differ —
`Parameter` vs `Header` vs schema property), threading the context with
save/restore around its embed recursion: the embed's own directive wins
over the inherited one, an absent directive carries the parent's through
(so nesting accumulates), and a promoted member's own directive always
wins over the inherited fallback. `ScanInLocation` (the `in:` line scan,
shared with the parameters/responses field-signal scanners) and
`grammar.NormalizeIn` back the `In` half.

## <a id="quirks-open"></a>§quirks-open — deferred follow-ups

These are real maintenance items the package author noted; they remain open for a future pass.

- **`ireturn` on `ParseBlock`.** The `nolint:ireturn` directive on `ParseBlock` carries because `grammar.Block`
  is a polymorphic interface — that's the documented return type.
  The lint could be disabled package-wide rather than per-function; consider as a `.golangci.yml` exclusion
  once the broader lint posture is reviewed.
