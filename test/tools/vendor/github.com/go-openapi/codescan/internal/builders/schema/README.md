# schema builder — maintainer notes

This document is the long-form companion to the schema builder code.

The source files keep godoc concise; complex invariants, design
trade-offs, and known quirks live here.

---

## Table of contents

- [§build-entry](#build-entry) — `Build` modes and dispatch entry points
- [§dispatch-table](#dispatch-table) — `buildNamedType`'s underlying-shape table
- [§dissolve-named](#dissolve-named) — when a named type is unwrapped instead of `$ref`'d
- [§special-types](#special-types) — `applyStdlibSpecials`, `applySpecialType`, the two UUID recognizers
- [§textmarshal-order](#textmarshal-order) — `buildFromTextMarshal` precedence
- [§aliases](#aliases) — `TransparentAliases` vs `RefAliases` vs default-expand
- [§discovery](#discovery) — `Models` / `ExtraModels` / `discovered`, dedup layers
- [§struct](#struct) — `buildFromStruct` two-pass shape
- [§allof](#allof) — `buildAllOf`, `buildNamedAllOf`, `scanEmbeddedFields`
- [§embedded](#embedded) — embed routing, struct/interface specials asymmetry
- [§embed-depth](#embed-depth) — ambiguous-embed diagnostic mechanism
- [§opaque-streams](#opaque-streams) — stream types, and the two answers a stream can take
- [§math-big](#math-big) — the arbitrary-precision numbers, and why the three disagree
- [§omit](#omit) — `swagger:omit` — the author's pre-filter on promoted fields
- [§json-dash](#json-dash) — what `json:"-"` does, and the two shapes it is confused with
- [§method-mangler](#method-mangler) — interface-method JSON-name derivation
- [§user-overrides](#user-overrides) — explicit user-driven type/format overrides at decl-site and field-site
- [§traceability](#traceability) — `x-go-name` / `x-go-package` / `x-go-type` origin extensions and `EmitXGoType`
- [§additional-properties](#additional-properties) — the `swagger:additionalProperties` decl-level marker
- [§pattern-properties](#pattern-properties) — the typed `swagger:patternProperties` decl-level marker
- [§ref-override](#ref-override) — `applyToRefField`, the allOf-on-$ref shape, `refOverrideCollector`, `applyPattern`
- [§simple-schema-mode](#simple-schema-mode) — the SimpleSchema build mode for OAS v2 non-body params and response headers
- [§classifier-walkers](#classifier-walkers) — per-call-site classifier walkers and `findAnnotationArg`'s single-word filter
- [§quirks](#quirks) — known behavioural caveats
  - [§quirks-resolved](#quirks-resolved) — ✅ fixed in this refactor
  - [§quirks-open](#quirks-open) — 🟡 deferred / 🟦 documented behaviour

---

## <a id="build-entry"></a>§build-entry — `Build` modes and dispatch entry points

`Builder.Build` has three modes selected by [`Option`](options.go):

| Option | Sets | Entry point | Output destination |
|---|---|---|---|
| `WithDefinitions(map)` | `s.definitions` | `buildFromDecl` | `map[s.Name]` |
| `WithType(tpe, tgt)` | `s.inputType`, `s.target` | `buildFromType(inputType, target)` | `tgt` (caller-owned, full Schema) |
| `WithSimpleSchema(tpe, tgt, in)` | `s.inputType`, `s.target`, `s.simpleSchema`, `s.paramIn` | `buildFromType(inputType, target)` + exit validator | `tgt` (caller-owned, SimpleSchema-shaped) |

`WithDefinitions` and the `WithType`/`WithSimpleSchema` pair are
mutually exclusive; `Build` panics on misuse. `WithDefinitions` is
the spec-orchestrator entry point for top-level type declarations.
`WithType` produces a full OAS v2 Schema for body parameters,
response bodies, and any other site that owns a `Typable` and accepts
the full schema vocabulary. `WithSimpleSchema` produces an OAS v2
SimpleSchema for non-body parameter sites and response headers — see
[§simple-schema-mode](#simple-schema-mode) for the contract and the
exit-validator's role.

`buildFromDecl` does three things in order:

1. Consume the decl's doc-comment block (may short-circuit on `swagger:ignore`).
2. Defer `annotateSchema` (`x-go-name`, `x-go-package` extensions).
3. Intercept stdlib special types ([§special-types](#special-types))
   before kind-dispatch. Necessary because stdlib decls can reach
   `buildFromDecl` via the discovery chain (e.g. `type X = time.Time`
   pulls `time.Time` itself into `discovered`).
4. Dispatch on `s.Decl.ObjType()`: `*types.Named` → `buildFromType(ti.Type, …)`;
   `*types.Alias` → `buildDeclAlias`; otherwise warn-and-skip.

Inside the `*types.Named` arm, an override classifier runs first, selected by
`Underlying()` kind — struct, basic, array and slice each have an arm. A
`swagger:model` type carrying an override publishes the **full** override schema
on its own definition here; field sites then `$ref` it, and
`buildNamedType`'s `refModel` gate skips the inline classifiers precisely
because this step is assumed to have run.

That assumption is why the array and slice arms matter: while they were missing,
a `swagger:model` sequence with a `swagger:strfmt` published a definition with
the format silently dropped, and no later stage could recover it. `buildDeclAlias`
applies the same rule on the alias side, so `type ID = [16]byte` and
`type ID [16]byte` publish the same definition.

---

## <a id="dispatch-table"></a>§dispatch-table — `buildNamedType`'s underlying-shape table

`buildNamedType` handles a `*types.Named` reaching as a field/embed/etc.
type (not a top-level decl). The body is a table indexed by `Underlying()`
kind, with each arm following the same three-step shape:

1. **shape-local pre-check** (e.g. `UnsupportedBuiltinType` for `*types.Basic`)
2. **classifier walk** — short-circuit on a match, may recurse on Underlying
3. **`FindModel` pivot** via `resolveRefOr` / `resolveRefOrErr` — hit ⇒ `makeRef`, miss ⇒ shape-specific fallback

The arms:

| Underlying | Classifier | On-miss fallback |
|---|---|---|
| `*types.Struct` | `classifierNamedStructStrfmt` | silent — `nil` |
| `*types.Interface` | — | `missingSource` error |
| `*types.Basic` | `classifierNamedBasic` (cascade) | `SwaggerSchemaForType(name)` |
| `*types.Array` / `*types.Slice` | `classifierNamedArrayLike(forSlice)` | inline element via `buildFromType` |
| `*types.Map` | — | silent — `nil` |

`buildNamedArrayLike` is the unified `Array`/`Slice` helper; the slice
arm passes `forSlice=true` so `classifierNamedArrayLike` can honour
the `bsonobjectid` slice-only special case.

---

## <a id="dissolve-named"></a>§dissolve-named — when a named type is unwrapped instead of `$ref`'d

After the prelude but before the `Underlying()` switch,
`buildNamedType` has a short-circuit:

```go
if s.Decl.Spec.Assign.IsValid() || (titpe.TypeArgs() != nil && titpe.TypeArgs().Len() > 0) {
    return s.buildFromType(titpe.Underlying(), target)
}
```

Two disjuncts, both meaning "this named type has no source-level
`TypeSpec` of its own to `$ref` — emit its structural form inline."

### Disjunct 1 — `s.Decl.Spec.Assign.IsValid()`

This is **the TransparentAliases plumbing mechanism**. When the
outer Builder was constructed for an alias-syntax decl (`type X = Y`),
`Spec.Assign` is the position of the `=` token (valid). Every named
type reached during this Build is then inlined rather than `$ref`'d —
which is what `TransparentAliases` semantically means.

It also covers the gotypesalias=0 legacy mode (`type X = Y` surfacing
as `*types.Named` directly), but that's an incidental side-benefit.

Example: under `TransparentAliases=true`, a body parameter aliased as
`type AliasBody = Payload` causes `schema.Builder` to dissolve
`AliasBody` → walk `Payload`'s underlying struct → emit `{type:object, properties:{…}}`
inline on the body param. Without the disjunct, `buildNamedType` would
emit `{$ref: "#/definitions/Payload"}` — wrong for the dissolve intent.

### Disjunct 2 — `titpe.TypeArgs().Len() > 0`

Generic instantiations like `GenericSlice[int]`. The instantiated
`*types.Named` has no source-level `TypeSpec`; only the generic
declaration (`GenericSlice[T any] []T`) has one. Unwrapping to
`Underlying()` substitutes type params with concrete types
(`[]int`) so the schema reflects the substituted shape.

Fixture: `testdata/enhancements/generic-instantiation/`.

---

## <a id="special-types"></a>§special-types — `applyStdlibSpecials`, `applySpecialType`, the two UUID recognizers

Three layers, all in `special_types.go`:

- **`applySpecialType(obj, target, recognizers...)`** — the engine.
  Iterates `recognizers` and applies the first match. Each recognizer
  is identity-based (exact `Pkg.Path` + `Name` match) except
  `recognizeUUID` which is **fuzzy** (case-insensitive name match).

- **`applyStdlibSpecials(obj, target)`** — the canonical safe set
  `{recognizeAny, recognizeTime, recognizeError, recognizeRawMessage,
  recognizeStdUUID, recognizeOpaqueStream, recognizeMathBig}`. All seven are
  identity-based and cannot misfire on user types, so this helper is
  **called uniformly at every site** that handles a `*types.TypeName`.
  `recognizeMathBig` is additionally passed by `buildFromTextMarshal`,
  the only recognizer in both lists — see [§math-big](#math-big) for why
  membership in one alone would split the answer by pointer-ness.

**What the set deliberately excludes.** A recognizer asserts that a
type means one thing on the wire wherever it appears, so one may only
be added where that holds for every use. `time.Duration` is the
standing counter-example: it counts nanoseconds, and whether an API
means nanoseconds, seconds or an ISO 8601 string is the author's
decision rather than a property of the type — which is why go-openapi
publishes `strfmt.Duration`, carrying a stated wire format, instead of
teaching this set to guess at one. `io.Writer` stays out of the
opaque-stream table for the same reason, see
[§opaque-streams](#opaque-streams).

A consumed type with no recognizer is therefore **reported, not
resolved by assumption**: where its declaring package was served
without source, the scanner says the declaration could not be read,
and the author states what they meant with a strfmt type or an
annotation.

The two UUID recognizers are a **certain/guessed pair**, and the
distinction is the whole reason both exist:

- **`recognizeStdUUID`** — identity match on the go1.27 stdlib
  `uuid.UUID` (`resolvers.IsStdUUID`: import path `uuid`, name `UUID`).
  Certain, therefore in the safe set. It is **not** behind a
  `//go:build go1.27` tag, on purpose: codescan compares types
  harvested from *scanned* code and never imports `uuid` itself, so
  tagging it would mean a codescan built by an older toolchain fails
  to recognize a go1.27 user type — the inverted matrix. Build tags
  live on the fixture and the integration test, which genuinely cannot
  compile before go1.27; the predicate keeps toolchain-independent
  coverage via `TestIsStdUUID`.

- **`recognizeUUID`** — fuzzy, case-insensitive name match, the
  fallback for `github.com/google/uuid`, `gofrs/uuid`, `strfmt.UUID`
  and every other third-party type named UUID. Opt-in via
  `applySpecialType`'s variadic and used **only by
  `buildFromTextMarshal`**, because the upstream `IsTextMarshaler`
  gate guarantees the type renders as text, making the name match
  safe. `buildFromTextMarshal` orders identity **before** fuzzy so
  the certain match wins.

Neither beats an explicit `swagger:strfmt` / `swagger:type` — the
classifier runs first, see [§textmarshal-order](#textmarshal-order).

**A struct embed reaches them through the property path.**
`uuid.UUID` has an array underlying type, so embedding it promotes no
member: `embedPromotes` sends it to `applyFieldCarrier` as an ordinary
property named after the type, and the recognizers run there like on
any other field — `Embedder` carries `UUID: {string, uuid}`. Go's
default marshaller renders the whole struct as a bare string instead,
via the promoted `MarshalText`, which codescan does not model; see
[§embed-marshaller](#embed-marshaller) and Q33. Witnessed by
`TestStdlibUUID`'s embed sub-test.

**`recognizeRawMessage` answers to two names.** go1.27 turned
`encoding/json.RawMessage` into an alias of
`encoding/json/jsontext.Value`, so unaliasing a field written as
`json.RawMessage` now lands on `jsontext.Value`.
`resolvers.IsStdJSONRawMessage` matches both — a predicate reading
only `(encoding/json, RawMessage)` let the type fall through to its
`[]byte` underlying and render as an array of `uint8`. The same
change makes an alias the ordinary shape of a written right-hand side
(`type DefinedRaw json.RawMessage`), which is why the responses
builder's `namedWrittenRHS` unaliases before testing for a declared
type.

The seven call sites of `applyStdlibSpecials` (`buildFromDecl`,
`buildDeclAlias` RHS, `buildAlias`, `buildNamedType`,
`buildNamedEmbedded` interface arm, `processEmbeddedType` named arm,
`buildNamedAllOf` struct arm) **previously varied their recognizer
subsets**. Unification preserved goldens — the narrow subsets were
historical accumulation, not semantic. Identity checks cannot
misfire, so passing the full safe set everywhere is correct by
construction.

---

## <a id="textmarshal-order"></a>§textmarshal-order — `buildFromTextMarshal` precedence

The function is entered from `buildFromType`'s shortcut
`hasNamedCore(tpe) && IsTextMarshaler(tpe)`. Pipeline:

1. peel pointers (self-recurse)
2. route aliases through `buildAlias` (honour `TransparentAliases` / `RefAliases`)
3. type-assert to `*types.Named` (fallback: `{string, ""}`)
4. **classifier (`swagger:strfmt`) — explicit user intent wins**
5. **stdlib recognizers via `applySpecialType(recognizeError, recognizeTime, recognizeRawMessage, recognizeStdUUID, recognizeMathBig, recognizeUUID)`** — identity before fuzzy
6. `PkgForType`-miss bail (gates only the generic fallback below)
7. **generic fallback** — `{string, ""}` + `x-go-type: pkg.Name`

The user-intent-first rule (step 4 before steps 5–6) is the same
shape applied in `buildNamedAllOf` and elsewhere. The order matters
because, e.g., a UUID-named type carrying `swagger:strfmt date`
should emit `{string, date}` — the classifier wins.

Fixtures: `testdata/enhancements/text-marshal/explicit_override/`
demonstrates classifier-beats-heuristic; `text-marshal/uuid_wrapping_time/`
demonstrates heuristic-still-fires when no override exists.

### Why stdlib recognizers run **before** the `PkgForType` bail

`PkgForType` looks `tpe` up in `s.app.AllPackages`. Stdlib packages
(`time`, `encoding/json`) often aren't there — the scanner only
registers packages it was asked to scan. Previously the stdlib
recognizers ran **after** the `PkgForType` bail, so stdlib types
reaching here when their package wasn't in `AllPackages` silently
emitted `{}`. The new order recognizes stdlib types via
`tio.Pkg()` alone (no `PkgForType` call needed) before the bail.

---

## <a id="aliases"></a>§aliases — `TransparentAliases` vs `RefAliases` vs default-expand

Alias handling has two axes: **decl shape** (what does the alias's
own definition look like?) and **use-site shape** (what does a
field / element / body site that references the alias produce?).
The same contract governs the schema, parameters and responses
builders — both at the decl and at the use site — so the rules
described here apply uniformly across all three layers
(see [parameters/README.md](../parameters/README.md#alias-handling)
and [responses/README.md](../responses/README.md#alias-handling)
for the layer-specific dispatches that consume this contract).

### Decl shape — `buildDeclAlias`

`buildDeclAlias` handles top-level alias declarations. Independent
flags with deterministic precedence:

| `TransparentAliases` | `RefAliases` | Outcome |
|---|---|---|
| true | (any) | **Dissolve** — `buildFromType(rhs, target)`. No LHS definition. Wins outright. |
| false | true | **`$ref`** — `makeRef` to the RHS's named target. |
| false | false (default) | **Expand** — `buildFromType(Underlying, target)`. LHS gets a structural definition. |

`swagger:model` on an alias decl forces decl-level registration
unconditionally — even under `TransparentAliases`, an annotated
decl reaches `definitions` (with a structural shape via the
`Spec.Assign.IsValid()` dissolve-named branch). The trade-off is
that under Transparent the annotated decl exists but **nothing
references it** at use sites — the orphan-annotated-decl shape.

The dissolve case propagates to nested named types via the
`Spec.Assign.IsValid()` disjunct in `buildNamedType`
([§dissolve-named](#dissolve-named)).

`swagger:strfmt <format>` and `swagger:type <go-type>` on an alias
decl are honoured at the decl entry — `swagger:strfmt` at the top
of `buildDeclAlias`, `swagger:type` via
`classifierNamedTypeOverride` in `buildFromDecl`. Both write the
canonical shape (`{type: string, format: <format>}` or the
Swagger shape for the named Go type) and short-circuit the
underlying-kind dispatch.

### Use-site shape — `buildAlias` (and the parameters / responses analogues)

`buildAlias` is the use-site handler — called whenever an
alias-typed value is encountered as a struct field, slice / array
element, allOf member, body parameter or response body. The rule
is **annotation-gated** and identical across the three builders:

| Mode | `swagger:model` on the alias? | Use-site shape |
|---|---|---|
| `TransparentAliases=true` | (irrelevant) | Dissolves to the unaliased target. No definition for the alias. Wins outright. |
| Default / `RefAliases=true` | yes | `$ref: <AliasName>` — the alias surfaces in `definitions` with shape per mode (decl shape table above). |
| Default / `RefAliases=true` | no | Dissolves to the unaliased target via `types.Unalias` (full chain collapse in one step). The alias produces no definition entry. |

The use-site `$ref` target is decided by **annotation alone**
(with Transparent as the override) — the mode flags only shape
the alias decl's own definition. A struct field typed with an
annotated alias produces the same `$ref: <AliasName>` under both
Default and Ref; the difference shows up downstream in the
alias's own definition (Expand structural under Default vs chain
`$ref` under Ref).

#### `swagger:strfmt` on the alias runs before the dissolve

Every row of the table above ends in either a `$ref` or a
dissolve, and a dissolve forgets that an alias was involved at
all. So a `swagger:strfmt` carried by the **alias declaration**
has to be applied *before* that point, or it is lost — which is
what used to happen at every use site.

`ClassifierAliasStrfmt` (on `common.Builder`, shared with the
parameters and responses builders) is that entry. It runs on the
alias's own comments, dispatching on the alias's underlying kind
so the format lands where the equivalent **named** declaration
would put it:

| Alias underlying | Where the format lands | Named counterpart |
|---|---|---|
| basic | whole schema | `classifierNamedBasic` |
| struct | whole schema — strfmt replaces the type | `classifierNamedStructStrfmt` |
| slice / array | by element type — see below | `classifierNamedArrayLike` |

#### Items vs. whole schema, for a sequence

A format on an array or slice can describe the sequence itself or each of its
elements. The decision is made by the **element type**, in
`common.IsStringLikeSequence`:

| Element | Meaning | Result |
|---|---|---|
| `byte` / `uint8` | a byte sequence — string-like | format on the **whole schema** |
| `rune` / `int32` | a rune sequence — string-like | format on the **whole schema** |
| anything else | a collection of formatted values | format on the **items** |

```go
type ID     [16]byte  // swagger:strfmt uuid  → {string, format: uuid}
type Emails []string  // swagger:strfmt email → {array, items: {string, format: email}}
```

This replaced a two-name allowlist (`byte`, and `bsonobjectid` for arrays only)
that was standing in for the same question — both are formats for a byte
sequence, `bsonobjectid` being a strfmt library type that happens to have an
array underlying. Keying on the element generalises to every such type (`uuid`
over `[16]byte`, `ulid`, …) with no list to extend, and it removes the
array-vs-slice asymmetry the allowlist had.

go/types cannot tell `rune` from `int32` (rune is an alias), so `[]int32` is
treated alike — harmless, since a *string* format on integer elements was
already a contradiction.

Note this settles only the mechanical half of the question. A format on a
sequence of some other element type is genuinely ambiguous — whether the author
means the sequence or its members cannot be read off the Go type — and it stays
on the items, as it always has.

Two ordering constraints, both load-bearing:

- **Above the `TransparentAliases` return.** The declaration
  lookup was historically below it, so that mode dissolved
  without ever reading the declaration. Lookup and classifier
  both sit above it now, and a not-found declaration is only an
  error on the paths that need it to emit a `$ref`.
- **Before the right-hand side is reached.** For an alias over a
  recognised stdlib type (`type Stamp = time.Time`), the dissolve
  lands on `time.Time` itself, where `applyStdlibSpecials` answers
  `date-time`. An author writing `swagger:strfmt date` must beat
  that: the annotation is the escape hatch for formats the library
  cannot infer, so a recognizer is a default for un-annotated code
  and never an override.

A `swagger:model` alias is the exception, mirroring
`buildNamedType`'s `refModel` gate: it publishes its override on
its own definition and is referenced here, so the inline
classifier is skipped — except under SimpleSchema (`$ref`
illegal) and `TransparentAliases` (no `$ref` emitted), where the
format must inline after all.

An alias that carries **no** annotation of its own is unaffected:
it dissolves as before, and if it lands on an annotated *named*
type the named machinery applies that type's format, exactly as
it always did.

The same applies inside allOf composition: `swagger:allOf` on an
embed governs the *composition* shape (allOf vs flat inline);
`swagger:model` on an embedded alias governs the *identity* of
the allOf member's `$ref` target (alias name vs unaliased target).
They are orthogonal.

### SimpleSchema reach contexts

Non-body parameters and response headers are SimpleSchema targets
and **cannot carry `$ref`** (OpenAPI 2.0 constraint). At those
sites the alias always expands to the unaliased target regardless
of annotation. The annotation gate has no effect for SimpleSchema
because the question "$ref to what" never arises.

A `swagger:strfmt` on the alias still applies, though — the
expansion is a dissolve like any other, so `ClassifierAliasStrfmt`
runs ahead of it in the parameters and responses builders too.
Since `$ref` is illegal here, it runs even when the alias carries
`swagger:model`.

### Top-level alias parameters and responses

When `swagger:parameters` or `swagger:response` is on an alias
declaration, the alias is **transparent re: model creation** in
all three modes — neither the alias nor any chain link of its
backing struct surfaces in `definitions`. The fields of the
unaliased target become the operation's parameters or the
response's body / headers. This clause has no schema-builder
analogue: `swagger:parameters` / `swagger:response` declare a
parameter set or response, not a model, and never promote their
backing chain to the spec's definitions section.

### `Rhs()` vs `Underlying()`

- `tpe.Rhs()` — immediate right-hand side of the alias declaration.
  For `type X = Y`, `Rhs` is `Y` as-is (may itself be a `*types.Alias`
  or `*types.Named`).
- `tpe.Underlying()` — peels through aliases **and** named types to
  reach the structural form (`*types.Struct`, `*types.Basic`, …).
- `types.Unalias(tpe)` — peels through aliases only, leaving Named
  types intact. Used at use sites for full chain dissolve in one
  step.

Example: `type X = Y; type Y = Z; type Z = int` gives
`X.Rhs() == Y (*types.Alias)`, `X.Underlying() == int (*types.Basic)`,
and `types.Unalias(X) == int (*types.Basic)` as well.

The branches use them intentionally:

- **Dissolve (decl)** uses `rhs` — dissolves one layer at a time; if
  the RHS is itself a named/aliased type, build from it (may recurse).
- **Expand (decl)** uses `Underlying()` — fully expand the structural
  shape onto the LHS definition.
- **Dissolve (use site)** uses `types.Unalias` — collapses the whole
  alias chain in one step at the field / element / body position.

---

## <a id="discovery"></a>§discovery — `Models` / `ExtraModels` / `discovered`, dedup layers

The scanner exposes two model indexes (in `scanner.app`):

- **`Models`** — decls carrying `swagger:model` (or response/parameter)
  annotations. The orchestrator builds these unconditionally.
- **`ExtraModels`** — decls discovered transitively that aren't
  annotated but need a top-level definition. Promoted to `Models`
  at consumption (`joinExtraModels` calls `MoveExtraToModel`).

The spec orchestrator also maintains an in-flight queue
`s.discovered` populated via `Builder.AppendPostDecl`:

- **`makeRef(decl, …)`** appends `decl` to `Builder.postDecls`.
- `spec.Builder.buildDiscoveredSchema` runs schema Builds and harvests
  each Builder's `PostDeclarations()` into `s.discovered`.
- `buildDiscovered` then drains `s.discovered` until empty.

`ScanCtx.AddDiscoveredModel(decl)` is the explicit hook for the
`ExtraModels` side (replacing the now-deprecated `FindModel`
side effect).

### Dedup layers

Three layers prevent the same decl from being built twice:

1. **`AppendPostDecl`** — per-Builder dedup keyed by `EntityDecl.Ident`.
2. **`AddDiscoveredModel`** — no-op when the decl is already in
   `Models` (avoids `Models ↔ ExtraModels` bouncing).
3. **`spec.Builder.buildDiscovered`** — per-pass dedup keyed by
   `decl.Names()` string.

Without (3), the `TestCoverage_RefAliasChain` shape regression: the
same alias-target decl, appended via multiple `makeRef` call sites,
got queued twice in one pass, and the second `Build` read the
half-built schema and **appended another set of `allOf` entries** —
doubling them.

---

## <a id="struct"></a>§struct — `buildFromStruct` two-pass shape

`buildFromStruct` emits the schema for a named Go struct in two
passes over the same `*types.Struct`. The split is required because
`allOf` composition (driven by `swagger:allOf` on embedded fields)
can change *which* schema receives the property map.

1. **Pass 1 — `scanEmbeddedFields`.** Walks every anonymous field.
   Each embed is classified:
   - **plain embed** (no `swagger:allOf`, embed is not an
     `*types.Alias`): its fields are merged into the outer schema
     directly via `buildEmbedded`. Returned `target == schema`.
   - **allOf embed** (`swagger:allOf` present, or the embed is an
     alias): the embedded type becomes an entry in `schema.AllOf`,
     and a *fresh* schema is allocated as the target for the
     property map. Returned `target == fresh schema`.
2. **Pass 2 — `buildStructFields`.** Iterates non-anonymous
   exported fields and emits each via `applyFieldCarrier`.

If `hasAllOf` is true and the fresh target ended up with
properties, the target itself is appended to `schema.AllOf` so the
inline properties live as their own compound member alongside the
embedded `$ref`s.

### Why `target.Typed("object", "")` always fires

The line runs unconditionally after target selection. Reason: a
struct with zero exported fields and zero embeds still emits as
`{type: object, properties: {}}` — distinguishable from a missing
schema. SimpleSchema (forthcoming with M1) introduces the only
path where this line would *not* fire; the current code is
Full-Schema only.

### User-classifier short-circuit

`classifierStructPreBuildType` runs at the very top of
`buildFromStruct`. It consumes only `swagger:type` on the decl's
own comment group. On match, the struct walk is skipped entirely
— the schema is whatever `swagger:type X` resolves to via
`SwaggerSchemaForType`. See [§user-overrides](#user-overrides) for
the cascade and the unknown-leaf fallthrough handled by the
caller (`buildFromDecl`), not here.

---

## <a id="allof"></a>§allof — `buildAllOf`, `buildNamedAllOf`, `scanEmbeddedFields`

OAS 2.0 allOf composition surfaces in three places in the schema
builder: `scanEmbeddedFields` (decides which embeds become allOf
members), `buildAllOf` (peels and dispatches an allOf member type),
and `buildNamedAllOf` (resolves a named-type allOf member through the
same user-classifier-first precedence the rest of the builder uses).

### `scanEmbeddedFields` — embed classification

**Which `fieldDoc` signals an embed consumes.** Exactly five: `Ignored`
(`swagger:ignore`), `JSONName` (`swagger:name`, via `embedNestName`),
`OmitTargets` (`swagger:omit`, via `embedOmitTargets`), `IsAllOfMember` /
`AllOfClass` (`swagger:allOf`), and the `required:` inheritance hint read by
`buildPlainEmbed`. All five describe the *embedding*.

`StrfmtName` and `TypeOverride` are parsed into the same `fieldDoc` and are
**never read here** — they describe the embedded type, whose shape comes from
its own declaration, not from the site that embeds it. Writing either on an
embed raises `CodeIneffectiveAnnotation` (`warnIneffectiveEmbedAnnotations`)
rather than being dropped in silence: both ARE honoured on a regular field
(`fields.go`), so the same annotation one field over means something, and the
scanner rejects an *unknown* annotation in that same comment — so without the
warning the author gets validation feedback implying it took effect.

Walks `*types.Struct`'s anonymous fields. Three signals decide
classification per embed:

- `swagger:ignore` — embed skipped entirely.
- `json:"-"` — embed skipped (parity with v1's JSON tag handling).
- `swagger:allOf` (via `fieldDoc.IsAllOfMember`) **or** the
  embedded type is `*types.Alias` — embed becomes an allOf member;
  remaining properties land on a fresh target schema.
- otherwise — plain embed, handled by `buildPlainEmbed`, which splits on
  whether the embed carries an explicit name:
  - an explicit json tag name (`Inner `json:"inner"``) — or a
    `swagger:name` — makes the embed **nest** under that name as a
    single regular property (a `$ref` when the embedded type is a model),
    matching Go's `encoding/json`, which treats a named embed as an
    ordinary field rather than promoting it (go-swagger#2038).
  - no explicit name — properties merge (promote) into the outer schema.

**`DefaultAllOfForEmbeds` (opt-in).** When `Options.DefaultAllOfForEmbeds`
is set, a plain embed with **no explicit name** is reclassified as an
allOf member — exactly as if it carried `swagger:allOf` — so it takes the
`buildAllOf` path ($ref when the embedded type is a model, inline member
otherwise) and the embedding struct's own fields move into a sibling allOf
member. The decision lives in `scanEmbeddedFields` (`isAllOf` is forced
true via `embedNestName(afld, fd) == ""`); a json-named embed keeps its
nested-property shape (go-swagger#2038), an explicit `swagger:allOf` is
already in this shape, and interface embeds are out of scope (they compose
via allOf regardless — see [§embedded](#embedded)). Default off ⇒ output
unchanged. Pinned by `testdata/enhancements/default-allof-embeds/` +
`integration/coverage_default_allof_embeds_test.go`.

The `swagger:allOf` arg, when present, is recorded as
`x-class: <arg>` on the outer schema (`fd.AllOfClass`). This is the
discriminator hint downstream go-swagger consumes.

### `buildAllOf` — three-arm peel

Strips pointers (recurses), routes `*types.Named` through
`buildNamedAllOf`, routes `*types.Alias` through `buildAlias`. Any
other input is dropped with a `validate.unsupported-go-type` Warning
diagnostic (`warnUnsupportedGoType`) — v1 had no surface for non-Named /
non-Alias allOf members.

### `buildNamedAllOf` — symmetric arm dispatch

Struct and interface underlyings share the same precedence shape:

1. **classifier first** — `classifierAliasTargetStrfmt(ftpe, tgt)`.
   On match the named type is emitted as `{string, <format>}` and the
   walk terminates. Same shape as
   [§textmarshal-order](#textmarshal-order)'s step 4.
2. **stdlib specials** — `applyStdlibSpecials(tio, tgt, skipExt)`.
   Identity-based, cannot misfire. Catches `time.Time`, `error`,
   `json.RawMessage`, `any`/`interface{}` if any of them reach as an
   allOf member.
3. **model lookup** — `Ctx.GetModel(pkg, name)`. Missing decl is a
   build error (the allOf member must be resolvable).
4. **`HasModelAnnotation()` → `makeRef`** — annotated types become
   `$ref` entries. The struct/interface body is built lazily via
   `discovered`.
5. **inline build** — `buildFromStruct` / `buildFromInterface` on the
   underlying. Used when the type is reachable but not
   `swagger:model`-annotated.

Both arms route through the same `tgt := NewTypable(schema, 0, skipExtensions)`
target, avoiding the v1 asymmetry where the struct arm used
`classifierAliasTargetStrfmt` (decl-fetched internally) and the
interface arm used a comment-group-keyed variant (`classifierAliasOwnDocStrfmt`,
since deleted) that pre-fetched the decl. The unified target lets
both arms run classifier-first without doing the model lookup
upfront — earlier classification means no orphan `ExtraModels`
side effect for strfmt-tagged interface types.

### Error message on missing decl

The missing-decl error is now uniform across arms:
`"can't find source for named allOf member %s: %w"`. Previous
phrasing differentiated struct vs interface; no test asserts on
the text, the change is golden-neutral.

---

## <a id="opaque-streams"></a>§opaque-streams — stream types, and the two answers a stream can take

`resolvers.IsOpaqueStream` recognizes the named types that carry a stream of bytes:

| Package | Types |
|---------|-------|
| `io` | `Reader`, `ReadCloser`, `ReadSeeker`, `ReadSeekCloser`, `ReadWriter`, `ReaderAt`, `ReaderFrom`, `LimitedReader`, `ByteReader`, `ByteScanner` |
| `mime/multipart` | `File` |
| `github.com/go-openapi/runtime` | `NamedReadCloser` |

`recognizeOpaqueStream` sits in the canonical `ApplyStdlibSpecials` set, so it fires at every site
that handles a `*types.TypeName` — model field, parameter, response body, response header — and it
runs **before** any declaration lookup, which is the point: these types used to reach structural
drilling, and an interface is the shape drilling handles worst.

**Two answers, chosen by position rather than by the declaration:**

- `In() == "formData"` → **`type: file`**. The only place OAS 2.0 permits `file`, and the canonical
  upload shape. It also repairs a spec that was outright invalid: a formData parameter typed
  `io.Reader` emitted **no `type` at all**, and SimpleSchema requires one.
- everywhere else → **`{type: string, format: byte}`**. In a JSON body a raw octet sequence has no
  representation; `byte` is the base64-encoded string OAS 2.0 defines for exactly that. `binary`
  would claim a framing the position cannot carry.

**`x-go-type` carries what neither answer can.** `byte` says base64 bytes and `file` says an upload;
both erase *which* stream this was, and all twelve recognized types collapse onto the same schema —
an `io.Reader` field and a `multipart.File` field become indistinguishable. So the recognizer stamps
`x-go-type: <pkg path>.<name>`, under `skipExt` like every other extension. This is the
`recognizeError` criterion ("the rendering erases the type"), not the `time.Time` one ("the format
*is* the type") — see [§traceability](#traceability).

**Identity, never structure.** A rule like "anything with a `Read([]byte) (int, error)` method"
would swallow any user interface that happens to expose one. The table is closed: a type joins it
because it is a known stream carrier, not because its method set resembles one.

**`io.Writer` is deliberately absent**, with its write-only closers. A sink the caller writes into
does not travel on the wire, so an API type containing one is unknown territory — recognizing it
would be inventing an intent. It keeps whatever the structural walk makes of it, and the author says
what they meant.

**This is not a guess about content.** Before the recognizers codescan already answered, and
answered worse: `io`'s own interfaces became definitions carrying io's godoc as title/description,
and `ReadCloser` grew a **`close` property of type `string`** out of `Close() error` (interface-method
promotion applied to a method that is not an accessor). `{string, byte}` is the *less* presumptuous
answer — the standard way to say "opaque bytes, framing unstated".

An explicit `swagger:strfmt` / `swagger:type` / `swagger:file` still wins; the classifier runs first.

Related: this closes part of [§quirks](#quirks)' degraded-graph concern for these types — a
recognizer answers from the object alone, so a truncated package graph no longer means a hard
`unable to find package and source file for: io.Reader`.

## <a id="math-big"></a>§math-big — the arbitrary-precision numbers, and why the three disagree

`recognizeMathBig` recognizes `math/big`'s three number types. They do **not** share an answer, and
the split is not a style choice — it is what `encoding/json` does:

| Type | Marshals via | On the wire | Emitted |
|------|--------------|-------------|---------|
| `big.Int` | `MarshalJSON` | `1234567890123456789000` | `{type: integer}` |
| `big.Float` | `MarshalText` | `"3.5"` | `{type: string}` |
| `big.Rat` | `MarshalText` | `"5/3"` | `{type: string}` |

All three carry a `MarshalText`; only `big.Int` also carries a `MarshalJSON`, and `encoding/json`
prefers `json.Marshaler` over `encoding.TextMarshaler`. So `big.Int` travels as a bare number while
its two siblings travel quoted. Unmarshalling enforces the same split in both directions — a number
offered to a `*big.Float` is rejected, and a string offered to a `*big.Int` is too — so reading
`Float` and `Rat` as `number` because of their names would publish a spec that round-trips through
nothing.

**No `format`, in any arm.** The precision is unbounded by construction: `int64` / `double` would be
a narrower claim than the type makes.

**It is in the canonical `ApplyStdlibSpecials` set *and* in `buildFromTextMarshal`'s list** — the only
recognizer in both. That is what makes the answer independent of pointer-ness. A `*big.Int` field
satisfies `TextMarshaler`, so `buildFromType`'s shortcut diverts it to `buildFromTextMarshal` before
any call site reaches `ApplyStdlibSpecials`; a `big.Int` field does not satisfy it, because the
marshal methods take a pointer receiver, so it arrives through the ordinary named-type path. Wiring
only one of the two would leave the same Go type rendering one way as a pointer and another by value.

That the two agree is a statement about `encoding/json`, not a convenience: it takes the address of an
addressable field to reach a pointer-receiver marshaller, so a `big.Int` field of a struct marshalled
through a pointer — what any server does — emits exactly what its `*big.Int` neighbour emits.

**`x-go-type` in every arm.** `integer` cannot say the value is unbounded rather than an `int64`, and
`string` cannot tell a decimal float from a quotient — `big.Float` and `big.Rat` land on the same
schema and differ in what the string *contains*. That is the `recognizeError` criterion ("the
rendering erases the type"), see [§traceability](#traceability).

**What this replaced.** Neither half was right. Through a pointer all three collapsed onto `{type:
string}` via the TextMarshaler fallback, which mis-stated `big.Int`. By value none of them satisfied
`TextMarshaler`, so they fell to structural drilling and published `Int`, `Float` and `Rat` as object
definitions carrying `math/big`'s own godoc as title/description — the same leak the stream types used
to produce, see [§opaque-streams](#opaque-streams).

**A defined type over one of them is recognized too**, through the written-RHS rule rather than
through identity: `type Tally big.Int` inherits none of big.Int's methods, but the scanner keeps the
type the declaration was *written over*, so it means `big.Int` here for the same reason `type MyTime
time.Time` has always meant a date-time rather than a drilled struct. An alias (`type Amount =
big.Int`) is the same type object and matches the recognizer directly.

An explicit `swagger:strfmt` / `swagger:type` still wins; the classifier runs first. A format override
adjusts the format and the `x-go-type` stamp survives; a type override replaces the schema outright
and the stamp goes with it.

Witnessed by `TestMathBig` over `testdata/enhancements/math-big/`.

## <a id="embedded"></a>§embedded — embed routing, struct/interface specials asymmetry

`buildEmbedded` is the entry point for a struct's embedded fields
that the embed classifier
([§allof](#allof) §scanEmbeddedFields → plain-embed arm) routed for
inline merge into the outer schema. It splits three ways: pointers
peel (recurse), `*types.Named` descends into `buildNamedEmbedded`,
`*types.Alias` goes through `buildAlias` (so alias-resolution
honours `TransparentAliases` / `RefAliases`).

### An embed that promotes nothing is an ordinary property

`buildEmbedded` is only reached for an embed that actually **promotes**. Go promotes struct fields
and interface methods; a named type over a basic, slice, array or map has no member to promote, so
Go keeps the value as an ordinary key named after the **type**:

```go
type Count int
type Host struct {
    Count                       // → {"Count": 0, "label": "…"}
    Label string `json:"label"`
}
```

`embedPromotes` (allof.go) makes that call, and `buildPlainEmbed` routes the false branch down the
same path as a **json-named** embed — because it is the same thing: a single named property built
from the embedded type, classifiers included. The name is the Go field name (which for an embed *is*
the type name), and the embed's own json tag renames or drops it exactly as on a regular field.

This shape used to reach `buildNamedEmbedded`, whose switch has struct and interface arms only, and
fall to a `default` that warned `unsupported Go type` and skipped — wording that describes a type
codescan cannot model rather than one it silently drops. The default arm survives as a defensive
guard; nothing on the struct path reaches it now.

### <a id="embed-marshaller"></a>Why a promoted marshaller is not modelled

A type reaching that false branch may implement `encoding.TextMarshaler`. Go promotes `MarshalText`
to the embedding struct, and under the **default** marshaller that makes the whole struct render as
a bare scalar — siblings and all:

```go
type Token [16]byte
func (t Token) MarshalText() ([]byte, error) { return []byte("tok"), nil }

type Embedder struct { Token; Name string `json:"name"` }
json.Marshal(Embedder{}) // → "tok"   — "name" never reaches the wire
```

codescan deliberately does not model this. In the convention it describes, an embed means
**composition**, and a composed model round-trips through a hand-written
`MarshalJSON`/`UnmarshalJSON` rather than the default one — go-swagger's generated models embed
their `allOf` members *and* generate the marshaller that flattens them. A promoted marshaller in
hand-written source is therefore not evidence about the wire.

Detecting it would also require answering a question a declaration cannot answer: a **pointer**
-receiver marshaller squashes for `&v` and not for `v`, and codescan reads the type, not the use
site.

The author who does want the scalar says so on the embedded type's own declaration, with
`swagger:strfmt` or `swagger:type` — the escape hatch that already works.

### `buildNamedEmbedded` — the two-arm specials asymmetry

The interface arm runs `applyStdlibSpecials(o, target, skipExt)`
**before** model lookup. The struct arm does *not*.

The asymmetry is deliberate:

- **Interface embed** is the common shape that surfaces `error`
  promoted into a struct (`type Err struct { error }`). The stdlib
  specials catch it and emit `{string}` with the `x-go-type: error`
  hint, matching the field-level recognizer behaviour.
- **Struct embed of `time.Time`** is uncommon — the fixture corpus
  has none. Adding `applyStdlibSpecials` to the struct arm would
  change behaviour only for code paths we don't currently exercise,
  and the change would surface as a golden delta on the day someone
  adds such a fixture. Until then the conservative choice is to
  preserve v1 parity: stdlib structs embedded as `*types.Struct`
  reach `GetModel` and build through the normal `buildFromStruct`
  path. If `time.Time` ends up embedded and the package isn't in
  `AllPackages`, `missingSource` fires — same as v1.

### Cross-source-file field promotion (go-swagger#2417)

When `buildNamedEmbedded` reaches the struct arm, `tpe.Underlying()`
collapses any chain of defined types straight to the `*types.Struct`,
so a cross-package defined type (`type AnotherPackageAlias
color.Color`) is built from the embedding type's `decl` even though the
promoted fields' source lives in the *underlying* type's file. The same
shape arises within a single package across files (a transparent alias
to a struct defined in a sibling file).

`structFieldCarrier` resolves each field's AST with
`FindASTField(decl.File, fld.Pos())`. That returns nil when the field
isn't in `decl.File` (a different file or package), which previously
dropped the field silently — the model came out a bare empty object.
The carrier now falls back to `ScanCtx.FileForPos(fld.Pkg().Path(),
fld.Pos())`, which locates the field's own source file via the shared
FileSet, so its json tag and doc are read correctly. The fallback only
fires when the primary lookup misses, so the common single-file path is
unchanged.

### Inherited `required:` from an embed (go-swagger#2701)

A `required:` annotation on a plain embed applies to the properties it
promotes. `scanEmbeddedFields` reads it via the shared
`common.EmbedInheritance` kernel (`ReadEmbedInheritance`) and threads it
through the embed recursion with save/restore; `applyFieldCarrier` then
adds each promoted property to the **enclosing** object's required list
(via `handlers.SetRequired`) unless the property set its own `required:`.
This is the schema half of the cross-builder rule shared with parameters
and responses — the schema builder consumes only `Required` (it has no
`in:` location). Response bodies inherit through this same path, since a
body is built by the schema builder. See
[common §embed-inheritance](../common/README.md#embed-inheritance).

### `AddDiscoveredModel` pairing

Both arms call `s.Ctx.AddDiscoveredModel(decl)` before recursing.
Reason: embedded user types appear as their **own** top-level
definition even when not annotated `swagger:model`. The
`discovered` queue picks them up on the next build pass. Parity
with v1.

### `processEmbeddedType` — interface-side allOf composition

Called from `buildNamedInterface` when walking the embedded
elements of an interface's underlying. Three shapes:

- **`*types.Named`** — runs `applyStdlibSpecials` (dummy target
  swallows the write so the recognizer can short-circuit without
  contaminating `schema`), then routes through
  `buildNamedInterface`.
- **`*types.Interface`** — anonymous embedded interface. Builds
  into a side schema; only when the result is non-empty
  (`Ref || Properties || AllOf`) is it appended to the outer
  `schema.AllOf`.
- **`*types.Alias`** — same non-empty guard, builds via
  `buildAlias` so alias modes are honoured.

The non-empty guard hides `interface{}` and other zero-content
interfaces at the allOf seam — they don't contribute an `{}` entry
to the outer schema.

---

## <a id="json-dash"></a>§json-dash — `json:"-"` is not a way to hide a promoted field

The emitted property set must match what `encoding/json` puts on the wire. Two
readings of the `-` tag used to diverge from it.

**A `-` field is ignored, not shadowing.** encoding/json drops such a field
entirely — it never enters the name set — so re-declaring a promoted field with
`json:"-"` does **not** hide the embedded one, which Go keeps marshalling:

```go
type Outer struct { Base; Age int32 `json:"-"` }   // Base has Age int32 `json:"age"`

json.Marshal → {"id":1,"name":"n","age":42}        // age survives, from Base
```

The builder used to delete the promoted property here, which understated the
wire. It no longer does. The intent is real, though — an author writing this
wants the field gone — so the shape raises `scan.shadowed-embed-field`, pointing
at [`swagger:omit`](#omit), which drops it for real. Contrast with a re-declaration
under a *real* name, where Go's depth rule does apply and the outer field wins.

Tagging the **embed itself** `json:"-"` is a different act and does drop the whole
embed; that always worked.

**The whole tag must be `-` to ignore.** encoding/json compares the entire tag,
so `json:"-,"` and `json:"-,omitempty"` name the field literally `-`. Splitting on
the comma first conflates the two and dropped a field Go marshals; `jsonTagIgnores`
in `resolvers` now does the whole-tag comparison, and `-` is accepted as a name
from the `json` tag only (no other tag type has an encoding rule to appeal to).

**Witness.** `testdata/enhancements/json-tag-fidelity` is differential: the fixture
module marshals its own types and commits the key sets as `wire.golden.json`, and
`TestJSONTagFidelity` asserts the emitted property sets equal them. Neither side
hard-codes an expectation — the oracle is encoding/json itself.

## <a id="omit"></a>§omit — `swagger:omit`, the author's pre-filter on promoted fields

Promoting an embed can produce a schema the author never meant, and codescan must not guess which
one they meant. The two cases that motivated the annotation (go-swagger#1992 plus the override
defects found while auditing `DefaultAllOfForEmbeds`):

| shape | Go marshals | inlined | composed (`allOf`) |
|---|---|---|---|
| outer `ID` re-declared to add `readOnly` | one `ID` | one, decorated | `ID` in BOTH members |
| outer `ID string` over `ID int64` | one, string | string | integer AND string — unsatisfiable |
| a shared type embedded in a request body | every field | every field | every field |

Inlining **resolves** an override (it applies Go's depth rule and emits the winner); `allOf`
**accumulates** — members conjoin, and conjunction can only narrow, never replace. So an override
that replaces is not expressible as composition, and no amount of tidying the members fixes it.
`swagger:omit` is the escape hatch: the author states which promoted fields do not belong, and the
scanner keeps mirroring the code otherwise.

### It is a pre-filter, not a post-hoc delete

`swagger:omit` means *"do not promote this field when walking the embed"*. One rule then covers both
renderings — the field is simply never written:

- inlined: an outer re-declaration wins as it already did (so omitting
  its promoted twin is correctly a no-op there);
- composed: the base member is built without the field, which removes
  the duplicate and the unsatisfiable pair above.

No mode-awareness, and no Go-name↔JSON-name reconciliation, because filtering happens *before* names
are computed — which is why targets are Go field names and why the annotation is indifferent to json
tags and to `NameFromTags`.

The filter itself is one condition in `processStructField` (`struct.go`); everything else lives in
`omit.go`.

### Resolution runs against the type, not the walk

Each target is resolved once, at the annotation site, with `types.LookupFieldOrMethod` — Go's own
promoted-field lookup, so depth and ambiguity rules come for free. The scope then carries the
resulting `*types.Var` objects and the filter is pointer identity.

Consequences worth keeping: there is no "what was actually omitted" bookkeeping (a target either
names a field of the type or it does not, known before the walk starts); a target already excluded
for another reason resolves fine and is silently redundant; and two same-named fields at different
depths can never be confused, because distinct fields are distinct objects.

### Placement

- **on the embed** (ergonomic): targets are plain field names of that
  embedded type;
- **on the declaration** (power form): a dotted path names the embed
  chain (`Base.ID`, head consumed per level), a bare name is offered to
  each embed and resolved against its promoted set.

Every path segment but the last must name an **embedded** field: `omit` removes promoted content,
the only thing the enclosing schema owns. A segment naming a regular field stops the walk and is
reported as unresolved — reaching through one would edit either another type's inlined copy or a
`$ref`'d definition.

### Diagnostics (all Hints)

| code | fires when |
|---|---|
| `scan.omit-unresolved` | the target names no field of the embedded type — a typo, or a rename upstream |
| `scan.omit-behind-ref` | the embed is composed as a `$ref` (an annotated model), where OAS2 cannot subtract a property; the omission is dropped rather than silently forking the definition |
| `scan.shadowed-embed-field` | a field re-declared with `json:"-"` carries the Go name of a promoted field (see below) |

`swagger:omit` is the only construct whose output depends on a hand-written name the compiler never
checks — everything else is derived from types. `scan.omit-unresolved` reports it when a field is
renamed upstream, instead of letting the annotation rot silently.

### The `json:"-"` shadow is not what authors think

`fields.go` deletes a promoted property when an outer field re-declares it with `json:"-"`. That is
**unfaithful to `encoding/json`**, which ignores a `-` field entirely: it never enters the name set,
so it cannot shadow the promoted one and Go keeps marshalling the embedded field. The behaviour is
locked by `TestOverridingOneIgnore` and left in place for now; the Hint points at `swagger:omit`,
which removes the field for real. Removing the eviction is a separate decision — see
`.claude/plans/swagger-omit.md` §7.

Fixtures: `testdata/enhancements/swagger-omit` (the annotation, both renderings, all three Hints,
plus the go-swagger#1992 shape verbatim) and `testdata/enhancements/default-allof-embeds-override`
(the same overrides *without* the annotation — the documented limit).

## <a id="embed-depth"></a>§embed-depth — ambiguous-embed diagnostic mechanism

`Builder.embedDepth` (incremented around `buildNamedEmbedded`'s
recursive descents via `defer s.enterEmbed()()`) tracks the depth
of embedded-type recursion in a single `buildFromStruct` /
`buildFromInterface` pass.

`applyFieldCarrier` uses it to distinguish:

- **`embedDepth == 0`** — legitimate explicit override (`S.Foo`
  redefining the JSON name of an embedded `E.Foo`). No diagnostic.
- **`embedDepth > 0` + prior at deeper depth** — Go's depth rule
  (shallower wins) already disambiguates. No diagnostic.
- **`embedDepth > 0` + prior at same-or-shallower depth + different
  Go name** — peer ambiguity Go itself would refuse to promote.
  Emits `CodeAmbiguousEmbed` (`SeverityWarning`). Last-write-wins
  behaviour is preserved; only the signal is added.

Fixture: `testdata/enhancements/diagnostics/types.go` covers all
three cases.

---

## <a id="method-mangler"></a>§method-mangler — interface-method JSON-name derivation

Interface methods can't carry struct tags. To pick a JSON property
name for a method, `Builder.methodMangler` (a
`mangling.NameMangler` from `go-openapi/swag`) applies the
acronym-aware lower-first transform: `CreatedAt → createdAt`,
`ID → id`, `ExternalID → externalId`.

### Principled asymmetry vs the struct path

The struct-field path does NOT mangle: `resolvers.ParseJSONTag`
returns the json-tag value when present, otherwise the Go field name
verbatim. So `CreatedAt string` with no tag emits property
`CreatedAt`, while `CreatedAt() string` on an interface emits
property `createdAt`.

The "Go field name" is the per-field name reported by `go/types`, not
the first identifier of the AST field group. A field group declaring
several names on one line (`R, G, B, A uint8`) expands to one
`*types.Var` per name, each promoted to its own property; the shared
AST `*ast.Field` is the same node for all of them, so the name must
come from the var, not `field.Names[0]` (go-swagger#2638). A json
rename names a single field, so it is dropped for a multi-name group
(each member keeps its Go name) while `-`, `,omitempty` and `,string`
still apply to every member.

This asymmetry is intentional, not a quirk:

- **Struct fields mirror real serialization.** `encoding/json`
  uses the tag-or-verbatim rule at runtime; codescan's emitted
  spec is what `json.Marshal` would actually produce. Adding an
  auto-mangle on the struct side would silently disagree with the
  user's running program.
- **Interface methods don't have a "natural" serialization.**
  Go's `encoding/json` can't marshal embedded interface methods at
  all without a custom `MarshalJSON`. There is no runtime ground
  truth to mirror, so codescan invents a sensible default JSON
  name. The mangler is the documentation convention, not a
  serialization mirror.

The "one size fits all" mangler on interfaces will not always be
what the author wanted, so it can be turned off globally with
**`Options.SkipJSONifyInterfaceMethods`** (opt-out, default `false`).
When set, the carrier emits the Go method name verbatim instead of
calling `s.interfaceJSONName(fld.Name())` — useful for an interface
already named for its JSON shape, or a codebase with its own
canonical-name discipline. A `swagger:name X` override still wins
verbatim regardless (see below); the flag only changes the
no-override fallback. The on/off contract is pinned by
`testdata/enhancements/interface-no-mangle/` +
`integration/coverage_skip_jsonify_interface_test.go`.

### `swagger:name X` is verbatim

When the author provides `swagger:name X` on an interface method,
**X is emitted exactly as written** — the mangler is bypassed
entirely. The carrier code
(`fields.go:methodCarrier`) checks `fd.JSONName` first; only when
empty does it call `s.interfaceJSONName(fld.Name())`.

This contract matters for non-camelCase user input — a user who
writes `swagger:name UserIdentifier` wants `UserIdentifier`, not
`userIdentifier`. The regression-detector for this is
`testdata/enhancements/interface-name-verbatim/` +
`integration/coverage_interface_name_verbatim_test.go`: PascalCase,
snake_case, SCREAMING_CASE, and hyphenated user inputs all assert on
the exact spelling reaching the spec.

### Constructor invariant

The mangler is initialised in `NewBuilder`; `&Builder{…}` literals
that bypass the constructor will nil-panic in `interfaceJSONName`.

---

## <a id="user-overrides"></a>§user-overrides — explicit user-driven overrides

Three classifier annotations let the author bend the type-driven
default. They live at two scopes — the **decl-site** (on the type
declaration's doc comment) and the **field-site** (on a struct
field or interface method's doc comment) — and the schema builder
applies each at a distinct point in the build pipeline.

### Where each override is consumed

| Annotation | Scope | Consumed by | Applied at |
|---|---|---|---|
| `swagger:type X` | decl-site | `classifierNamedTypeOverride(s.Decl.Comments, ps)` | `buildFromDecl` — **before** kind-dispatch |
| `swagger:type X` | decl-site, reached via field reference | same classifier, walked from the field's *types.Named decl | `buildNamedType` / `buildNamedArrayLike` / `classifierNamedBasic` arms |
| `swagger:type X` | field-site | `fieldDoc.TypeOverride` populated by `scanFieldDoc` | `applyFieldCarrier` — **after** `buildFromType` |
| `swagger:strfmt X` | decl-site | per-arm classifiers (`classifierTextMarshal`, `classifierNamedStructStrfmt`, …) | dispatch-table arms |
| `swagger:strfmt X` | field-site | `fieldDoc.StrfmtName` | `applyFieldCarrier` |
| json tag `,string` | field-site | `fieldCarrier.isString` | `applyFieldCarrier` |

### Ordering inside `applyFieldCarrier` (last-write-wins)

```
buildFromType(propType, ps)
    → isString   sets {type: string, format: <kept>}
    → StrfmtName sets {type: string, format: <X>}
    → TypeOverride  resets ps; runs SwaggerSchemaForType(X) or falls back to Underlying()
    → applyBlockToField  consumes everything else (description, validations, extensions)
```

A field that picks up multiple overrides resolves by the last write.
Mixing `,string` and `swagger:strfmt` and `swagger:type` is misuse
in source — the precedence simply prevents accidental contradictions
from corrupting the schema mid-build.

### Decl-site `swagger:type` fallthrough — known leaf vs unknown leaf

`classifierNamedTypeOverride` tries `SwaggerSchemaForType(name, tgt)`:

- **known leaf** (`object`, `string`, `integer`, `boolean`, `number`):
  classifier returns `(handled=true, recurse=false)`. The override
  terminates the dispatch — the schema is left typed as the user
  asked.
- **unknown leaf** (`array`, `badvalue`, …): `SwaggerSchemaForType`
  errors, classifier returns `(handled=true, recurse=true)`. The
  caller falls back to `s.Decl.ObjType().Underlying()` so item
  shapes are filled from the Go-level shape. For
  `type X json.RawMessage // swagger:type array`, the Underlying is
  `[]byte` and the result is `{type: array, items: {integer, uint8}}`.

The same `(handled, recurse)` contract applies at field-reference
sites via `classifierNamedArrayLike` (the wrapper-decl path that
inlines into the field's schema) and at the decl-site via
`buildFromDecl` (the wrapper's own top-level definition).

### Two scopes, two effects, independent precedence

The same `swagger:type` annotation at the **decl-site** decides
what the type's `definitions` entry emits; at the **field-site** it
decides what one specific field emits, regardless of its Go type's
natural shape.

A field referencing a wrapper-decl honours **both** overrides — the
wrapper's own definition reflects its decl-site override, and the
referring field reflects its own field-site override (which wins
locally by ordering). Both layers compose without mutating each
other.

### `scanFieldDoc` and the `AnnType` filter

`scanFieldDoc` is the field-level walker that pre-extracts every
classifier signal in one pass over `ParseBlocks(afld.Doc)`. Most
annotations (`ignore`, `name`, `strfmt`, `allOf`) go through the
lexer's `firstIdent` arg-classifier, which already produces a
single-token arg — no filter needed.

`AnnType` is the exception: its arg-classifier `TrimSpaces` the
whole rest of the line, so prose noise like
`swagger:type so the scanner emits …` reaches `b.AnnotationArg()`
as a multi-word string. `scanFieldDoc` filters those out with an
inline `strings.ContainsAny(name, " \t")` check, mirroring the
filter inside `findAnnotationArg` (`walker_classifiers.go`). The
`enhancements/named-basic` fixture documents the v1 trap this
filter protects against.

### Recognizer `skipExt` plumbing

`applySpecialType` and `applyStdlibSpecials` take a `skipExt bool`
parameter that gates any vendor-extension writes the recognizers
would otherwise emit. Two write one: `recognizeError`
(`x-go-type: error`) and `recognizeOpaqueStream`
(`x-go-type: <pkg path>.<name>`). The others
(`recognizeTime`, `recognizeAny`, `recognizeRawMessage`,
`recognizeUUID`) are purely type / format mutations and don't
consult `skipExt` — see [§traceability](#traceability) for why the
split falls where it does. All eight schema-internal call sites pass
`s.skipExtensions` so the recognizer subsystem honours the same
`SkipExtensions` flag as the rest of the builder.

### `swagger:title` / `swagger:description` overrides

Two override annotations let the author replace the **godoc-derived**
title / description with curated API-facing text (Q30 close-out: a
Go doc comment written for Go readers — e.g. `time.Time`'s monotonic-
clock prose — should not have to leak into the spec).

- **`swagger:title <text>`** — single line; sets the schema/property
  `title`. Schema-only.
- **`swagger:description <text>`** — replaces the `description`. May span
  multiple lines: the prose lines following the annotation fold into the
  description (joined with `\n`), terminated by the first blank line,
  keyword, or annotation (Option B). On responses/headers only the
  description override applies; `swagger:title` there raises
  `parse.context-invalid` (OpenAPI 2.0 has no Response/Header title).

Semantics:

- **Precedence.** Present ⇒ replaces the godoc value; absent ⇒ godoc is
  used unchanged (no behaviour change for un-annotated decls).
- **Empty ⇒ suppress + warn.** A bare `swagger:description` (or a
  whitespace/blank-only body) applies the empty value — the deliberate
  godoc-suppression affordance — and raises `scan.empty-override`.
- **Harvest.** `common.Builder.HarvestOverrides` collects the two
  annotations from a comment group's sibling blocks; `WarnEmptyOverride`
  raises the empty warning at the consumption point (sibling classifier
  blocks are not `Walk`-ed, so a grammar-stored diagnostic would not reach
  `OnDiagnostic`). The schema builder wraps both in `overridesFor`.
- **Family.** Unlike the classifier annotations above, `swagger:title` /
  `swagger:description` dispatch through the **schema** family (like
  `swagger:name`), so a co-located validation keyword (`maximum:`,
  `pattern:`, …) on the same field surfaces as a Property and is applied
  rather than rejected as context-invalid.
- **`$ref` fields.** title and description are symmetric `$ref` siblings:
  they ride description's existing preservation rule — kept under
  `EmitRefSiblings` / a forced `allOf` compound, dropped to a bare `$ref`
  under the default flags (see [§ref-override](#ref-override)).

See `.claude/plans/features/swagger-description-override-design.md`.

---

## <a id="traceability"></a>§traceability — `x-go-*` origin extensions and `EmitXGoType`

`annotateSchema` (`schema.go`) decorates each emitted **definition** with
scanner-derived origin metadata, deferred so it runs after the type is built:

- `x-go-name` — the Go identifier, emitted only when it differs from the
  spec definition name (`s.Name != s.GoName`).
- `x-go-package` — the originating package import path.
- `x-go-type` — the fully-qualified Go type (`<package path>.<type name>`),
  **opt-in** behind `Options.EmitXGoType` (go-swagger#2924). Useful for
  round-tripping a generated spec back to its source types.

All three pass through `resolvers.AddExtension(..., s.skipExtensions)`, so
`SkipExtensions` suppresses the whole family.

`x-go-type` predates the option as a narrow type-rendering signal: the
generic `PkgForType` fallback (`special_types.go`), `recognizeError` and
`recognizeOpaqueStream` stamp it deliberately to record an
otherwise-unmodellable type. The criterion is whether the emitted schema
still identifies the Go type: `time.Time` → `{string, date-time}` and
`uuid.UUID` → `{string, uuid}` do, so those recognizers stay silent;
`error` → `{string, ""}` and every stream → `{string, byte}` / `file` do
not — the latter collapses twelve distinct types onto one schema. The
`annotateSchema` stamp is **presence-guarded** (`if _, exists :=
schema.Extensions["x-go-type"]; !exists`) so it never clobbers a value a
recognizer already chose — for ordinary types the recognizer leaves it
unset and the option supplies it.

---

## <a id="additional-properties"></a>§additional-properties — the `swagger:additionalProperties` marker

`swagger:additionalProperties <spec>` is a decl-level classifier
(`grammar.AnnAdditionalProperties`) consumed by
`classifierAdditionalProperties` (`additional_properties.go`), applied from
`Build` **after** `buildFromDecl` has resolved the Go type so it can ride on
top of the type-derived schema.

`<spec>` is one of:

| Arg | Effect | Render |
|---|---|---|
| `true` | allow extra keys | `additionalProperties: true` (`SchemaOrBool{Allows:true}`) |
| `false` | forbid extra keys | `additionalProperties: false` (`SchemaOrBool{Allows:false}`) |
| `<TypeSpec>` | typed value schema | `additionalProperties: {<schema>}` |

`<TypeSpec>` reuses the `swagger:type` argument grammar (primitive / Go-builtin
spelling / leading `[]` array layers) via `resolveAdditionalPropertiesType`,
with one deliberate difference from `resolveTypeOverride`: a **type-name
reference resolves to a `$ref`** (and registers the model for discovery), not an
inline expansion — an `additionalProperties` value naturally references a model,
matching how a `map[string]Model` field renders.

Semantics depend on what the Go type produced:

- **struct** → COMPLEMENT: the named properties stay; the marker adds
  `additionalProperties`. This is the #2539 / §17 case (`false` to close an
  object) and the #3005 case (a typed value alongside named properties).
- **map** → OVERRIDE: `buildFromMap` already emitted `additionalProperties: V`;
  the marker replaces it.
- **bare `$ref`** (a map/wrapper type that resolved to a reference) → DEFINE: the
  `$ref` is cleared and a clean `{type: object, additionalProperties: …}` is
  emitted (the marker beats the Go type; a `$ref` cannot carry siblings).

**Precedence — lowest priority.** `additionalProperties` only rides on an
`object`. If a prior rule already fixed a non-object type (`swagger:type` on a
non-object, `swagger:strfmt`, a special/known type), the marker is dropped with a
`CodeShapeMismatch` diagnostic. It composes freely with the other object
validations (`maxProperties`, `minProperties`, `patternProperties`).

### Field keyword — `additionalProperties: <spec>`

The same `<spec>` is also accepted as a **field keyword**
(`grammar.KwAdditionalProperties`, `CtxSchema`) decorating a struct field, with
the same value grammar and lowest-priority precedence. Two landing paths:

- **non-`$ref` field** (a map, an inline object, a primitive) —
  `applyBlockToField` post-scans the block (`block.GetString`) after the normal
  keyword dispatch and calls `applyAdditionalPropertiesSpec` on the field schema:
  it overrides a map's element schema, or warn-drops on a primitive.
- **`$ref`'d field** — handled in `applyToRefField`'s `refOverrideCollector`: the
  value rides as an **allOf sibling** (`{allOf: [{$ref}, {additionalProperties:
  …}]}`) so the reference is preserved (JSON-Schema-draft-4), rather than the
  marker's `$ref`-reset.

Both paths share `resolveAdditionalPropertiesValue` (the pure
`true | false | <TypeSpec>` → `SchemaOrBool` resolver, no parent mutation).

---

## <a id="pattern-properties"></a>§pattern-properties — the typed `swagger:patternProperties` marker

`swagger:patternProperties "<re>": <spec>, …` is a decl-level classifier
(`grammar.AnnPatternProperties`) consumed by `classifierPatternProperties`
(`pattern_properties.go`), applied from `Build` alongside the
additionalProperties marker. It is the **typed** counterpart of the regex-only
`patternProperties:` field keyword (which sets an empty `{}` value schema): each
pair maps a property-name regex to a value schema resolved through the same
`<TypeSpec>` grammar (`resolveAdditionalPropertiesType`, so a type-name → `$ref`).

The whole `"<re>": <spec>, …` remainder is captured by the lexer as one raw arg
token (regexes may contain spaces / colons / commas), read back via the
non-filtering `findRawAnnotationArg`, and split by `parsePatternPropertyPairs` —
a small hand-parser that respects the double-quoted regex (only `\"` is an escape
inside it; every other backslash, e.g. `\d`, is preserved verbatim) and reads
each spec up to the next top-level comma.

Same precedence as additionalProperties: object-only (a non-object resolution
warn-drops the marker; a bare `$ref` is reset). Each regex is RE2-hygiene-checked
— an invalid regex is preserved on the schema but raises a `CodeInvalidAnnotation`
warning, mirroring the `patternProperties:` keyword wording. A structurally
malformed pair list is dropped with a diagnostic rather than partially applied.

OAS-2 caveat: `patternProperties` is a JSON-Schema-draft-4 keyword, not part of
the Swagger-2.0 Schema Object subset — emitted ungated by design (go-openapi
favours JSON Schema; see the additional-properties plan).

---

## <a id="ref-override"></a>§ref-override — field-level overrides on a `$ref`'d field

`applyToRefField` handles the case where a struct field's Go type
resolves to a named type whose schema lives in `definitions`
(`ps.Ref` set). Field-level sibling content — description,
`pattern`, `enum`, `example`, `x-*` extensions — cannot ride
alongside `$ref` per JSON-Schema-draft-4: the ref predates and
replaces siblings. The correct shape is an **allOf compound**:

```json
{
  "description": "...",
  "allOf": [
    { "$ref": "#/definitions/Parent" },
    { "...override validations and extensions..." }
  ]
}
```

### Per-keyword landing rules

- **`required:`** writes to `enclosing.Required` (it's a
  parent-side concern, not a sibling of the `$ref`).
- **Description** rides the outer allOf compound when any
  field-level content is present, including just the description
  itself. The `DescWithRef` option (below) covers the only-description
  edge case.
- **Validations** (`maximum`, `pattern`, `enum`, …) land on `allOf[1]`
  — the override schema arm.
- **Vendor extensions** (`x-*` via the `extensions:` raw block)
  are **lifted onto the outer compound**, not nested inside
  `allOf[1]`. Reason: `x-*` siblings of `$ref` should live at the
  same level as scanner-derived metadata (`x-go-name`,
  `x-go-package`) for consistency. See `refOverrideCollector`'s
  flag explanation below.
- **externalDocs** (the `externalDocs:` raw block) is likewise an
  annotation sibling of the `$ref` and is **lifted onto the outer
  compound** alongside the description and `x-*` keys, not nested
  inside `allOf[1]` (go-swagger#2655). A non-ref field emits its
  externalDocs via `handlers.schemaRawHandler` instead.

### Sibling-rendering toggles — two orthogonal axes

`$ref` siblings split into two classes by how they can be emitted:

- **description & extensions** — *siblings-eligible*: they can ride
  directly beside the `$ref` (`{$ref, description, x-*}`), which strict
  draft-4 ignores but OpenAPI 3.1 / JSON Schema 2020-12 / modern
  Swagger-UI honour; or via the allOf wrap.
- **validations & externalDocs** — *compound-only*: they have no valid
  bare-`$ref` form, so they can only ride an allOf compound (validations
  on the override arm).

Three options steer the rendering. The **defaults reproduce the legacy
behaviour byte-for-byte** — both new opt-ins off:

- **`EmitRefSiblings`** (default false): when true, description and
  extensions ride as **direct `$ref` siblings** (no allOf), *unless* a
  validation/externalDocs already forces a compound — in which case they
  ride the outer compound as before. Changes only the no-forced-compound
  cases.
- **`DescWithRef`** (default false; **deprecated**, kept for
  compatibility): governs only the *description-only* case in the legacy
  wrap path — `true` preserves the description as a single-arm allOf
  (`{description, allOf:[{$ref}]}`), `false` drops it. A no-op when
  `EmitRefSiblings` is set. Prefer `EmitRefSiblings`.
  The `false` drop raises one `CodeDroppedRefSibling` **Hint** naming the
  field and pointing at `EmitRefSiblings` — the output is deliberate, but
  an author whose description vanished otherwise had nothing to go on. A
  Hint rather than a Warning: nothing is wrong, the default simply cannot
  carry prose beside a bare `$ref`.
- **`SkipAllOfCompounding`** (default false): when true, **no allOf
  compound is ever produced**. Validations and externalDocs are
  therefore **dropped**; description and extensions are dropped too
  *unless* `EmitRefSiblings` keeps them as direct siblings. Each drop
  raises one `CodeDroppedRefSibling` diagnostic through
  `Options.OnDiagnostic`, so the loss is never silent. For downstream
  consumers (e.g. go-swagger codegen) that expect a bare `$ref`.

Invariants:

- When validations or externalDocs are present, the allOf wrap is
  mandatory (unless `SkipAllOfCompounding` drops them) — no toggle
  promotes a validation to a bare sibling.
- **`required:` is always preserved.** It is a parent-side concern (it
  lands on the enclosing object's `required` list, not as a `$ref`
  sibling), applied during the collector Walk regardless of any flag.

### `refOverrideCollector` — accumulate-then-decide

The collector accumulates field-level overrides into a scratch
schema so `applyToRefField` can pick the final shape after the
Walker has finished firing. Three flags track what was collected:

- **`collectedValidation`** — a JSON-Schema validation keyword
  fired (`maximum`, `pattern`, `enum`, …). When true, the override
  arm (`allOf[1]`) is emitted carrying the validation.
- **`collectedExtension`** — a vendor extension fired. When true,
  the collected extensions are **lifted onto the outer compound**
  (not the override arm) so `x-*` keys live alongside the
  scanner-derived `x-go-name` / `x-go-package`.
- **`collectedExternalDoc`** — an `externalDocs:` block fired. When
  true, the parsed `*ExternalDocumentation` is set on the outer
  compound (sibling of the `$ref`), mirroring the extension lift.

The collector also records each collected sibling in `collected`
(keyword, source position, and `siblingKind` — validation / extension
/ externalDoc). `applyRefSiblingDrop` consumes this under
`SkipAllOfCompounding`: extension-kind siblings survive when
`EmitRefSiblings` is set, everything else is dropped with one
`CodeDroppedRefSibling` diagnostic per keyword.

Splitting the collector out of `applyToRefField` keeps the
per-shape Walker callbacks short and the orchestrator's cognitive
complexity in check. The Walker fires; the collector records; the
outer function shapes.

### `applyPattern` — best-effort RE2 hint

`applyPattern` stores a regex pattern unconditionally — JSON Schema
regex's grammar is broader than Go's RE2 (lookaheads, named
groups, etc.) and a user may rely on a downstream validator that
accepts the wider syntax. A best-effort `regexp.Compile` check
runs alongside: if the pattern is invalid against RE2, a
`SeverityWarning` diagnostic surfaces the issue without dropping
the value.

The diagnostic rides on `CodeInvalidAnnotation` rather than a
dedicated `CodeInvalidPattern`. Reason: it's the closest existing
class for "the value is grammatically valid but semantically off."
A dedicated code can land alongside a broader pattern-hygiene pass
when one materialises.

---

## <a id="simple-schema-mode"></a>§simple-schema-mode — SimpleSchema build mode for non-body params and response headers

OAS v2 distinguishes a **full Schema** (body parameters, response
bodies, top-level definitions) from a **SimpleSchema** that applies
to non-body parameter sites and response headers:

- A SimpleSchema is a parameter with `in: {query, path, header,
  formData}` (anything except `in: body`) or a response header.
- SimpleSchema vocabulary is a restricted subset of Schema — `$ref`,
  `allOf` / `oneOf` / `anyOf` / `not`, `properties` /
  `additionalProperties`, object-level `required`, `discriminator`,
  `readOnly`, `xml`, `externalDocs` are all forbidden.
- The notion disappears in OAS v3.

References: <https://swagger.io/specification/v2/#parameter-object>
and <https://swagger.io/specification/v2/#header-object>.

The schema builder offers `WithSimpleSchema(tpe, tgt, in)` as the
third Build mode (parallel to `WithType` / `WithDefinitions`). The
caller signals SimpleSchema-context explicitly via the option; the
builder no longer infers it from `tgt.In()`. See
[§build-entry](#build-entry) for the option table.

### Allowed keyword surface

Common between parameters and headers:

| Keyword | Notes |
|---|---|
| `type` | `{string, number, integer, boolean, array}`; `file` for params only |
| `format` | same vocabulary as full Schema |
| `items` | required when `type: array`; nested SimpleSchema, recursive |
| `collectionFormat` | `{csv, ssv, tsv, pipes, multi}`, default `csv` — SimpleSchema-only |
| `default` | |
| `maximum`, `exclusiveMaximum`, `minimum`, `exclusiveMinimum`, `multipleOf` | |
| `maxLength`, `minLength`, `pattern` | |
| `maxItems`, `minItems`, `uniqueItems` | |
| `enum` | |
| `x-*` vendor extensions | |

Parameter-only extras: `allowEmptyValue` (only when
`in ∈ {query, formData}` — forbidden on `path` and `header`);
`type: file` (only when `in: formData`).

Response headers exclude `file` and `allowEmptyValue` entirely.

### "Catch at exit" contract

The schema builder does **not** pre-filter inputs in SimpleSchema
mode. `*types.Struct` and `*types.Interface` are allowed to enter
the resolution pipeline because they can legitimately resolve to a
SimpleSchema-legal primitive:

- `time.Time` → `{string, date-time}` (stdlib recognizer)
- `TextMarshaler` → `{string, format}` (textmarshal shortcut)
- user-driven overrides (`swagger:strfmt`, `swagger:type`, the
  `swagger:alias` per-type opt-in via
  [§classifier-walkers](#classifier-walkers)'
  `classifierNamedBasic`) win the cascade as they always do

The exit validator (`validateSimpleSchemaOutcome` in
`simpleschema.go`) inspects the resolved target after
`buildFromType` returns:

- accept when `shape.Type` is in
  `{string, number, integer, boolean, array}`, plus `file` if
  `in == "formData"`.
- when `shape.Type` is **empty**, emit
  `CodeUnderspecifiedInSimpleSchema` and default to `{type: string}`.
- otherwise emit `CodeUnsupportedInSimpleSchema` (severity
  `SeverityWarning`), **reset** the target, and default it the same
  way.

**Empty is not an accepted outcome.** OAS v2 makes `type` required on
a non-body parameter and on a response header, so resolving to
nothing is invalid there rather than permissive — `{}` is the one
answer the position cannot carry. `any` and `json.RawMessage` are the
usual sources: "any JSON" answers a body or a definition and answers
nothing here. This reverses the earlier "empty is honest about the
failed resolution" rule, which produced honest *invalid* output; both
paths now end at `{type: string}` and both say so with a diagnostic.

`string` rather than `byte` or `binary`: a query string, a path
segment and a header are percent-encoded text, so `binary` would
claim octets the position cannot hold and `byte` a base64 framing
nobody applied — the same reasoning that keeps `binary` out of
[§opaque-streams](#opaque-streams). No format is invented, but an
already-resolved one survives, so a `swagger:strfmt` that landed a
format without a type keeps it. `x-go-type` records what the fallback
erased, under the `SkipExtensions` umbrella like any other extension.

**The validator is not the only place this must happen.**
`ApplyStdlibSpecials` also answers inside the parameters and
responses builders' own field arms (`buildNamedField`,
`buildFieldAlias`), which return *without* entering `Build` — so the
catch-at-exit contract never sees those targets, and `any` /
`json.RawMessage` reach a parameter exactly that way. Those arms call
the exported `EnsureSimpleSchemaTyped`, which holds the rule, and
report it themselves: each owns the position it is building and can
name the field. `EnsureSimpleSchemaTyped` no-ops on `in == "body"`,
so a schema position keeps the empty schema that is correct there.

### `SimpleSchemaProbe` interface

The target must expose three methods so the validator can inspect
the post-resolution shape and reset it on violation:

```go
type SimpleSchemaProbe interface {
    SimpleSchemaShape() *oaispec.SimpleSchema
    HasRef() bool
    ResetForViolation()
}
```

Implemented structurally by `paramTypable` in
`internal/builders/parameters`, the response header typable in
`internal/builders/responses`, and `resolvers.ItemsTypable` (the
shared array-items adapter). Consumers don't need to import the schema
package — the interface is satisfied by method set.

A target that doesn't implement `SimpleSchemaProbe` is trusted: the
validator no-ops. A `nil` `SimpleSchemaShape` is also trusted — the
caller chose SimpleSchema mode for something that can't surface a
violation, by intent.

#### Array element shapes (go-swagger#1088)

`ItemsTypable` implements the probe, which extends the catch-at-exit
contract **one level down**, to array element shapes. An array IS legal
under SimpleSchema, but its `items` are themselves a SimpleSchema and so
may not be a `$ref`. A named object element (`[]Ele` under `in: query`,
or an array-of-object response header) otherwise resolves to
`items: {$ref}`, which the Swagger 2.0 editor rejects. Because the
element is built through a fresh `WithSimpleSchema` sub-build whose
target is the `ItemsTypable`, the same validator now inspects the items
shape and dissolves the illegal `$ref` to an empty `{}` (named
primitives like `[]Label` still expand inline to `{type: string}` and
are untouched).

When the dissolved shape was a `$ref`, the validator also calls
`ResetPostDeclarations` on the sub-builder: `MakeRef` had discovered the
target's decl and enqueued it, and once the reference is gone that decl
would linger as an orphan definition. A single-type sub-build renders
exactly one target, so every queued decl is reachable only through it;
a decl genuinely referenced elsewhere is re-discovered by that site and
deduplicated by the orchestrator. This is why a `[]Ele` query parameter
no longer drags an unreferenced `Ele` into `definitions`.

### Knock-on cleanups this contract enables

Once `WithSimpleSchema` carries the mode flag explicitly, the
builder can stop sniffing `tgt.In()`:

- `classifierNamedBasic`'s primitive-inline arm (see
  [§classifier-walkers](#classifier-walkers)) now keys on
  `s.simpleSchema` instead of the v1-era `isAliasParam(tgt)`
  predicate. That closes the `in: header` omission documented as a
  resolved quirk below ([§quirks-resolved](#quirks-resolved)).
- The `swagger:alias` per-type author override stays orthogonal —
  it bypasses the model-ref pipeline regardless of mode.

### Walker keyword gating under SimpleSchema mode

The single source of truth for the SimpleSchema vocabulary is
`internal/builders/handlers.IsSimpleSchemaKeyword(name)`. The
package-level `simpleSchemaAllowed` map enumerates every
keyword legal on a non-body parameter, response header, or
items chain within either, per OAS v2's Parameter Object and
Header Object tables.

`schemaBoolHandler` consults this predicate when
`s.simpleSchema == true`:

- **Full-Schema-only Bool keywords** (`readOnly`, `discriminator`)
  trigger a `CodeUnsupportedInSimpleSchema` `SeverityWarning`
  diagnostic and the write is skipped. Even if the path lands on
  a throwaway scratch schema (the common case under SimpleSchema
  mode — `paramTypable.Schema()` returns nil for non-body), the
  diagnostic still surfaces the misuse to the author.
- **`required:`** is accepted by the predicate (it IS
  SimpleSchema-legal as a parameter-level boolean) but the
  schema's Bool handler skips it silently under SimpleSchema mode
  anyway. The parameter-level write — `param.Required = val` —
  lives in `parameters/walker.go:paramRequiredBool`. Headers
  don't carry `required:` at all. The schema walker's full-Schema
  semantics (object-level required-array via
  `enclosing.Required[name]`) don't fit the SimpleSchema shape.
- Number / Integer / String / Raw / Extension dispatchers are
  unchanged: all the keywords they handle are SimpleSchema-legal.

The `IsSimpleSchemaKeyword` set is locked down by unit tests in
the handlers package — any future addition (or removal) of a
SimpleSchema keyword has to update the test alongside the map,
so the contract can't drift silently.

---

## <a id="decl-shape-recheck"></a>§decl-shape-recheck — top-level model shape re-check after type resolution

For a top-level model declaration, `buildFromDecl` dispatches the
doc-comment block (`applyDeclCommentBlock` → `DispatchSchemaLevel0`)
**before** `buildFromType` resolves the Go type onto the schema. So at
dispatch time `schema.Type` is still empty, and the inline `checkShape`
guard — which gates a validation keyword on the schema's resolved type —
sees `""` ("type unknown") and accepts everything. A shape-constrained
keyword on a mismatched scalar model (e.g. `minProperties:` on a
`type Foo string`) would therefore be written and never flagged.

The same ordering has a second consequence, on VALUES rather than on
shape gating. `default:`, `example:` and `enum:` are coerced against
`SchemaTypeOf(ps)`, which is `""` at dispatch time, so `ParseDefault` /
`ParseEnumValues` fall back to the raw string: `default: 8080` on a named
int became the string `"8080"`, `enum: 1,2,3` became `["1","2","3"]` — an
enum no validator can satisfy on an integer schema — and a JSON array
literal became a string holding JSON source.

`RecoerceDeclValues` runs at the same seam as `RecheckSchemaShape` and
re-types those three from their raw form. A value that cannot be read as
the schema's type is **dropped with a warning** rather than emitted at the
wrong type: a document carrying `"notanumber"` on an integer schema is one
no validator accepts, while a document missing a default is merely
incomplete. Field sites always dropped such a value — but silently, which
was the invisible half of the same defect; they now report it too, so the
two sites agree on behaviour AND on reporting.

For `enum:` the drop is per member (`pruneUncoercibleEnum`), and the
warning names the member: dropping narrows a closed set, which is a real
change to the author's contract, so a count would not be enough to act on.
It fails closed, matching what the `swagger:enum` annotation already does
when a const value does not fit.

Coercion **towards** string never fails — every literal has a string form
— so a string-typed schema is skipped outright, and no diagnostic can
arise there. Re-coercion is sound precisely
because the fallback preserved the author's text verbatim: a value still
stored as a string is one that was never typed. String-typed schemas are
skipped, since there the fallback and the correct answer coincide.

Field- and items-level dispatch don't have this problem: their target's
type is already set when their block is dispatched, so `checkShape`
gates correctly inline.

To close the top-level gap, `Build` calls
`handlers.RecheckSchemaShape(&schema, pos, diag)` after `buildFromDecl`
returns. It re-validates the shape-constrained validations now present on
the schema against the resolved `schema.Type`, stripping any that are
illegal for that type and emitting one `CodeShapeMismatch` warning per
strip (at the decl position). `uniqueItems` is intentionally not
rechecked — its grammar keyword (`unique`) carries no type-domain rule,
matching the field/items paths which likewise never shape-gate it.

This is a deliberate post-hoc strip rather than a reorder of
`buildFromDecl`: moving the validation dispatch after type-building would
also move `default:` / `enum:` coercion (which reads `schema.Type` /
`schema.Format`), changing coercion results for top-level scalar models
and rippling through goldens. The recheck is purely additive — it only
removes already-illegal validations — so it leaves every valid case (and
its golden) untouched.

---

## <a id="classifier-walkers"></a>§classifier-walkers — per-call-site classifier walkers and `findAnnotationArg`'s single-word filter

The schema builder dispatches user-classifier annotations
(`swagger:strfmt`, `swagger:type`, `swagger:enum`,
`swagger:default`, `swagger:allOf`, `swagger:alias`) via per-call-
site walker functions in `walker_classifiers.go`. The shape is
deliberate:

- **One walker per call site.** Each walker is named for the call-
  site context (e.g. `classifierTextMarshal`,
  `classifierNamedBasic`, `classifierNamedArrayLike`). Its godoc
  documents which `swagger:<kind>` classifier annotations it
  consumes — explicit per-site contract instead of a single
  catch-all dispatcher.
- **Reads through the ParseBlocks cache.** Each walker calls
  `s.ParseBlocks(cg)` so a single CommentGroup is parsed exactly
  once per build regardless of how many walkers inspect it.
- **Site-local writes.** The walker performs the call-site's writes
  directly onto the typable target — both lookup and side effect
  encapsulated.

Correctness first; if a future re-read finds genuine redundancy,
the factorisation is mechanical and safer last.

### `findAnnotationArg` and the single-word filter

`findAnnotationArg(cg, kind)` returns the first positional argument
of the first `Block` of the given annotation kind, **filtered to
non-empty single-word arguments**:

```go
if strings.ContainsAny(arg, " \t") {
    continue
}
```

The single-word filter only matters for annotation kinds whose
lexer arg-classifier doesn't already split on whitespace — namely
`AnnType` (via `argTypeRef`) and `AnnDefaultName` (via
`argDefaultValue`). Kinds that go through `firstIdent` in the lexer
(`AnnStrfmt`, `AnnName`, `AnnAllOf`, `AnnModel`, `AnnResponse`,
`AnnEnum`) already produce single-token args, but the filter is
harmless on those.

The check matches v1's de-facto `\S+`-anchored capture, which
silently rejected prose lines that happened to open with
`swagger:<kind>` followed by a sentence. The
`testdata/enhancements/named-basic` fixture documents this trap
with a `swagger:type so the scanner emits ...` prose line preceding
the real `swagger:type string` annotation — without the filter, the
prose's "so" would pre-empt the real arg "string".

`findAnnotationArg` reads through the per-Builder `ParseBlocks`
cache, so the lookup is parse-once per CommentGroup and the
`ParseAll`-aware multi-annotation case still surfaces every
annotation of interest.

### Walker inventory

| Walker | Call site | Consumes |
|---|---|---|
| `classifierTextMarshal` | `buildFromTextMarshal` end-of-pipe | `swagger:strfmt` |
| `classifierNamedTypeOverride` | `buildFromType` named fallback, `buildFromStruct` pre-pass | `swagger:type` |
| `classifierNamedBasic` | `buildNamedBasic` | cascade: `swagger:strfmt → swagger:enum → swagger:type`, then the SimpleSchema-mode primitive-inline branch (see [§simple-schema-mode](#simple-schema-mode)). `swagger:default` and `swagger:alias` are deprecated sinks: each raises `validate.deprecated` and falls through. `swagger:default` used to be TERMINAL here, returning handled on a target it never wrote — which published a typeless schema for the declared type |
| `classifierNamedArrayLike` | `buildNamedArray` / `buildNamedSlice` | `swagger:strfmt`, `swagger:type` |
| `classifierAliasTargetStrfmt` | `buildNamedAllOf` (struct + interface arms) | `swagger:strfmt` |
| `classifierStructPreBuildType` | `buildFromStruct` top | `swagger:type` |
| `classifierNamedStructStrfmt` | `buildNamedStruct` strfmt-first branch | `swagger:strfmt` |
| `scanFieldDoc` | field-level FieldWalker (`applyFieldCarrier`) | `swagger:ignore`, `swagger:name`, `swagger:strfmt`, `swagger:type` (with the same single-word filter), `swagger:allOf` |

### <a id="enum-typing"></a>Enum typing — the declared Go type wins

`classifierNamedBasic`'s `swagger:enum` arm delegates to
`applyEnum`, which resolves the schema's `type` / `format` from
`utitpe` — the enum's **declared** underlying basic type — and then
normalises every member to it via
`validations.CoerceConstant`.

This is deliberate, and it is the opposite of what the code did
before.

**What the scanner loses is the value's width, not the type.** An
enum member arrives as an `any` holding one of five Go
representations — `int64`, `uint64`, `float64`, `string`, `bool` —
because that is what `go/constant` converts to
(`constant.Int64Val`, `Float64Val`, …). A member of an `int8` enum
and a member of an `int64` one are both an `int64` in that box;
nothing about the box says which. The **declared Go type is still
right there** on the builder side (`utitpe`), and it is the only
place `format: int8` can come from — so the value is the wrong
thing to ask.

Typing the schema from `reflect.TypeOf(values[0])` — asking the
box — therefore:

- collapsed `int8` / `uint16` / `float32` enums to
  `int64` / `int64` / `double`, while the *same* Go types on a
  plain field resolved correctly through
  `resolvers.SwaggerSchemaForType`;
- made the emitted type depend on the **declaration order** of the
  const block — a `float64` enum whose first member was written
  `= 0` (an INT literal) emitted `{type: integer}` while still
  carrying its fractional members, a schema no validator can
  satisfy, and reordering the block silently changed the output.

Both call sites (`buildFromDecl`'s named-basic arm and the field
site) pass the declared `*types.Basic`, so definitions, non-body
parameters and response headers all get the same treatment. On the
SimpleSchema targets that means the full set — `in: path` /
`query` / `header` / `formData`, the `items` of an array-typed one,
and response headers including their `items` — all carry the
members with the declared `type` / `format`.

### The underlying type is not always the whole answer

"Declared type" means the declaration, not `Underlying()`. The two
differ as soon as a type is written **over another named type**:

```go
type Kind strfmt.UUID          // Underlying() == string. The uuid is gone.
```

`strfmt.UUID` carries the `swagger:strfmt uuid` annotation, and
go/types offers no way back to it — `Underlying()` jumps straight
to the bottom `string`. Without the enum annotation this type
still emits `{type: string, format: uuid}`, because the ordinary
declaration path (`buildFromDecl`) resolves `Spec.Type` — the
right-hand side — rather than the underlying. The enum arm typed
from the underlying alone, so **annotating a type as an enum cost
the author their format**, silently.

`applyEnumType` closes that: for a string-domain enum it first
asks `inheritedStrfmt`, which walks the chain of declarations to
the right (`Spec.Type`, repeatedly) looking for a
`swagger:strfmt`, so an indirection several redefinitions long
resolves like a single one. The walk stops at the basic bottom, at
a declaration the scan cannot see the source of, or at a repeat.
Only `swagger:strfmt` is inherited — a type-*changing* annotation
on an intermediate would contradict the enum's own members rather
than decorate them — and only onto a string, which is all a strfmt
can legally decorate.

`basicSchemaType` maps that type to the Swagger value domain from
go/types' own kind bits (`IsInteger` / `IsFloat` / `IsString` /
`IsBoolean`) rather than a name table, so `byte` and `rune` need no
special casing and a domain-less type (complex) yields `""` — the
cue to leave values untouched.

### A `rune` / `byte` enum is an integer enum, and that is correct

```go
type Letter rune
const LetterA Letter = 'a'     // → {type: integer, format: int32, enum: [97]}
```

An author who writes character literals usually expects
`enum: ["a"]`, and gets `97`. That surprise is Go's, not ours: a
scalar `rune` is an `int32` on the wire, so
`json.Marshal(Letter('a'))` is `97`, and `encoding/json` **refuses**
to unmarshal `"a"` into that field. A string-typed schema here
would describe a payload the server rejects — the integer is the
only faithful answer, and no rule about enums can change it.

The author's remedy is a type whose *wire form* is a string:
`type Letter string` with `LetterA Letter = "a"`. Left visible
rather than papered over; the `byte` / `rune` cases in
`testdata/bugs/3412/constforms` pin the behaviour.

See [§enum-const-values](../validations/README.md#enum-const-values)
for the coercion rules on the values themselves.

---

## <a id="quirks"></a>§quirks — known behavioural caveats

Entries are split into two groups: **[Resolved in this refactor](#quirks-resolved)** lists
quirks fixed in the current pass with a short note on how, so a
future reader can find the change and the rationale. **[Still open](#quirks-open)** lists
quirks the refactor either deliberately did not touch (deferred to
v2 / design call required) or documents as intentional behaviour.

---

## <a id="quirks-resolved"></a>§quirks-resolved — fixed in this refactor

### ✅ `recognizeRawMessage` now emits an empty schema (was `{type: object}`)

`json.RawMessage` is `[]byte` underneath but JSON-marshals as
arbitrary JSON. The recognizer now emits an empty schema (`{}`),
which in Swagger 2.0 / JSON Schema means "any type" — the most
faithful representation of `RawMessage`'s contract. Previous
behaviour emitted `{type: object}` as a narrower approximation.

**Fix:** `recognizeRawMessage` arm in `applySpecialType` rewritten to
call `_ = target.Schema()` (the "any" pattern used by `recognizeAny`)
instead of `target.Typed("object", "")`. Golden delta captured in
`go123_special_spec.json`'s `Message` property.

### ✅ `recognizeError` x-go-type extension now honours `SkipExtensions`

The `error` arm of `applySpecialType` writes `x-go-type: error` in
addition to typing the target as `{string, ""}`. This is for
downstream tooling that wants to detect the Go-error origin. The
write is now gated by the `skipExt` argument threaded into
`applySpecialType` / `applyStdlibSpecials` from `s.skipExtensions`,
so `SkipExtensions=true` suppresses it like any other vendor
extension.

**Fix:** added `skipExt bool` parameter to `applySpecialType` and
`applyStdlibSpecials`; gated the `target.AddExtension(...)` call in
the `recognizeError` arm. Eight schema-internal call sites updated to
pass `s.skipExtensions`. Golden-neutral (no existing fixture combines
the `error` shape with `SkipExtensions=true`).

### ✅ Field-level `swagger:type` on `json.RawMessage` fields

A struct field of type `json.RawMessage` with a field-level
`swagger:type object` / `swagger:type array` annotation now produces
the user-specified shape instead of silently emitting the recognizer
default. Pre-fix: `scanFieldDoc` only consumed
`ignore` / `name` / `strfmt` / `allOf`, so field-level `swagger:type`
was dropped before `applyBlockToField` ran.

**Fix:** added `TypeOverride` to `fieldDoc`; `scanFieldDoc` consumes
`AnnType` with a single-word filter (mirroring `findAnnotationArg`,
since `AnnType` uses TrimSpace and can carry prose on noise lines);
`applyFieldCarrier` applies the override after `buildFromType` —
tries `SwaggerSchemaForType(name, …)` first, falls back to
`buildFromType(c.propType.Underlying(), …)` on unknown leaves like
`"array"` so item shapes are computed from the Go type. Fixture
`testdata/enhancements/raw-message-override/` (case C); golden
`enhancements_raw_message_override.json`.

### ✅ Wrapper-decl `swagger:type` honoured at top-level definition

A named wrapper of `json.RawMessage` (or any other type the recognizer
would otherwise short-circuit) decorated with `swagger:type` on the
decl now emits the user-specified shape at its **own** top-level
definition, not just at field reference sites. Pre-fix: only the
field-reference path consulted the wrapper's `swagger:type`; the
top-level definition emitted an empty schema because `buildFromDecl`
dispatched on `ti.Type` (the RHS) and the RHS recognizer fired
before any wrapper-side classifier could.

**Fix:** `buildFromDecl` now calls `classifierNamedTypeOverride` on
`s.Decl.Comments` before the kind-dispatch. Known leaves (`object`,
`string`, …) terminate; unknown leaves (`array`) fall back to
`s.Decl.ObjType().Underlying()` so items / properties are filled
from the Go-level shape. Isolation fixture
`testdata/enhancements/wrapper-decl-type-override/` —
`BareWrapperObject` / `BareWrapperArray`.

### ✅ `in: header` parameters now inline named-basic types

Pre-fix, `classifierNamedBasic`'s primitive-inline arm only fired
for `in: query` / `in: path` / `in: formData` — `header` was
silently omitted from the `isAliasParam` predicate (carried over
verbatim from v1's `parsers.IsAliasParam`). So a header parameter
typed as a named string `type SessionID string` resolved through
`FindModel → makeRef` and emitted `{$ref: "#/definitions/SessionID"}`
— invalid under OAS v2 SimpleSchema, which forbids `$ref` on
non-body parameter sites.

**Fix:** the `isAliasParam(tgt)` `In()`-sniffing predicate replaced
with the M1 `s.simpleSchema` flag (set by `WithSimpleSchema`). The
parameter bridge now wires `WithSimpleSchema` for all four non-body
locations uniformly (`query` / `path` / `header` / `formData`), so
the primitive-inline arm covers `header` automatically. The
predicate function deletes (sole consumer).

Two paths through the arm remain orthogonal: the SimpleSchema flag
is caller-driven (parameter/header build mode); `swagger:alias` on
the decl is a per-type author override. Either triggers
`SwaggerSchemaForType(underlying basic name)` on the typable.

Isolation fixture `testdata/enhancements/header-named-basic/` and
test `internal/integration/coverage_header_named_basic_test.go`
pin the post-fix shape. Responses won't pick up the same fix until
M2 wires `WithSimpleSchema` on header-field builds; the response
edges fixture covers a different (strfmt-tagged) shape already.

---

### ✅ Named-strfmt + `swagger:model` combo (was 🟡 deferred)

A type carrying both `swagger:strfmt phone` and `swagger:model` used to emit an
inconsistent pair: `{type: string, format: phone}` at the field site, but a
struct walk (`{type: object, properties: …}`) for the top-level definition. The
decl-level strfmt now wins and the field `$ref`s it — verified in
`enhancements_named_struct_tags-ref.json`: `PhoneNumber` is
`{type: string, format: phone}` and `Contact.phone` is a `$ref` to it.

Fixed by the F-series pass (`8e20d2f`, quirk F1), not by the attempt described
in the original entry — which was reverted. History:
`.claude/plans/archive/deferred-quirks.md` D3.

### ✅ Cross-package definition-name collisions (was 🟡 silently overwrite)

Two packages declaring the same identifier (`pkg/a.User`, `pkg/b.User`) both
mapped to `definitions["User"]`, and the second build silently overwrote the
first — one `User` in the output, no record of the collision.

Fixed by the name-identity / cyclic-`$ref` work: every definition is keyed by a
compiler-unique `DefKey` (`<pkgpath>/<name>`) while building, and a final reduce
stage projects each back to the shortest unique name, deconflicting collisions
(`AWidget` / `BWidget`) and raising a `scan.renamed-definition` Hint. See
[§discovery](#discovery) and `.claude/plans/name-identity-cyclic-ref.md`.

## <a id="quirks-open"></a>§quirks-open — still open

> **Where open quirks live.** This section documents caveats *of this package*.
> The project-wide register of what is actually open — verified, with the stale
> historical registers called out — is `.claude/plans/quirks-open.md`.

### 🟦 `interface{}` literals (documented behaviour)

A bare `interface{}` field hits the `*types.Interface` arm of
`buildFromType` (anonymous, not Named). It produces an empty
schema. The user-named `type X interface{}` is a `*types.Named`
with empty `Underlying()` and emits as `$ref` to a definition
with no `properties` — JSON-equivalent to "any object" in v1.
Behavioural; changing it would break consumers.

### 🟦 Generic declarations vs instantiations (documented behaviour)

Generic **declarations** (e.g. `GenericSlice[T any] []T`) are
processed but their schemas are essentially empty — the type
parameter `T` is filtered out by `UnsupportedBuiltinType` as a
`*types.TypeParam`. Generic **instantiations**
(e.g. a field of type `GenericSlice[int]`) emit correctly with
the substituted underlying via the `TypeArgs` short-circuit
([§dissolve-named](#dissolve-named)). No bug — generic decls
without a concrete instantiation simply have no representable
schema.

### 🟡 A field-level enum override discards the type's per-value docs, silently

When a field whose type is marked `swagger:enum TypeName` carries its own
`enum:` override, the inherited `x-go-enum-desc` is stripped along with the
replaced values — and **no diagnostic is raised**, so the per-value docs
`TypeName` contributed vanish without a trace. Reproduced by
`testdata/enhancements/enum-overrides` case E (`NotificationE`).

Whether the docs should be filtered through (when the override *subsets* the
type's values) or dropped with a Hint (when it narrows to *different* ones) is a
design call that belongs with the enum feature, not with this package: see
`.claude/plans/features/enum-richer-values.md` §1.2b.

The stripping itself lives in `handlers/dispatch_schema.go:clearStaleEnumDesc`,
which is where either resolution would land.
