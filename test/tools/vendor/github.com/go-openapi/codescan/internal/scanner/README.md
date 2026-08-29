# scanner — maintainer notes

This document is the long-form companion to the scanner package code.
The source files keep godoc concise; complex invariants, design
trade-offs, and known quirks live here.

The `scanner` package owns package loading and entity discovery. It
turns a set of Go package patterns into a `ScanCtx` that exposes the
classified per-decl inventory (meta, routes, operations, models,
parameters, responses) consumed by the builder layer.

---

## Table of contents

- [§options](#options) — `Options.DescWithRef` shape and rationale
- [§descwithref](#descwithref) — the description-only-decoration
  $ref shape and why it has a flag
- [§diagnostics](#diagnostics) — `OnDiagnostic` contract and
  experimental-API caveat
- [§prune](#prune) — `PruneUnusedModels` reachability and why it
  runs before name reduction
- [§model-lookup](#model-lookup) — `GetModel` vs `FindModel` —
  pure read vs implicit registration
- [§classifier](#classifier) — `detectNodes` bitmask semantics and
  struct-annotation exclusivity
- [§after-decl](#after-decl) — `AfterDeclComments` — reading annotations
  inside / below a declaration
- [§clean-godoc](#clean-godoc) — `CleanGoDoc` — filtering godoc syntax out
  of carried-over title / description prose
- [§quirks-open](#quirks-open) — deferred follow-ups

---

## <a id="options"></a>§options — `Options` overview

`Options` is the externally-visible configuration struct. It is
re-exported from the package root as `codescan.Options`. The default
zero value is a valid configuration: every flag defaults to false and
every slice/map defaults to nil.

Most fields are simple toggles (scope inclusion, debug, vendor
extension suppression). Two fields carry non-trivial semantics that
warrant the inline godoc and the deeper notes below:

- `DescWithRef` — controls the `$ref` shape used when a struct field
  resolves to a named type and its only decoration is a description.
  See [§descwithref](#descwithref).
- `OnDiagnostic` — diagnostic callback hook. See
  [§diagnostics](#diagnostics).
- `PruneUnusedModels` — drop discovered definitions unreachable from
  any root, on top of `ScanModels`. See [§prune](#prune).

## <a id="descwithref"></a>§descwithref — description-only-decoration $ref shape

When a struct field's Go type resolves to a named type (so the spec
emits a `$ref` to its definition) and its only field-level
decoration is a description (no validations, no user-authored
vendor extensions), the spec has two possible shapes:

1. **Bare $ref** — `{$ref: ...}`. The field's description is
   dropped. This is the conservative default when `DescWithRef` is
   false.
2. **Single-arm allOf** — `{description: "...", allOf: [{$ref}]}`.
   The description is preserved by wrapping the `$ref` in a
   single-arm `allOf` compound. This is JSON-Schema-draft-4 correct
   for sibling description.

`DescWithRef=true` opts into the second shape. The default is false
because the bare-`$ref` shape interoperates more broadly with
Swagger 2.0 tooling that does not implement the `allOf` compound.

When the field also carries validation overrides (pattern, enum,
example, etc.) or user-authored vendor extensions, the `allOf`
compound is mandatory regardless of `DescWithRef` — the override
would be lost otherwise.

## <a id="diagnostics"></a>§diagnostics — `OnDiagnostic` callback

`Options.OnDiagnostic`, when non-nil, is invoked for every
`grammar.Diagnostic` the builder layer records: lexer/parser
warnings, semantic-validation failures from the validations package,
and any future diagnostic class wired into the builder pipeline.

Contract:

- The callback fires **once per diagnostic, in source order**.
- Diagnostics **never block the build**. An invalid construct is
  silently dropped from the output spec; the explanation flows
  through this channel instead.
- The callback may be called from any per-decl builder; it is the
  caller's responsibility to make it goroutine-safe if the consumer
  ever drives `codescan.Run` concurrently (today it is single-
  goroutine, but the callback contract makes no such guarantee).

The diagnostic surface is **experimental**. Once the LSP integration
matures the shape is expected to grow: typed severity classes,
structural deduplication, per-position provenance. Callers that
adopt `OnDiagnostic` today should treat the signature as subject to
breaking change in a future minor release.

`ScanCtx.OnDiagnostic` returns the user-supplied callback verbatim;
builders pipe diagnostics through it via `common.Builder.RecordDiagnostic`.

## <a id="prune"></a>§prune — `PruneUnusedModels` reachability

`Options.PruneUnusedModels` is a modifier on `ScanModels` (`-m`). The
three emission modes:

1. **no `ScanModels`** — only models transitively reachable from
   routes/responses/parameters are emitted (discovery-driven).
2. **`ScanModels`** — every `swagger:model` type is emitted, reachable
   or not.
3. **`ScanModels` + `PruneUnusedModels`** — discovery runs as in (2),
   then unreachable definitions are pruned again. The middle ground a
   shared-library scan wants: keep only the `$ref`'d subset
   (go-swagger/go-swagger#2639).

Without `ScanModels` the flag is a no-op (the set is already
reachable-only) and raises one positionless `scan.pruned-unused` Hint.

**Shared objects pruned first (C4).** Before the definition walk, the
shared parameters (`#/parameters/*`) and responses (`#/responses/*`) that
no operation and no path-item references are themselves pruned
(`spec/prune.go`, `pruneUnusedSharedObjects`; the read-only "is
referenced" mirror is `collectSharedRefs`). `InputSpec`-supplied shared
objects are pinned (never pruned), mirroring the definitions rule. Each
drop raises a located `scan.pruned-unused` Hint. Because this precedes
the definition walk, a definition kept alive only by a now-pruned shared
object becomes prunable in turn. A pruned shared response's buffered
provenance anchors are dropped (`DropDeferredOrigins`) so none dangle —
shared-response anchors are buffered (`BeginDeferredOrigins`) and flushed
verbatim after the prune only when `PruneUnusedModels` is set, so the
non-prune anchor stream is unchanged.

**Reachability.** Roots are the paths (operation body parameters +
response schemas), the *surviving* shared `responses` and `parameters`,
and every definition supplied via `InputSpec`. Overlay definitions are
**pinned**: never pruned and seeded as roots so their `$ref` targets
survive. The walk (`spec/prune.go`, `collectDefRefs`) is the read-only
mirror of the ref-rewriter (`reduce.go`, `rewriteSchemaRefs`) and must
cover the same container set; a `visited` set handles recursive / cyclic
models. A model referenced only by another unreferenced model is itself
pruned.

**Ordering — before name reduction.** The prune runs *before*
`reduceDefinitionNames`, in the fully-qualified `#/definitions/<pkgpath>/
<name>` key namespace. This is the point of the feature, not an
implementation detail: name reduction deconflicts cross-package leaf
collisions (`a.Thing` / `b.Thing` → `AThing` / `BThing`). Pruning an
*unused* twin first means the collision never materialises, so the
surviving model keeps its bare leaf name — no spurious concat churn.
Each prune raises a located `scan.pruned-unused` Hint; the buffered
provenance for a pruned node is dropped so no anchor dangles. The
collision renames the reduce stage *does* perform are surfaced as
`scan.renamed-definition` Hints (located at the Go type).

**Known limitation.** A discriminator base references its subtypes by
mapping string, not by `$ref`, so a subtype reachable only through a
discriminator could be pruned. codescan does not auto-wire discriminator
subtypes today; revisit if it ever does (forthcoming-features §15).

## <a id="model-lookup"></a>§model-lookup — `GetModel` vs `FindModel`

`ScanCtx` exposes two lookup helpers with similar signatures but
different side-effect contracts. The choice between them is
load-bearing for the shape of the emitted spec.

### `GetModel(pkgPath, name)` — pure read

Looks up a model decl across three sources, in order:

1. `Models` — decls annotated with `swagger:model`. Always emitted
   as top-level definitions regardless of lookup.
2. `ExtraModels` — decls discovered as dependencies of other
   emitted shapes. Already enqueued for top-level emission.
3. `FindDecl` — fall through to a syntactic search over the
   loaded packages.

No side effect. A `FindDecl` hit through `GetModel` does **not**
register the decl in `ExtraModels`. Callers that want the lookup
to also surface the decl as a top-level definition must follow up
with `AddDiscoveredModel` explicitly.

### `FindModel(pkgPath, name)` — implicit registration

The older sibling of `GetModel`. It does the same three-source
lookup, but a `FindDecl` hit also writes the decl into `ExtraModels`
as a side effect.

`FindModel` is deprecated. The implicit registration surprises
readers and pulls stdlib types (notably `time.Time`,
`json.RawMessage`) into the spec's top-level definitions when they
should be inlined where referenced. Builders that need the
registration should use the explicit `GetModel` + `AddDiscoveredModel`
pair.

### `AddDiscoveredModel` — explicit registration

Registers a decl in `ExtraModels`. No-op for decls already in
`Models` (annotated decls are emitted unconditionally — registering
them as discovered would create a Models↔ExtraModels bouncing loop
in the spec orchestrator's `joinExtraModels` pass). Nil and
Ident-less decls are silently ignored, which is defensive against
the scanner emitting partial decls during error recovery.

## <a id="classifier"></a>§classifier — `detectNodes` bitmask

`TypeIndex.detectNodes` scans every comment group in a file and
returns a bitmask of detected annotation kinds. Each kind drives
downstream processing:

| Bit | Annotation | Downstream |
|---|---|---|
| `metaNode` | `swagger:meta` | file-level meta block |
| `routeNode` | `swagger:route` | path-level route annotations |
| `operationNode` | `swagger:operation` | path-level operation annotations |
| `modelNode` | `swagger:model` | per-decl model registration |
| `parametersNode` | `swagger:parameters` | per-decl parameter registration |
| `responseNode` | `swagger:response` | per-decl response registration |

`route`, `operation`, and `meta` accumulate freely across comment
groups in a file. The three struct-level annotations (`model`,
`parameters`, `response`) are **mutually exclusive within a single
comment group** — a struct cannot simultaneously be a model and a
parameters bag, for instance. `checkStructConflict` enforces the
rule per comment group and returns an error if the constraint is
violated.

The annotation vocabulary recognised by the classifier is a closed
set. Unknown annotations beginning with `swagger:` raise a
classifier error. A handful of annotation tokens (`strfmt`, `name`,
`enum`, `default`, `alias`, `type`, `title`, `description`, …) are
recognised but produce no bit — they are field/decl-level decorations
that downstream builders parse out of the comment block directly.
(`title` / `description` are the godoc title/description overrides; see
the schema builder's [§user-overrides](../builders/schema/README.md#user-overrides).)

## <a id="after-decl"></a>§after-decl — `AfterDeclComments`

`Options.AfterDeclComments` (opt-in, default false) lets swagger annotations
live **inside** a declaration or **inlined** as a trailing comment, so the godoc
*above* the declaration stays clean and human-facing. It is **solely a scanner
concern** — the located comments are folded into the comment source the builders
already consume (`EntityDecl.Comments` and `ast.Field.Doc`), so the grammar and
builders are untouched. Same annotation grammar, no new syntax.

What the scanner folds, by shape (`index.go`):

| Shape | Folded comment | Into |
|---|---|---|
| struct type | leading body comment groups (after `{`, before the first field, excluding any field `.Doc`) — `leadingBodyComments` | a fresh merged `EntityDecl.Comments` (`ts.Doc` untouched) |
| alias / non-struct type | trailing `TypeSpec.Comment` (`type X = Y // swagger:model …`) | same |
| struct field | trailing `Field.Comment` (`B string // swagger:strfmt date`) — `enrichStructFields` | the shared `Field.Doc` (the one mutation, see below) |

The clean godoc above still provides the title/description: the merged group is
`docAbove ++ located`, and because positions stay ascending (doc above < the
inside/trailing comment below), the grammar reconstructs a blank-line gap and
parses it without change. Discovery works because `detectNodes` already scans
every `file.Comments` group (the file bitmask flips), and the merged
`EntityDecl.Comments` makes the per-decl `HasModelAnnotation` gate pass.

**Idempotency.** Decl-level folding is pure construction — `ts.Doc` is never
mutated, so re-processing is safe with no guard. Field-level folding is the only
place the shared AST is mutated (`Field.Doc` is repointed to the merged group),
guarded by `TypeIndex.enrichedFields` so a field is rewritten at most once.

**Routes / operations** are already position-agnostic
(`collectRoute/OperationPathAnnotations` scan all `file.Comments`), so a
`swagger:route` inside a func body is discovered with or without this option.

**Out of scope.** A standalone `const X = … // swagger:enum`: `swagger:enum` is
type-based (it resolves a *type* and collects that type's consts via
`FindEnumValues`), so a lone const is not an enum carrier and has no builder
semantics today. Supporting it would mean new builder behaviour, which this
scanner-only feature deliberately avoids. Nested/anonymous inline structs are
likewise not enriched (only named struct type decls are walked).

## <a id="clean-godoc"></a>§clean-godoc — `CleanGoDoc`

`Options.CleanGoDoc` (opt-in, default false) rewrites godoc-specific syntax that
reads as bracket noise when a title / description is carried **from godoc** into
the spec, and recomposes resolvable doc-links to the name the referenced schema
is **exposed under**. Off ⇒ output is byte-identical.

The scanner side is thin: it holds the flag (`CleanGoDoc()`) and a shared
`mangling.NameMangler` (`Mangler()`, used for humanization). The transform,
the consumption-seam wiring, the go/types resolver, and the post-reduce marker
substitution all live in the **builders** — see
[`internal/builders/godoclink/README.md`](../builders/godoclink/README.md) for
the two-phase marker contract and the full mechanics.

Like `swagger:title` / `swagger:description` (overrides) and `AfterDeclComments`,
this is part of the **clean-godoc cluster**: keep the Go-facing doc clean while
the API spec carries curated text. Crucially it touches **only godoc-derived
prose** — author-written overrides (harvested separately) are never filtered.

## <a id="quirks-open"></a>§quirks-open — deferred follow-ups

- **`FindModel` deprecation.** The deprecated alias is still on the
  `ScanCtx` surface for in-tree callers. Once every builder has been
  audited and migrated to the `GetModel` + `AddDiscoveredModel` pair,
  the deprecated method can be removed in a future major release.
- **Recognised-but-unused annotation tokens.** `detectNodes`
  recognises a list of field-level tokens (`strfmt`, `name`,
  `discriminated`, `file`, `enum`, `default`, `alias`, `type`,
  `allOf`, `ignore`, `title`, `description`) only to avoid raising the
  "unknown annotation" error. Promoting them to per-file bits would let downstream
  builders skip whole files that carry no decorations — an
  optimisation, not a correctness change.
- **`shouldAcceptTag` precedence.** When both `includeTags` and
  `excludeTags` are populated, `includeTags` wins (a tag in
  `includeTags` admits the operation even if it also appears in
  `excludeTags`). This is deliberate but easy to mis-read; an
  explicit "the include list takes precedence" doc on `Options`
  would help callers, but the field-level prose is already dense.
