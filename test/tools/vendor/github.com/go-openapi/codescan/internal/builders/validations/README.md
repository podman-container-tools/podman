# validations — maintainer notes

This document is the long-form companion to the `validations` package
code. The source files keep godoc concise; complex invariants,
design trade-offs, and intentionally-deferred follow-ups live here.

The `validations` package owns cross-builder validation and coercion
concerns shared by the `schema`, `parameters`, `responses`, and
items/headers code paths. Its two halves are:

- **Value coercion** (`coerce.go`) — turns raw annotation text into
  the Go value implied by the target schema's `type` + `format`,
  for keywords whose payload is a primitive literal (`default:`,
  `example:`, `enum:`). `const_values.go` is the sibling half of
  this concern: it normalises values the scanner read out of a
  Go **const block** rather than out of annotation text
  (see [§enum-const-values](#enum-const-values)).
- **Shape legality** (`shape.go`) — answers "is this keyword legal
  on a schema of this type?" against the JSON-Schema draft-4
  domain rules that Swagger 2.0 inherits.

---

## Table of contents

- [§contract](#contract) — why these helpers live here and not in the grammar
- [§coercion-dispatch](#coercion-dispatch) — `CoerceValue` / `ParseDefault` / `ParseEnumValues` routing
- [§enum-shapes](#enum-shapes) — JSON-array form vs comma-list form
- [§enum-const-values](#enum-const-values) — `CoerceConstant` and the declared-type rule
- [§format-axis](#format-axis) — why `Format` is reserved but not consulted today
- [§format-compat](#format-compat) — `IsFormatCompatible` type×format legality
- [§type-domain-table](#type-domain-table) — the keyword-vs-type legality table
- [§empty-type](#empty-type) — how an unknown schema type is treated
- [§quirks-open](#quirks-open) — deferred follow-ups

---

## <a id="contract"></a>§contract — why these helpers live in the builder layer

The grammar parser produces a typed annotation block but does not
resolve the Swagger `type` / `format` of the field, parameter,
or header the block is decorating — that resolution is the
builder's job, and it depends on the surrounding Go type system.

Two consequences:

1. **Coercion of `default:` / `example:` / `enum:` payloads** cannot
   happen at parse time. The grammar lexes `default: 3` as the raw
   string `"3"`; only the builder has resolved whether the target is
   `integer` (so the value should be `int(3)`), `string`
   (so the value stays `"3"`), or `array` (so the value should
   `json.Unmarshal` into `[]any`).
2. **Keyword-vs-type legality** (`pattern:` only on `string`,
   `multipleOf:` only on `number`/`integer`, ...) similarly needs
   the resolved type. The grammar accepts the keyword
   syntactically; the builder applies the domain rule once the
   target's `Type` is known.

This package is the seam where those two concerns meet — small,
type-aware helpers that the builder layer calls from its Walker
callbacks.

## <a id="coercion-dispatch"></a>§coercion-dispatch — `CoerceValue` / `ParseDefault` / `ParseEnumValues`

`CoerceValue(s, schema *spec.SimpleSchema)` is the primitive
coercer. It dispatches on `schema.TypeName()` — a Swagger helper
that returns `Format` when set, falling back to `Type`. This
Format-wins behaviour is convenient at the items/parameter sites
(where the SimpleSchema is the authoritative source) but is the
wrong axis for the schema-builder paths that already hold a
resolved `(type, format)` pair.

`ParseDefault(s, schemaType, schemaFormat)` is the explicit
two-axis entry point. It dispatches on `schemaType` only,
ignoring `schemaFormat` for routing. The `schemaFormat`
argument is reserved for future per-format paths (e.g.
size-bounded integer parsing for `int32` vs `int64`) but is
unused today.

`ParseEnumValues(val, schemaType, schemaFormat)` mirrors
`ParseDefault` for enum payloads, delegating per-element typing
to `CoerceEnum`.

Dispatch table (after stripping surrounding quotes from
`TypeName()`):

| Source label | Dispatcher | Coercer |
|---|---|---|
| `integer`, `int`, `int64`, `int32`, `int16` | both | `strconv.Atoi` |
| `bool`, `boolean` | both | `strconv.ParseBool` |
| `number`, `float64`, `float32` | both | `strconv.ParseFloat` (bitSize=64) |
| `string` | both | `unquoteIfQuoted` — strips a surrounding quote pair (F8) |
| `object` | both | `json.Unmarshal` into `map[string]any` |
| `array` | both | `json.Unmarshal` into `[]any` |
| anything else / `nil` schema | both | raw string unchanged |

The `string` arm strips one pair of surrounding double quotes from a
quoted literal (`example: "Foo"` → `Foo`, `example: ""` → the empty
string) while leaving a bare value (`example: Foo`) untouched — the
quotes are delimiters, not content (quirk F8; go-swagger#2547 / #2899).
Only the plain `string` type is unquoted here; string *formats*
(`date`, `uuid`, …) surface their format name as the dispatch label and
fall to the raw-string arm.

Numeric and boolean parse errors are surfaced to the caller so
the consumer can decide whether to emit a diagnostic. JSON
parse failures on `object` / `array` are absorbed and the raw
string is returned — the assumption is that an author who wrote
`default: notjson` against an object target intended a textual
placeholder rather than a machine-readable default. A future
strict-mode option could turn this into a diagnostic.

## <a id="enum-shapes"></a>§enum-shapes — JSON-array form vs comma-list form

`CoerceEnum` accepts two input shapes for the `enum:` annotation:

- **JSON-array form** — `enum: ["a", "b", "c"]`. Detected by
  attempting `json.Unmarshal` into `[]json.RawMessage`. Each
  element is `strconv.Unquote`d (so the literal `"a"` becomes
  `a` before per-value coercion) and then routed through
  `CoerceValue`.
- **Comma-list form** — `enum: a, b, c`. Triggered when the
  JSON-array unmarshal fails. Each comma-separated token is
  `TrimSpace`d before per-value coercion so `enum: a, b`
  produces `["a", "b"]`, not `["a", " b"]`. A surrounding `[ ]`
  pair is stripped first: the bracketed `enum: [a, b, c]` form has
  unquoted values, so it is not valid JSON and lands here — without
  the strip the brackets glue onto the first/last value
  (go-swagger#2396). The quoted (`["a","b"]`) and numeric
  (`[1,2]`) bracketed variants are valid JSON and take the
  JSON-array path instead.

Per-element coercion is the same `CoerceValue` path as
`default:` / `example:`, so type-aware typing applies
uniformly across the three keywords.

## <a id="enum-const-values"></a>§enum-const-values — `CoerceConstant` and the declared-type rule

`swagger:enum` members do not come from annotation text: the
scanner reads them off a Go const block. It has two readings of
that block, and `CoerceConstant` reconciles them.

The **primary reading** takes the values from the type-checker
(`go/types` constants). There, a constant's kind already follows
its declared type — a `float64`-typed `= 0` is a `constant.Float`,
an `int`-typed `= 42.0` is a `constant.Int` — so `CoerceConstant`
has nothing to do and passes the value through.

The **degraded reading** (a partially loaded package, where the
type-checker has no value) falls back to literal syntax, and there
the declared type is invisible:

```go
type Tilt float64

const (
    TiltFlat Tilt = 0      // INT literal   → int64(0)
    TiltDown Tilt = -0.5   // FLOAT literal → float64(-0.5)
)
```

One enum, two Go kinds, one declared type. `CoerceConstant`
normalises against the Swagger type resolved from that declared
type, so the degraded reading emits what the primary one would.

Two rules make this safe:

- **The declared type wins, always.** The caller
  (`schema.applyEnum`) resolves `type` / `format` from the Go
  type, never from the values. Before this rule existed, the type
  was read off `reflect.TypeOf(values[0])`, which collapsed `int8`
  to `int64` and `float32` to `double` — and let the *declaration
  order* of the const block decide the schema type, so a float
  enum whose first member was written `= 0` emitted
  `{type: integer}` carrying fractional members.
- **A non-representable value is never emitted.** `(nil, false)`
  tells the caller to drop the member and raise a diagnostic
  rather than write something the schema's own `type` forbids.

An unresolved (empty) Swagger type passes values through
untouched: with no type to normalise against, guessing would be
worse than leaving the scanner's reading intact.

## <a id="format-axis"></a>§format-axis — `Format` is reserved but not routed

`ParseDefault` and `ParseEnumValues` accept a `schemaFormat`
argument that is currently discarded — the helpers underscore-
assign it explicitly so the surface stays stable for callers
while the format-aware paths are deferred.

Two paths could exercise it later:

- **Size-bounded integer parsing.** `int32` could parse via
  `strconv.ParseInt(s, 10, 32)` and surface overflow as a
  diagnostic rather than the silent truncation `strconv.Atoi`
  performs on `int32` targets.
- **Float precision.** `float32` could parse with `bitSize=32`
  to match the target's range; today both float widths share
  the `bitSize=64` path.

Neither is strictly required for spec correctness — the
emitted Swagger document carries the value via `interface{}`
and downstream consumers re-validate against `(type, format)`
themselves. They are tagged here as straightforward refinements
once a concrete consumer asks for them.

## <a id="format-compat"></a>§format-compat — `IsFormatCompatible` type × format legality

`IsFormatCompatible(schemaType, format)` (in `format.go`) is a sibling of
[`IsLegalForType`](#type-domain-table) on the **format** axis: it answers
"may this `format` ride on a schema already typed `schemaType`?" It exists for
the `swagger:type` + `swagger:strfmt` combination, where `swagger:type` wins on
the type axis and the strfmt format is applied as a **supplementary hint only
when it is consistent with that type** (the F3 reconciliation — see
`.claude/plans/archive/quirks-F-series-fix.md`). It is **not** used for the
strfmt-alone path, where strfmt still forces `{type: string, format: X}`
(go-swagger#1512).

| Resolved type | Formats accepted |
|---|---|
| `string` | **any** (strfmt is string-oriented) |
| `integer` | `int{n}` (`int`,`int8`…`int64`) + the swagger-extension `uint{n}` |
| `number` | the integer set **+** float widths (`float`,`double`,`float32`,`float64`) |
| `boolean` / `object` / `array` / `file` | none |

`int32`/`int64` are the only OAS-2-official integer formats and `float`/`double`
the only official number formats; the wider Go-spelled set round-trips back to
Go (the same vendor convention #1512 documents). An empty `format` is trivially
compatible (nothing to apply); an empty/unknown `schemaType` is accepted
best-effort, mirroring [§empty-type](#empty-type). A `false` result returns a
diagnostic-ready hint naming the offending format and type.

## <a id="type-domain-table"></a>§type-domain-table — keyword × Swagger type legality

`keywordTypeRules` (in `shape.go`) carries the per-keyword
type-domain table sourced from JSON-Schema draft-4 (the
dialect Swagger 2.0 inherits):

| Keyword family | Legal on |
|---|---|
| `pattern`, `minLength`, `maxLength` | `string` |
| `maximum`, `minimum`, `multipleOf` | `integer`, `number` |
| `minItems`, `maxItems`, `uniqueItems` | `array` |
| `minProperties`, `maxProperties`, `patternProperties` | `object` |

Keywords intentionally absent from the table:

- **`required`, `readOnly`, `deprecated`, `discriminator`** — the
  rule is type-independent (or, in the case of `discriminator`,
  the OAS-level legality check happens elsewhere). The table
  returns "no rule" and `IsLegalForType` accepts the keyword
  for any type.
- **`default`, `example`, `enum`** — coerced via `CoerceValue` /
  `CoerceEnum`, so they are legal on any type and the value
  conforms by construction.

The table is returned by a function rather than held as a
package variable to keep the package `gochecknoglobals`-clean
and to leave room for a future `WithRules(...)` constructor
that lets callers extend the table for custom keywords.

## <a id="empty-type"></a>§empty-type — `schemaType == ""` is accepted

`IsLegalForType` treats an empty `schemaType` as "type unknown"
and returns `ok=true` with no hint. Two situations produce an
empty type at the call site:

- The grammar parsed a keyword before the type has been
  resolved (the typeless preamble case).
- The target's type is genuinely indeterminate — a
  free-form schema such as `additionalProperties: true`.

In both cases the caller — typically a Walker callback in the
schema or parameters builder — is responsible for deciding
whether to apply the keyword. The package-local rule is
"best-effort apply": never block on an unknown type from this
seam.

`Format` is intentionally not consulted by `IsLegalForType`.
Format is a refinement of type (`int32` is an `integer`-typed
field with `format: int32`); the domain rules apply at the
type level. A future `IsLegalForFormat` sibling could add
format-specific constraints (e.g. `pattern:` only on
`format: regex` strings) without disturbing this surface.

## <a id="quirks-open"></a>§quirks-open — deferred follow-ups

- **Format-aware numeric parsing.** `ParseDefault` ignores
  `schemaFormat` today; per-bit-size integer parsing
  (`int32` via `ParseInt(_, 10, 32)`) and per-bit-size float
  parsing (`float32` via `ParseFloat(_, 32)`) are the
  obvious next refinements once a consumer surfaces the need.
- **Strict JSON for `object` / `array` defaults.** Invalid
  JSON on an `object`- or `array`-typed `default:` /
  `example:` currently falls back to the raw string. A strict
  mode could emit a diagnostic and drop the value rather
  than silently retain it.
- **Custom keyword rule extension.** `keywordTypeRules` is
  built per call so a `WithRules(...)` constructor (or a
  `RegisterKeyword` hook) could let downstream tools extend
  the legality table for vendor-extension keywords without
  patching this package.
