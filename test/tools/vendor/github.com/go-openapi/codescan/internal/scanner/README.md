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
- [§declaration](#declaration) — `EntityDecl`'s two halves: what is
  always available, what needs parsed source, and the accessors that
  make the difference impossible to ignore
- [§diagnostics](#diagnostics) — `OnDiagnostic` contract and
  experimental-API caveat
- [§prune](#prune) — `PruneUnusedModels` reachability and why it
  runs before name reduction
- [§subtypes](#subtypes) — discriminator subtype discovery — the
  reverse `swagger:allOf` index
- [§model-lookup](#model-lookup) — `GetModel` vs `FindModel` —
  pure read vs implicit registration
- [§classifier](#classifier) — `detectNodes` bitmask semantics and
  struct-annotation exclusivity
- [§after-decl](#after-decl) — `AfterDeclComments` — reading annotations
  inside / below a declaration
- [§enum-values](#enum-values) — where `swagger:enum` member values
  come from, and what the degraded reading can still see
- [§clean-godoc](#clean-godoc) — `CleanGoDoc` — filtering godoc syntax out
  of carried-over title / description prose
- [§loader](#loader) — which loader to pick and why, how the choice is
  reconciled, and what each one costs
- [§export-data](#export-data) — preparing compiler export data offline, and
  what it buys
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
- `ToolchainFreeLoader` / `FS` — select codescan's own package loader,
  optionally over a virtual filesystem. See [§loader](#loader).

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

## <a id="declaration"></a>§declaration — the two halves of `EntityDecl`

A declaration is made of two halves that come from different places and are
not equally available.

The **type half** — `Type` / `Alias`, and the `Obj`, `ObjType`, `Name`,
`Pos`, `PkgPath`, `IsAlias` accessors built on them — comes from the
type-checker. A package whose types were read from compiled export data
carries it in full, positions included: `time.Duration` reports
`$GOROOT/src/time/time.go`.

The **syntax half** — the comment group, the declaring identifier, the type
spec, the enclosing file and the loaded package — comes from parsed source,
which a package need not have. Nothing that the type half can answer may be
read from it: a name, a position and a package path all come from the
object, or the declaration stops working the moment its source is not read.

The syntax half is **unexported**, so its absence is not a nil field a
caller can forget about. It is reached through accessors, split by whether
absence is already a legitimate outcome:

| accessor | absent ⇒ | why |
|---|---|---|
| `HasSource() bool` | — | the single honest gate |
| `Comments() *ast.CommentGroup` | `nil` | nil is already "no annotations here" — a declaration with no doc comment answers the same |
| `File() *ast.File` | `nil` | its one consumer, `resolvers.FindASTField`, yields no field for a nil file |
| `TypeExpr() (ast.Expr, bool)` | `false` | the written right-hand side; `Underlying` is not a substitute |
| `Imports() ([]*ast.ImportSpec, bool)` | `false` | an import alias is spelled nowhere else |
| `PkgImport(path) (*packages.Package, bool)` | `false` | the other half of `Imports`: an unaliased import's own name |
| `EnumSourcePkg() (*packages.Package, bool)` | `false` | `swagger:enum` members are read from the package's const blocks |

There is deliberately **no `Ident` accessor**. Everything the identifier was
ever asked — the name, the position, the identity a dedup index keys on —
the object answers, and it answers for a declaration with no source. The
identifier survives only inside this package, as the key of the model
indexes.

### What genuinely needs source

Three things, and only three:

1. **The annotations** (`Comments()`, and a field's `Doc`). A comment exists
   nowhere but in source.
2. **The written right-hand side** — `Stamp` in `type StampResp Stamp`.
   `go/types` keeps no record of it on a defined type: `Underlying` peels
   every named layer at once, and a stdlib recognizer keys on that named
   layer, as does a `swagger:strfmt` one declaration to the right. `WrittenRHS` reads it through `TypeExpr()`.
3. **A file's imports**, for resolving a godoc doc-link.

### Why `WrittenRHS`'s two refusals are not the same refusal

`WrittenRHS` reports `(nil, false)` for two unrelated reasons, and a caller
that cannot tell them apart is one silent wrong answer away from trouble:

- **structural** — there is no syntax at all, so there is nothing to read;
- **semantic** — the right-hand side is a shape the resolution declines
  (a generic instantiation, a dot-imported name).

`Underlying()` is the honest fallback for the second and a *wrong answer*
for the first: it peels exactly the named layer the stdlib recognizers key
on, so `type Stamp time.Time` in a syntax-less package would render as a
struct instead of `format: date-time`. The schema builder therefore checks
`HasSource()` before falling back, and reports an error rather than guessing
when there is no source. That branch is unreachable today — every `EntityDecl`
is built from an AST walk — and it is written out so it cannot start
springing quietly the day one is not.

### Not `types.Info.Types`

`WrittenRHS` and the embed pairing (`builders/resolvers.Embeds`) both
resolve a source expression to a type. Neither goes through
`types.Info.Types`, and that restraint is load-bearing rather than
stylistic: that map records what each type *expression* denotes, only a
source type-check produces it, and it cannot be rebuilt by hand because the
field distinguishing a type from a value is unexported. A package served
from export data therefore cannot be handed to the builders with its source
attached for as long as anything downstream reads it.

So the two resolve without it:

- `WrittenRHS` prefers the checker's record when there is one, and
  otherwise resolves the right-hand side expression against the scopes
  `go/types` exposes — a package scope is complete whether its types were
  checked from source or read from export data. It declines a generic
  instantiation rather than guessing at one.
- `Embeds` pairs each anonymous entry of a struct's field list or an
  interface's method list with its type positionally, read from the
  declared type's underlying. The pairing is exact in both shapes: a
  struct's fields are built in source order (an entry with N names
  contributing N fields), and an interface's embedded types are recorded in
  source order.

Both are checked against the type-checker's own answer over the whole
fixture corpus rather than argued from the emitted spec, because a
mispairing produces a plausible wrong answer instead of a failure. The
end-to-end witness builds each corpus twice, once with `types.Info.Types`
dropped, and compares the documents.

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

**Discriminated families are kept whole.** A subtype `$ref`s its base,
never the reverse, so the walk above cannot see the subtypes of a
discriminated base and would prune a polymorphic family down to its base
alone. A reachable definition carrying `discriminator` therefore also
marks its subtypes reachable, via the reverse `swagger:allOf` index
(`spec/subtypes.go`, `subtypeKeysOf`). Note the rule keeps a *reached*
base's family — it does not make bases roots: a discriminated base
nothing references is still pruned, together with its subtypes. See
[§subtypes](#subtypes).

## <a id="subtypes"></a>§subtypes — discriminator subtype discovery

Interface-based polymorphism emits a base definition carrying
`discriminator` plus one `allOf: [{$ref base}, {own props}]` definition
per subtype — a struct embedding the base under `swagger:allOf`.

The reference direction is the problem: **a subtype `$ref`s its base;
nothing `$ref`s a subtype.** So a route that references only the base
reaches the base and stops, and the emitted family is the base alone —
useless to a consumer that has to unmarshal the polymorphic payload
(go-swagger/go-swagger#1913). Before v0.37 the only way to get the
subtypes was `ScanModels`, which emits every annotated model whether it
belongs to the family or not.

**Reverse index.** `spec/subtypes.go` builds, once per scan, a
`base Go type identity → subtype declarations` map from the model index
(`TypeIndex.Models`, which classification populates whether or not
`ScanModels` is set — the pull depends on that independence).
A subtype relation is an *embedded member carrying `swagger:allOf`* — a
struct's anonymous field **or an interface's anonymous interface**, which
is how a mid-level type in a multi-level hierarchy is written:

- the pointer is unwrapped (`*Base` composes like `Base`), and an alias
  embed is indexed under both its own and the aliased type's identity, so
  the relation is found whichever definition the `allOf` member ends up
  `$ref`ing under `RefAliases` / `TransparentAliases`;
- `swagger:ignore` on the embed drops it, exactly as in the schema
  builder — the index never claims a relation the document lacks;
- a *plain* (unannotated) embed is not a subtype: it inlines the base's
  properties, no `allOf` member. `DefaultAllOfForEmbeds` is deliberately
  **not** honoured here — it is a rendering knob, and letting it decide
  which definitions exist would be a surprising coupling.

The index is keyed by **Go type identity** (`<pkgpath>.<TypeName>`), not
by swagger name: it is the one fact both ends can compute without knowing
the other's `swagger:model` override. Entries are ordered by definition
key so the pull order — and hence the Hint order — does not follow map
iteration.

**Two hooks, one per hole.**

1. *Discovery* (`spec.go`, `buildDiscoveredSchema` →
   `discriminatedSubtypesOf`): when a definition has just been built and
   carries a `discriminator`, its subtypes are appended to `s.discovered`,
   so the existing fixpoint loop builds them, discovers *their*
   dependencies, and cascades if a subtype is itself a discriminated
   base. Each genuinely new pull raises a located
   `scan.discovered-subtype` Hint. A no-op under `ScanModels`, where every
   model is built up front anyway.
2. *Prune reachability* (`prune.go`): see
   [§prune](#prune) — required because under `ScanModels` the family is
   built and then lost, not never-built.

**The gate is the built definition's own `discriminator`**
(`isDiscriminated`), not a source-level re-derivation. It reads
identically for an interface base with a `discriminator: true` member and
a struct base with a `discriminator: true` property, and it is the same
fact the emitted document exposes. A base with no discriminator pulls
nothing: its `allOf` users are ordinary compositions, not a polymorphic
family.

**Multi-level hierarchies.** A mid-level type — a subtype that is itself a
base — renders as `allOf: [{$ref parent}, {own props, discriminator}]`:
its properties, and therefore its discriminator, sit in its own compound
member, *not* at the top level. So the gate looks for an inline
discriminator anywhere in the definition's own schema. The `$ref` member
is deliberately **not** followed: a leaf must not inherit its base's
discriminator, or every subtype would pull in its own siblings. Because
hook A feeds the discovery fixpoint, the levels cascade — the root pulls
the mid-level, and the mid-level (only just pulled in itself) pulls the
leaves on the next round.

Fixtures: `testdata/enhancements/discriminated-subtypes` (`edges/` holds
the embed-shape corner cases, in a family no route references — which
also locks the other half of the gate: an unreached base pulls nothing)
and `testdata/enhancements/discriminated-subtypes-nested` (two-level
hierarchy: `Shape` → `Polygon` → `Square`/`Triangle`).

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
groups in a file. For `meta` that means the group the annotation was
found in **is** the meta block, wherever in the file it sits — not
the file's package doc, which the block was once taken from
regardless of where it had been detected. A `swagger:meta` outside
the package doc then had an unrelated comment parsed in its place,
or, in a file with no package doc at all, nothing to parse and a nil
comment group carried into the origin recorder.

The three struct-level annotations (`model`,
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

## <a id="enum-values"></a>§enum-values — reading `swagger:enum` members

`FindEnumValues` walks the const declarations of a package and
emits one row per constant whose type is the annotated enum type.
Two decisions shape it.

### Membership is decided per name, from the type-checker

The spec's syntactic type (`vs.Type`) is not usable as the
membership test, because inside an `iota` block only the *first*
spec carries a type at all:

```go
const (
    Sunday Weekday = iota   // Type=Weekday  Values=[iota]
    Monday                  // Type=<nil>    Values=<nil>
    Tuesday                 // Type=<nil>    Values=<nil>
)
```

Monday and Tuesday inherit both implicitly, so a syntactic reader
sees two specs that declare nothing. Membership therefore comes
from `TypesInfo.Defs[name].(*types.Const).Type()` — the type the
checker assigned — which also covers a constant declared without a
written type (`const Extra = StatusOn`).

The test is **type identity, not name**: the named type must also
come from the package being walked. A constant declared in the
annotated package can perfectly well have an imported type
(`const ForeignDay foreign.Weekday = 13` next to a local `Weekday`
enum), and it is a member of neither. The syntactic reading ruled
that out structurally — a qualified type is a selector expression,
not the bare ident it required — so the package check is what
keeps the type-checked reading from being *wider* than the one it
replaced.

An enum cannot be hosted on an **alias to a basic type**
(`type Unsigned = uint64`): the checker erases the alias, so
`const Zero Unsigned = 0` is indistinguishable from any other
`uint64` constant and there is nothing left to match on. The
annotation collects nothing there, and now says so: it raises
`parse.invalid-enum-option` naming the declaration and suggesting a
named type. It used to be silent, which left an author with a
correct-looking annotation and no members.

An alias to a *named* enum type (`type Weekday2 = Weekday`) is
fine — the underlying named type survives — and is deliberately
NOT warned about.

This is where `swagger:enum` parts company with `swagger:strfmt`
and `swagger:type` on aliases. Those two decorate the emitted
schema and were made to work at alias use sites; this one has no
data to work with, so the remedy is a diagnostic rather than
plumbing.

### Values come from the type-checker, not from the literal

A const's right-hand side is only incidentally a literal. It can
be `iota`, an expression (`1 << 3`), a reference to an earlier
member (`Prev * 2`), a rune literal (`'a'`, whose constant is the
integer 97), or `true` / `false` — which are predeclared
*identifiers*, since Go has no boolean literal token. Reading the
value out of the syntax means reimplementing Go's constant
evaluator: iota counting, implicit repetition, and constant
folding.

`go/types` has already done that, exactly and with arbitrary
precision, so `enumConstantValue` converts the resulting
`constant.Value` by kind (`Int` → `int64`, or `uint64` past
`MaxInt64`; `Float` → `float64`; `String`; `Bool`). A constant with
no JSON representation (complex) or one the checker could not
evaluate is dropped rather than emitted as a null member.

**The degraded reading.** When the package only partially
type-checked (see `ErrDegradedLoad`), a constant may have no value
in `Defs`. Rather than let an annotated enum vanish,
`enumValue` falls back to the literal syntax — a lone literal,
optionally signed, with rune literals and raw/escaped strings
handled. It is a strict subset: `iota`, expressions and references
are invisible to it by construction, and its values keep the kind
their literal implies rather than the kind of their declared type.
The builder's `validations.CoerceConstant` closes that last gap —
see
[§enum-const-values](../builders/validations/README.md#enum-const-values).

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

## <a id="loader"></a>§loader — choosing the package loader

**Experimental.** Leaving `ToolchainFreeLoader` false and `FS` nil keeps the
historic `go/packages` behaviour unchanged; everything below applies only once
one of them is set.

### Which one, and why

Three ways to get a package graph. The short version, before the mechanism:

- **Standard loader** (default) — loads your code with the Go toolchain. Maintained
  by the Go team, and the **reference for how patterns and imports resolve**: where
  either of the others disagrees with it, the other one is wrong. Requires Go
  installed, and uses the build cache.
- **Pure-Go loader** (`ToolchainFreeLoader`) — loads your code with our own
  reimplementation. Cuts memory by ~45%, and needs no `go` command and no
  subprocess — though it still reads `GOROOT/src` for the standard library, so it
  wants a Go *installation*, not a runnable toolchain. Modules only. It uses no
  build cache, so **cold costs what warm costs**: about level with the standard
  loader warm, ~30% faster cold, and the only option whose cost is predictable.
  Usually the right pick for CI.
- **Compiled dependencies** (`CompiledDependencies`) — the standard loader, taking
  dependency types from compiled export data instead of their source. Warm, it is
  35–55% faster and 2.5–4× smaller, and it produces the same document, because a
  dependency's declarations are fetched on demand when the spec needs them — see
  [§compiled-dependencies](#compiled-dependencies).
  It must **compile** the closure, not merely type-check it, so on a cold cache it
  is *an order of magnitude slower* and writes a large build cache. Opt in where the
  cache is warm by construction — a developer's machine, a watch loop, a pipeline
  that restores it. Off by default because a CI runner is cold by construction, and
  that is the case a default should protect.

Two knobs below drop the GOROOT requirement entirely, for environments that have no
Go installation at all (a WASI guest, a browser): `StubStdlib`, which pays for the
reach in fidelity, and `ExportData`, which pays in preparation. See
[What it costs](#what-it-costs).

Figures are from [`internal/benchmarks`](../benchmarks/README.md), which measures two
generated projects that ship with this repository, warm and cold, against the
released versions as well as the working tree. That document carries the tables and
the method. They are indicative, not a promise — the balance moves with corpus size,
and on a small tree the pure-Go loader is *slower* warm than the standard one.

### Choosing

Both strategies live behind one `internal/packages.Loader`; the scanner states a
preference and the loader reconciles it with what the build and the filesystem
allow:

| `ToolchainFreeLoader` | `FS` | strategy | needs a toolchain |
|---|---|--------|-------------------|
| false (default) | nil | `StrategyGoPackages` (`go list`) | yes |
| **true** | nil | `StrategyToolchainFree`, real filesystem | no |
| either | non-nil | `StrategyToolchainFree`, reading `FS` | no |

`FS` forces the toolchain-free strategy without being asked, because `packages.Load`
reaches the filesystem by running `go list` against the real one — a virtual
filesystem is not something it could honour. So there is no coherent
configuration being overridden, and no way for the two settings to contradict
each other. That override lives in the loader, not here, so there is one place
to get it wrong rather than two.

The flag exists separately because the interesting case is the middle row: running
codescan's own strategy over an ordinary tree on disk. Before it, the only way to
reach it was to hand it a virtual filesystem.

Two consequences of that override are worth stating, because both have bitten:

- an option only one strategy can honour is **dropped, not refused**, when the other
  one runs. Compiled dependencies are the case — only the go command takes
  dependency types from export data, so under `FS` nothing does. This is why the
  announcement is driven by `Loader.Strategy()` and not by the request: announcing it
  from the request is how a toolchain-free scan came to be told its types were
  compiled while it read every one of them from source. A request the resolved
  loader cannot meet is a Warning rather than silence, since the caller chose it for
  the speed-up and did not get it.
- `FS` is **the whole world the scan can read**, not just the tree being scanned —
  see [§virtual-filesystem](#virtual-filesystem).

<a id="virtual-filesystem"></a>

### `FS` is the whole world

Dependencies and the standard library are read through `FS` as well. Neither GOROOT
nor the module cache lives inside a module, so a filesystem holding only the module
under scan resolves neither: every import outside it is synthesized from the names
selected through it, and the spec comes out **valid and quietly thinner** — a lost
`format`, a lost byte-array rendering.

That is announced rather than fatal: one `scan.synthesized-import` per unresolved
import, plus `scan.degraded-load`. Measured on the petstore, `FS` rooted at the
testdata module against the same scan on the real filesystem:

| | definitions | `orderedAt.format` | diagnostics |
|---|---|---|---|
| real filesystem | 4 | `date-time` | none |
| `FS` at the module root | 4 | *(empty)* | `degraded-load` ×1, `synthesized-import` ×5 |

Both scans succeed. The difference is entirely in what the document says.

**Paths.** Everything the scan derives is interpreted against the root of `FS`,
following `io/fs` conventions — slash-separated and unrooted. An absolute path is
mapped onto that convention by dropping its leading separator, which is what lets
an unrooted tree (`embed.FS`, `fstest.MapFS`) be used at all. It is a convention
rather than a heuristic: `fs.FS` exposes no way to tell a rooted tree from an
unrooted one, so there is nothing to detect.

The corollary is that a tree meaning to serve GOROOT or the module cache has to
**mirror their absolute layout** beneath its own root — `/usr/lib/go/src/…` is
looked up as `usr/lib/go/src/…` — and is therefore tied to the host it was recorded
from. Which is precisely why the other two options exist: `ExportData` carries the
compiler's own types instead of its source (no fidelity cost, valid only for the
toolchain that produced it), and `StubStdlib` needs nothing mounted at all (reach
over fidelity).

### The embed.FS recipe

`FS` was built for `embed.FS` first, and what can be embedded decides the shape.
A module's own source and its `vendor/` directory can, since both are inside the
module; GOROOT cannot. So the two halves come from different places:

| half | how |
|---|---|
| module source + non-stdlib dependencies | `go mod vendor`, then embed the module — read and parsed like any tree |
| the standard library | `ExportData`, which is itself an `fs.FS` and embeds alongside |

Both ship in one binary, and no absolute-path mirroring is needed — this is the
configuration that sidesteps the host-tied recorded tree entirely.

Vendoring is doing real work here rather than being a packaging detail: a vendored
dependency is read **from inside the tree**, so it keeps its own annotations. A
vendored `go-openapi/strfmt` still supplies its `swagger:strfmt` marks, where a
dependency reduced to compiled types would have lost them.

`TestEmbeddedTree_SourceAndVendorFromFS_StdlibFromExportData` pins the composition,
and both halves are load-bearing: dropping the blob synthesizes the standard
library, dropping the vendored source loses the dependency's `format`.

`StubStdlib` substitutes for the second row where no blob can be produced, at the
fidelity cost documented on that option.

Putting the switch inside the loader is also what keeps the options honest. `GOOS`
/ `GOARCH` are the example: the go command reads the target from the environment
and nowhere else, so a target that is merely stored applies under one strategy and
vanishes under the other. The loader pushes it into the child environment itself
(`golist.go`), and `TestGoPackagesStrategyHonoursTarget` fails if it stops.

### What the two loaders agree on

Across the fixture corpus the two produce **byte-identical specs** — 378 package
trees, mounted at `/` so `GOROOT` and the module cache stay reachable. That
equivalence is the point: the loader is a substitution, not a second dialect.

Mount narrower than the dependencies reach and the agreement stops, because
unreachable imports get synthesized rather than read. That is not a loader bug,
it is `StubStdlib`'s failure mode arriving uninvited — see below.

Note that agreement alone never proves the *choice* was honoured: with a no-op
flag both sides run the same loader and agree trivially. What witnesses the
routing is a behaviour only one loader has — `StubStdlib`, which the go/packages
path documents as inert (`TestLoaderChoice_SelectsTheLoader`).

### What it costs

- **`StubStdlib`** trades fidelity for reach: a synthesized type has the right
  package path and name (so `time.Time` is still a date-time) but no fields and
  no method set — so `json.RawMessage` stops rendering as a byte array, and no
  type is seen to implement `encoding.TextMarshaler`. Quiet by nature: the spec
  comes out thinner rather than erroring, which is why every synthesized import
  raises `scan.synthesized-import` (Warning when unresolved, Hint when withheld
  on purpose).
- **`ExportData`** costs no fidelity — the types are the ones the compiler
  computed — but is valid only for the toolchain that produced it. A package it
  does not cover falls back to source, and then to synthesis. See
  [§export-data](#export-data).

### The go environment: `GOOS` / `GOARCH` / `GOFLAGS` / `GOWORK` / `GOEXPERIMENT`

Not loader-specific: both strategies honour all five, `go list` through the
child environment. Each decides *what* gets built rather than where the output
goes, so each changes the emitted spec exactly as `BuildTags` does.

They are options rather than inherited state on purpose. A scan that picks them
up from whatever shell it started in is not reproducible, and — as `GOOS` proved
before `3294669` — an inherited value is easy to apply on one code path and
forget on another. Empty still means "whatever the environment says", which is
what the go command would do.

- `GOOS`/`GOARCH` — the platform. Inside a WASI guest the default is `wasip1`,
  which drops every `_linux.go` file without a word.
- `GOFLAGS` — default command-line flags (`-tags=integration`). `BuildTags` wins,
  as an explicit flag does for the go command.
- `GOWORK` — `off`, a path to a `go.work`, or empty to search upwards. Inside a
  workspace a sibling module resolves to the copy being worked on rather than to
  the module cache. Missing that does not fail: the import is looked up in the
  cache, missed, and **synthesized**, so the sibling arrives with no fields and
  no method set.
- `GOEXPERIMENT` — each enabled experiment contributes a `goexperiment.<name>`
  build tag. The baseline is the configuration the codescan binary was built with,
  since `go/build` computes `ToolTags` at init from its own configuration and
  cannot be asked about another toolchain — exact when both come from the same
  Go release, approximate otherwise.

### Pattern resolution

`...` stops at a module boundary, as it does for the go command: a nested `go.mod`
is a different module and its packages are not part of the pattern. Import paths
are derived from the module that actually **contains** the directory, not from the
main module — on this repo, deriving them from the main module turned 25 packages
into 468 and labelled fixture corpora, sibling modules and a vendored theme as
belonging to `github.com/go-openapi/codescan`.

Naming a nested module's package explicitly (`./sub/pkg`) is refused by the go
command and allowed here, with the nested module's own path. Scanning a
subdirectory module directly is a reasonable thing to ask of a scanner; answering
with a well-formed path that names a package which does not exist is not.

Vendoring follows the go command's own test — `vendor/modules.txt`, not the directory —
so a tree that merely contains a `vendor/` folder is not treated as a vendored build, and
`-mod=mod` opts out. A wildcard reaches the directory named `vendor` but never inside it.

A bare pattern (no `./`) is an import path, never a directory, which is what `go list`
means by it. `...` is a wildcard anywhere in the pattern, not only as a trailing element.

Two differences run in codescan's favour and are kept on purpose: a tree with **no module**
still scans (the go command refuses one outright), and another module's **`internal/`**
packages are readable (the go command rejects the import, which makes a scan fail
altogether). A module-less tree names its packages relative to the scan directory, never by
absolute path — a package path reaches the output through `x-go-package`.

An unreadable `go.mod` is fatal rather than degraded. The alternative places no requirement
at all, so every dependency falls through to synthesis and the real fault disappears behind
a wall of `scan.synthesized-import` warnings.

A dependency is placeable when the main `go.mod` names it — a `require`, a `replace`, a
workspace `use`, or the vendor tree. The module graph is never walked, and no dependency's
own `go.mod` is consulted for requirements, so what any dependency declares as its minimum
Go version does not enter into resolution: one set of rules applies across the board.

That list is complete when the main module is at go ≥ 1.17 and tidy, since the go command
then guarantees an explicit require for every module providing a transitively-imported
package. Where it is not — a main module at go 1.16, or a go.mod that needs updating — the
dependency's directory is simply absent and synthesis reports it as a warning. Known
remaining gaps are tracked in `.claude/plans/loader-vs-gopackages.md`.

## <a id="export-data"></a>§export-data — preparing it offline

Parsing and type-checking dependencies is most of what a load does. The compiler
already did that work and wrote the answers down, so the fast path is to read them
instead.

`hack/genexportdata` produces the tree:

```sh
go run ./hack/genexportdata -out ./exportdata std          # the standard library
go run ./hack/genexportdata -out bundle.zip std ./...      # a single-file archive
go run ./hack/genexportdata -dir hack/genexportdata/bundle -out ./exportdata std ./...
```

Point `Options.ExportData` at the result. It is read as an `io/fs`, so a WASI guest
can be handed the tree — or the zip, since `archive/zip`'s reader is already an
`fs.FS` — without a toolchain, a GOROOT, or a module cache anywhere in sight.

Underneath it is `go list -deps -export`, which builds whatever is missing and
reports the build-cache path per package. The tool is worth the indirection for
three reasons: it copies out the **export section** rather than the whole compiled
archive (the standard library is 9.3MB instead of 97MB), it can write a zip, and it
declines to bundle a package whose meaning lives in its comments — see below.

`hack/genexportdata/bundle` is a module whose only content is a list of imports. It
names what a published archive covers, so the set is reviewable in one place and
its versions are pinned independently of the module being scanned. It is
deliberately outside `go.work` for that reason, which is why the tool runs
`go list` there with `GOWORK=off`.

### Annotated dependencies

Export data holds types and not comments, so the loader decides per dependency and
decides whole: **a package whose source carries `swagger:` is read from source, in
the ordinary way; everything else comes from export data and is never parsed.**
Little is given up — the saving was never in the handful of packages a scan
actually reads, it is in the closure behind them. Nothing is lost either, but not
by this rule alone: it settles what a dependency *says*, and the section below
settles what it *declares*.

Putting the two halves back together — export-data types with parsed syntax
bolted on beside them — was once impossible, and that is no longer why the loader
avoids it. The builders used to read `types.Info.Types`, whose entries cannot be
constructed outside `go/types` (the field distinguishing a type from a value is
unexported), so a package assembled that way handed them declarations to read and
no record of what those declarations denote: not a quieter scan but a panicking
one. They have since been taken off that map — see
[§Not `types.Info.Types`](#not-typesinfotypes) — and a spec builds identically
with `types.Info` reduced to `Defs` alone.

That is what the go/packages strategy runs on, because it has no per-dependency
lever at all: a `LoadMode` is one value for the whole load, with no hook to say
"except this one". So it takes every dependency from export data and then hands
the source back to the few that were worth reading — the same policy reached from
the other end, after the load rather than during it. The cheap load still says
where the source *is* (`compiledDepsMode` keeps `NeedFiles`), so this is parsing
on known paths and nothing is type-checked twice.

The toolchain-free strategy has no use for the assembled form during the load: it
already has the source, and reading an annotated dependency in full costs one
type-check of a small package. It reaches the assembled form afterwards all the
same, through the on-demand read-back below, which is why both routes now agree
with an ordinary scan rather than only with each other.

<a id="compiled-dependencies"></a>

#### What a dependency says, and what it declares

The marker scan answers the first question, and it is the only one it can answer:
a `swagger:strfmt` mark is in the dependency's own source or nowhere, so finding
one means reading. It is the wrong question for the second. Any dependency,
annotated or not, may declare a type the scanned code goes on to name as a model —
and a definition renders from its declaration or not at all, so its doc comment,
field tags and per-field annotations all hang on source nothing in that source
asked to have read.

So the declaration is fetched **at the lookup that wants it**
(`ScanCtx.readBackOnDemand`), by parsing that one package and bridging it to the
export-data scope. A dependency nothing reaches into is still never read, which is
what keeps this affordable: the cost is one parse per declaration wanted, against
one per dependency loaded.

The version of this that looks obvious is the one that fails, on both counts.
Reading back every non-stdlib dependency up front — the line a std-only bundle
draws — brings the definition back **empty and unwarned**, which is worse than its
absence because `GetModel` then succeeds and the warning that named it stops
firing. It also costs a third of the wall clock and a third of the peak RSS, which
is most of the reason to choose the option at all.

Empty, because struct fields were located by **position**, and the two halves do
not share one. The `*types.Var` comes from export data; `decl.File()` is the AST we
parsed. Both reach one `FileSet` and get a `token.File` each for the same filename,
so no position of one indexes the other and every field is skipped in silence.

Export data preserves the filename and the **line**, never the column, which its
fabricated line table pins at 1. Line plus the object's own name
identifies the field, since a struct cannot declare a name twice and the file is
already known; embedded fields match on the last identifier of their type
expression, which is what go/types names them after. That is
`resolvers.FindASTFieldFor`, reached only when the position lookup fails, so an
ordinary scan never walks a file twice.

Both halves are needed, and together they close every known divergence: the loader
agreement A/B over the whole fixture corpus is empty for all three configurations
(`internal/integration/loader_agreement_test.go`). That A/B holds the **source**
scan as its reference, which is what plain `Options` give — a test asking whether
the shortcut changes anything must not have it on both sides.

#### Code that does not build

Closing the fidelity gap made this option usable at all, and it left one
thing to handle that has nothing to do with fidelity.

`go list -export` **builds** the packages it is asked about. So a scanned package
that does not compile comes back as a package that could not be loaded at all — a
`ListError`, which aborts the scan — where an ordinary load reports a type error on
a package whose definitions are still perfectly usable. That is go-swagger#2874
exactly: one non-building package sinking a whole `./...` scan. Scanning a tree
mid-edit is the ordinary case, so opting into a shortcut must not reintroduce it.

The fast path is therefore abandoned rather than allowed to change the answer:
`loadPackages` retries from source when the compiled load reports a `ListError`,
and says so with a `scan.compiled-dependencies` Hint. The retry costs a second
load, and only on a tree that was not going to build; a healthy tree pays nothing.
A caller who sees the Hint every run wants `CompiledDependencies` unset, which skips
the wasted first attempt.

Note that dependencies need no such treatment — go/packages already type-checks a
dependency from source when its export data is missing, so a dependency that does
not build degrades on its own. It is the **scanned** packages that needed this.

`TestNewScanCtx_NonBuildingCode_FallsBackToSource` pins the fallback, and
`TestNewScanCtx_PartialLoad_WarnsAndContinues` — the original #2874 witness — fails
without it.

`genexportdata` mirrors that, skipping annotated packages by default and saying so:

```
skipping github.com/go-openapi/strfmt: carries swagger annotations, which export
data cannot hold — it has to be read from source
```

`-with-annotated` includes them anyway. That matters only for an archive shipped
**without** source — the WebAssembly case — where having the types is better than
having nothing, and the lost annotations are announced per package. Where the
source does ship, the entry is simply unused.

### What it buys, and what it does not cost

Full scans of `testdata/goparsing/petstore` through the toolchain-free loader:

| | scan | spec |
|---|---|---|
| source only | 748 ms | — |
| bundle, `strfmt` skipped | **79 ms** | byte-identical |
| bundle, `-with-annotated` | 81 ms | byte-identical |

Identical output is not a happy accident, it is the point: a dependency that has
anything to say is read the same way it always was. The two bundle rows differ by
noise because `strfmt` is read from source in both — with the source present, its
export-data entry is never consulted.

Where a dependency's source cannot be found the types still stand, and a
`scan.compiled-dependencies` Hint names the package rather than letting the spec
quietly say less.

The data is only valid for the toolchain that produced it, since the export format
tracks the Go release. Regenerate it when the toolchain moves.

### The go/packages loader benefits too

The go/packages loader reads the same data straight from the build cache, so its
preparation is simply having built the project — the cold-cache penalty measured
elsewhere is not extra work, it is the work a normal `go build ./...` already did.

The two reach the source by different routes and end up in the same place. The
toolchain-free strategy has it in hand during the load, so an annotated dependency
is read from source there and then. `go list` hands over compiled types with no
syntax, so the same packages get their source handed back after the load
(`attachAnnotatedDependencies`). Either way, what neither route read is fetched at
the lookup that wants it, which is why both agree with an ordinary scan.

## <a id="quirks-open"></a>§quirks-open — deferred follow-ups

> **Where open quirks live.** This section documents caveats *of this package*.
> The project-wide register of what is actually open — verified, with the stale
> historical registers called out — is `.claude/plans/quirks-open.md`.

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
