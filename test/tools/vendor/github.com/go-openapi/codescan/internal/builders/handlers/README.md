# handlers — maintainer notes

This document is the long-form companion to the `handlers` package code.

The source files keep godoc concise; complex invariants, design trade-offs, and intentionally-deferred follow-ups live here.

The `handlers` package ships shared grammar Walker callbacks for the `SimpleSchema` family of OAS v2 dispatchers
(parameter level-0 and items chain, response-header level-0 and items chain) as well as the full-Schema dispatchers
used by the `schema` builder.

---

## Table of contents

- [§dispatch-surface](#dispatch-surface) — how SimpleSchema and full-Schema dispatch differ
- [§walker-payloads](#walker-payloads) — payload conventions per Walker callback
- [§raw-errsink](#raw-errsink) — the `errSink` contract on `Raw` and the parameter vs header posture
- [§collection-format-fallback](#collection-format-fallback) — why the `collectionFormat:` handler accepts arbitrary strings
- [§simple-schema-keywords](#simple-schema-keywords) — keyword allow-list under SimpleSchema mode and the `required:` carve-out
- [§extensions](#extensions) — how vendor extensions land via `AddExtension`
- [§stale-enum-desc](#stale-enum-desc) — why a field-level `enum:` strips an inherited `x-go-enum-desc`
- [§quirks-open](#quirks-open) — deferred follow-ups

---

## <a id="dispatch-surface"></a>§dispatch-surface — SimpleSchema vs full-Schema dispatch

The package exports two dispatcher families:

- **SimpleSchema** — `DispatchParamLevel0`, `DispatchHeaderLevel0`,
  `DispatchItemsLevel`. These fan a single switch-on-`Keyword.Name`
  out across consumers that write through an
  `ifaces.ValidationBuilder` or
  `ifaces.OperationValidationBuilder` adapter
  (`paramValidations`, `headerValidations`, `items.Validations`).
- **full-Schema** — `DispatchSchemaLevel0`, `DispatchSchemaItemsLevel`.
  These add a `checkShape` gate that emits `CodeShapeMismatch` on
  keyword-vs-resolved-type mismatches, and the Bool handler does
  cross-target writes (`required:` → `enclosing.Required` keyed by
  name, `discriminator:` likewise).

The shape-gate and cross-target writes are full-Schema-specific
concerns and don't share the SimpleSchema seam. The `SchemaOptions`
struct carries a `SimpleSchemaMode` flag that the full-Schema
dispatchers use to gate full-Schema-only keywords (`readOnly`,
`discriminator`) with `CodeUnsupportedInSimpleSchema` and to
silently skip `required:` (parameters handle `required:` at the
parameter level via SimpleSchema dispatch; headers don't carry
`required:` at all).

## <a id="walker-payloads"></a>§walker-payloads — Walker callback payload conventions

- **Number / Integer / Bool** callbacks fire with a zero-value
  payload when the lexer rejected the source value; the parser
  has already emitted a `CodeInvalid{Number,Integer,Boolean}`
  diagnostic. Consumers must gate on `pr.IsTyped()` before
  writing — the helpers in this package do that internally.
- **String** fires with the raw value alongside `pr.Value`; for
  `pattern:` consumers should read `pr.Value` (the regex source)
  rather than the formatted string, so the regex reaches
  `SetPattern` verbatim.
- **Raw** fires for `ShapeRawValue` keywords (`default:`,
  `example:`, `enum:`) and reads `pr.Value` for the raw text.

## <a id="raw-errsink"></a>§raw-errsink — `errSink` contract on `Raw`

`Raw` accepts an `errSink func(error) bool` argument that controls
how coercion errors from `default:` / `example:` propagate:

- `errSink == nil` → swallow silently. The response-header path
  uses this posture: a malformed default/example on a header does
  not fail the build. Both `DispatchHeaderLevel0` and
  `DispatchItemsLevel` wire `errSink=nil`.
- `errSink != nil` → invoked with the first
  `ParseValueFromSchema` error. Returning `true` short-circuits
  subsequent `Raw` callbacks within the same Walker invocation
  (the closure's `stopped` flag); returning `false` continues.

`DispatchParamLevel0` wires a sink that captures the first error
and returns `true`, so `DispatchParamLevel0` bubbles a malformed
parameter `default:` / `example:` up to the caller as a hard
failure. See `TestMalformed_DefaultInt` /
`TestMalformed_ExampleInt` in the integration suite for the
end-to-end behaviour.

## <a id="collection-format-fallback"></a>§collection-format-fallback — `collectionFormat:` accepts arbitrary strings

`CollectionFormatString` tries the Walker-supplied typed string
first. When that is empty (the grammar's closed-vocab string-enum
rejected the source), it falls back to
`strings.TrimSpace(pr.Value)` and writes the raw value through.

The OAS v2 spec defines a closed vocabulary
(`csv`/`ssv`/`tsv`/`pipes`/`multi`), but the codescan grammar is
intentionally permissive at this site: a typo such as `pipe`
instead of `pipes` round-trips verbatim onto the parameter or
items object. This preserves the source author's intent for
downstream tools that may surface validation errors against the
spec text directly.

SimpleSchema-only — the full-Schema `Validations` adapter does
not expose `SetCollectionFormat` because `collectionFormat:` is
not a full-Schema keyword.

## <a id="simple-schema-keywords"></a>§simple-schema-keywords — allow-list and `required:` carve-out

`simpleSchemaAllowed` in `keywords.go` enumerates the grammar
keyword names legal on an OAS v2 SimpleSchema site (parameter
with `in != body`, response header, and the items chain within
either). Source of truth: the OAS v2 Parameter Object and Header
Object allowed-keyword tables.

Vendor extensions (`x-*`) are not listed in the table — they are
gated by `classify.IsAllowedExtension`, which runs by name-prefix.

`required:` is included in the SimpleSchema allow-list because it
is valid on the parameter site (as a parameter-level boolean), but
it is NOT valid on response headers. Two consequences:

- The parameters walker writes `required:` to `param.Required`
  directly via `paramRequiredBool` — the value lands on the
  parameter object, not on a schema.
- The full-Schema walker (`schemaBoolHandler`) silently skips
  `required:` under `SimpleSchemaMode` because its full-Schema
  target is `enclosing.Required[name]` — the object-level
  required-array — which doesn't fit the SimpleSchema shape.

`IsSimpleSchemaKeyword` returns `false` for full-Schema-only
keywords (`readOnly`, `discriminator`, `$ref`, `allOf`, ...) and
for unknown names. Consumers wired in SimpleSchema mode use this
predicate to gate writes and emit
`CodeUnsupportedInSimpleSchema` diagnostics on miss.

## <a id="extensions"></a>§extensions — vendor extensions land via `AddExtension`

`ExtensionTarget` is the minimal surface a `Walker.Extension`
consumer needs to write a vendor extension. It is implemented by
every `oaispec` object that embeds `VendorExtensible` (`Schema`,
`Parameter`, `Header`, `Response`, `Operation`, ...) via the
`AddExtension` method promoted from the embed.

`Extension` returns a callback that filters non-`x-*` names via
`classify.IsAllowedExtension` and writes the typed extension
value onto the target.

User-authored extensions are not gated by the `SkipExtensions`
option — that flag suppresses scanner-derived `x-go-*` keys, not
author intent. Consumers that need additional side effects on a
successful write (e.g. the schema builder's
`refOverrideCollector` marking the collector) wrap this with
their own callback rather than reusing this helper.

## <a id="stale-enum-desc"></a>§stale-enum-desc — field-level `enum:` strips inherited `x-go-enum-desc`

`SchemaValidations.SetEnum` writes the parsed enum onto the
schema, then calls `clearStaleEnumDesc` to strip the
`x-go-enum-desc` extension (and the matching suffix from
`Description`) when present.

The extension is set by the type-level `swagger:enum TypeName`
pass and carries per-value documentation text. When a field-level
`enum:` annotation overrides the inherited values, the per-value
text describes values that are no longer in the field-level
enum — it is stale and must be dropped to avoid misleading
documentation.

## <a id="quirks-open"></a>§quirks-open — deferred follow-ups

- **`collectionFormat:` lax acceptance.** The fallback to the
  raw string preserves typos by design. A future strict-mode
  option could emit a diagnostic when the value is outside the
  OAS v2 closed vocabulary, leaving the lax default in place
  for compatibility.
- **`SchemaOptions.SimpleSchemaMode` on items dispatch.** The
  option is accepted on `DispatchSchemaItemsLevel` for
  symmetry but currently does not alter items-level behaviour.
  Worth revisiting if items dispatch ever needs the same
  gating as level-0 dispatch.
