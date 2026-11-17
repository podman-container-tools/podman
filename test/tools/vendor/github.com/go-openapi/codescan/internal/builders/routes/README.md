# routes — maintainer notes

This document is the long-form companion to the `routes` builder
code. The source files keep godoc concise; complex invariants,
design trade-offs, and intentionally-deferred follow-ups live here.

The `routes` package builds OAS v2 path entries from a single
`swagger:route` annotation — Summary, Description, schemes,
deprecated, consumes, produces, security, parameters, responses,
and vendor extensions. One `Builder` runs per route annotation;
one grammar parse runs per route.

---

## Table of contents

- [§overview](#overview) — files and per-file responsibilities
- [§builder](#builder) — `Builder`, `Build`, and the dispatch chain
- [§dispatch](#dispatch) — `dispatchRouteKeyword` per-keyword routing
- [§body-parsers](#body-parsers) — parameters / responses body lowering
- [§extensions](#extensions) — typed extensions surfaced by the lexer
- [§quirks](#quirks) — known behavioural caveats

---

## <a id="overview"></a>§overview — files and responsibilities

| File | Contents |
|------|----------|
| `routes.go` | `Builder` (embeds `*common.Builder`), `Inputs`, `NewBuilder`, top-level `Build` |
| `walker.go` | Grammar dispatch — `applyBlockToRoute`, `dispatchRouteKeyword`, parameter and response materialisation |
| `errors.go` | `ErrRoutes` sentinel |

The body grammars for `parameters:` and `responses:` live in
`internal/parsers/routebody`. The routes builder calls
`routebody.ParseParameters` / `routebody.ParseResponses` to lower
the raw body to typed `ParamDecl` / `ResponseDecl` slices, then
walks each decl through the handlers seam to populate the
operation. See [§body-parsers](#body-parsers).

Extensions are NOT parsed here — they ride grammar's typed-extensions
surface directly. See [§extensions](#extensions).

The builder embeds `*common.Builder` (Ctx, ParseBlocks cache,
diagnostic sink). Its `Decl` field is nil — routes build off a
path annotation, not a declaration — so `MakeRef` and other
Decl-anchored helpers must not be called.

## <a id="builder"></a>§builder — top-level `Build`

`Build(tgt *spec.Paths)` looks up the path-item slot on `tgt`,
allocates or reuses the operation for the HTTP verb via
`operations.SetPathOperation`, attaches the route's `Tags`, then
dispatches into `applyBlockToRoute` for the header content and
per-keyword bodies.

`applyBlockToRoute` parses `route.Remaining` (the
`*ast.CommentGroup` after the `swagger:route` header line has been
stripped by `parsers.ParseRoutePathAnnotation`) into a grammar
Block. `block.Title()` and `block.Description()` give the
lexer-classified prose; `block.Properties()` yields one entry per
recognised route-level keyword. Items-depth entries
(`items.maximum:` and friends) are skipped — they belong to a
nested schema, not the route header.

After the property loop, `applyBlockToRoute` reads the typed
extension and security surfaces straight off the block.
The lexer routes their raw bodies through `yaml.TypedExtensions`
and the security sub-parser at lex time, so the dispatcher skips
them and the orchestrator picks up the typed values directly.

## <a id="dispatch"></a>§dispatch — `dispatchRouteKeyword`

One switch over `p.Keyword.Name`. Per-shape:

- **List-shaped keywords** (`schemes`, `consumes`, `produces`)
  flow through `Property.AsList`, which unifies inline comma-lists,
  multi-line bare-line bodies, and YAML-style `-` markers. The
  resulting `[]string` is assigned directly onto the operation.
- **`tags`** also flows through `Property.AsList` (a plain string
  list — the meta `Tags:` object shape is a different builder), but
  is **unioned** onto `op.Tags` via `unionTags` rather than
  assigned, so route-header-line tags and body `Tags:` names merge
  with duplicates dropped (go-swagger#2655).
- **Inline boolean** (`deprecated`) reads `p.Typed.Boolean` after
  an `IsTyped()` guard, so malformed inputs (which leave
  `ShapeNone`) are skipped silently.
- **Body-parser keywords** (`parameters`, `responses`) hand the raw
  `Property.Body` and `Property.Pos` to `routebody`; the returned
  decl slices fan into `buildRouteParam` / `buildRouteResponse`.

`extensions:` and `security:` are not on the dispatcher — see
[§extensions](#extensions) and the security surface read at the
end of `applyBlockToRoute`.

## <a id="body-parsers"></a>§body-parsers — parameters and responses

Two body shapes are too domain-specific to express through grammar's
keyword table and live in `internal/parsers/routebody`:

- **Parameters** — the `+ name:` block syntax used to describe
  route parameters inline. `routebody.ParseParameters` returns one
  `ParamDecl` per parameter, each carrying head fields (`Name`,
  `In`, `Required`, `Description`, `TypeRef`, `Format`,
  `AllowEmpty`) and a sub-Block of validation properties.
- **Responses** — the `200: body Foo description text`
  mini-language for status-code → response-or-definition mappings.
  `routebody.ParseResponses` returns one `ResponseDecl` per entry,
  each carrying `Code`, `BodyTypeRef` / `ResponseRef`, `Arrays`,
  and `Description`.

`buildRouteParam` and `buildRouteResponse` materialise each decl
into the corresponding `spec.Parameter` / `spec.Response`:

- **Non-body params** dispatch through
  `handlers.DispatchParamLevel0` (SimpleSchema). Validation
  properties pass through `typeGateBlock` first, which drops any
  keyword that is not legal for the declared type and emits a
  `CodeShapeMismatch` diagnostic per dropped keyword so the author
  sees the loss.
- **Body params** populate `param.Schema` from the type reference
  (primitive type → typed schema; otherwise a `$ref` resolved
  against `r.definitions` with optional array wrapping), then
  dispatch through `handlers.DispatchSchemaLevel0`. The
  description lives only on the parameter — the referenced model
  owns the schema-level description.
- **Responses** assemble `op.Responses` from each `ResponseDecl`,
  routing `"default"` to `Responses.Default` and integer codes to
  `Responses.StatusCodeResponses`. Ref resolution follows the
  definition-fallback rule documented in
  [§quirks](#quirk-definition-fallback).

`normaliseSimpleType` maps short type spellings (`bool` →
`boolean`) to their OAS v2 canonical forms before the parameter
type lands on the spec.

### Format timing on SimpleSchema parameters

`param.Format` is assigned **after** `DispatchParamLevel0` on
purpose. `spec.SimpleSchema.TypeName()` returns `Format` when it
is non-empty, so `validations.CoerceValue` would key
default/example coercion off the format string instead of the
type. Setting Format post-dispatch keeps `param.Type` stable
through coercion.

## <a id="extensions"></a>§extensions — typed via grammar's lexer

Routes consumes vendor extensions through the same path schema,
parameters, and responses use:

```go
for ext := range block.Extensions() {
    op.AddExtension(ext.Name, ext.Value)
}
```

Grammar's lexer recognises `extensions:` (and `infoExtensions:`)
raw blocks and, at lex time, runs the body through
`yaml.TypedExtensions`:

1. `normaliseExtensionBody` (dedent + tab→space conversion)
2. `yaml.Unmarshal` into `any`
3. `yamlutils.YAMLToJSON` to coerce to JSON-typed values
4. `json.Unmarshal` into `map[string]any`

The result is exposed on the parsed Block as `[]Extension` with
each entry carrying `Name`, `Pos`, and the JSON-typed `Value`
(`bool` / `float64` / `string` / `[]any` / `map[string]any`).

## <a id="quirks"></a>§quirks — behavioural caveats

### <a id="quirk-block-comment-prefix"></a>Block-comment prefix on Title / Description

Route docs are most often `/* ... */` block comments. Each non-first
line of such a comment carries a leading tab / whitespace indent
that `//`-style line comments shed naturally (the preprocessor
strips the `// ` prefix per line). Grammar's lexer classifies the
prose correctly (Title vs Description, markdown ATX heading
stripping included) but preserves the raw source text. Consumers
that surface the prose verbatim need to shave the per-line
comment-marker noise (`space`, `tab`, `/`, `*`, `-`, optional `|`)
off the result themselves.

The lexer deliberately does not do this stripping: its contract is
"preserve source verbatim, classify into tokens." Comment-marker
noise is a consumer-side concern, and stripping at the lexer would
break the LSP-diagnostics target (per-line `file:line:col`
positions must survive Preprocess).

### <a id="quirk-definition-fallback"></a>Response definition-fallback

A response whose ref name does not appear in `r.responses` but
does appear in `r.definitions` is silently promoted to a body ref
(`Schema: $ref: #/definitions/<name>`) rather than emitting an
invalid `$ref: #/responses/<name>`. This kindness exists because
authors commonly reference a model by name in a `responses:` block
without first declaring a `swagger:response` wrapper.

Dangling refs (not in either map) emit a `CodeInvalidAnnotation`
diagnostic and the response is dropped — the author sees the loss
rather than discovering it as a malformed spec downstream.

### <a id="quirk-type-gating"></a>SimpleSchema type-gating diagnostics

`typeGateBlock` filters validation properties through
`validations.IsLegalForType` for the parameter's declared type and
emits `CodeShapeMismatch` per dropped keyword. A SimpleSchema
parameter with no declared `type:` drops every validation property
with a diagnostic explaining the loss — the author sees the
mismatch rather than a silently-empty validation surface.

### <a id="quirk-coercion-errors"></a>Coercion errors surface as diagnostics

`DispatchParamLevel0` may return an error when a `default:` or
`example:` value cannot be coerced to the declared type. The
router surfaces the first such error as a
`CodeInvalidAnnotation` diagnostic rather than dropping it silently
so the author sees the bad input.

### <a id="quirk-extensions-typed-values"></a>Extensions are JSON-typed

Vendor extensions ride `block.Extensions()` and surface as
JSON-typed values (`bool`, `float64`, `string`, `[]any`,
`map[string]any`). Goldens treat `x-some-flag: false` as `false`
(bool), not `"false"` (string). The full lex pipeline is documented
in [§extensions](#extensions).
