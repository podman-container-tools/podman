# routebody — maintainer notes

This document is the long-form companion to the `routebody` package
code. The source files keep godoc concise; the full sub-language
grammars, design trade-offs, and intentionally-deferred follow-ups
live here.

`routebody` parses the two raw sub-blocks carried inside
`swagger:route` / `swagger:operation` annotations — `Parameters:`
and `Responses:` — into typed declarations
(`ParamDecl`, `ResponseDecl`) plus a `grammar.Block` of validation
properties. The orchestrating builder (`builders/routes`) reads the
typed head fields directly and dispatches the Block through
`handlers.DispatchParamLevel0` / `handlers.DispatchSchemaLevel0`,
the same seam every other parameter and schema site uses.

---

## Table of contents

- [§parameters-grammar](#parameters-grammar) — the `Parameters:` chunk grammar
- [§responses-grammar](#responses-grammar) — the `Responses:` line grammar
- [§diagnostics](#diagnostics) — what routebody emits, and what it drops
- [§definition-fallback](#definition-fallback) — untagged response refs resolved against `definitions`
- [§quirks-open](#quirks-open) — deferred follow-ups

---

## <a id="parameters-grammar"></a>§parameters-grammar — the `Parameters:` chunk grammar

A `Parameters:` body is a sequence of chunks. Each chunk starts
with a line whose first non-whitespace character is `+` (canonical)
or `-` (alias, accepted for YAML-style authoring). Subsequent
indented lines carry the chunk's `key: value` pairs, one pair per
line. Lines without a colon are silently ignored.

Head fields (consumed directly by the orchestrator, NOT lowered
into Block properties):

| Key | Lands on | Notes |
|---|---|---|
| `name:` | `ParamDecl.Name` | |
| `in:` | `ParamDecl.In` | one of `path` / `query` / `header` / `body` / `formData`; `form` is accepted as an alias and normalised to `formData` |
| `type:` | `ParamDecl.TypeRef` | primitive name or Go identifier; `[]` prefixes accepted on body params |
| `format:` | `ParamDecl.Format` | |
| `description:` | `ParamDecl.Description` | |
| `required:` | `ParamDecl.Required` | parsed via `strconv.ParseBool` |
| `allowempty:` / `allowemptyvalue:` | `ParamDecl.AllowEmpty` | parsed via `strconv.ParseBool` |

Validation fields are lowered to `grammar.Property` entries on
`ParamDecl.Block` and dispatched via `handlers.DispatchParamLevel0`:

- `min` / `max` / `minimum` / `maximum` / `multipleOf`
- `minlength` / `maxlength` / `minitems` / `maxitems`
- `pattern`
- `unique`
- `collectionformat`
- `default` / `example` / `enum`

Unknown keywords (any key not in the head-field set and not in the
`grammar.Lookup` validation table) emit `CodeInvalidAnnotation` and
are dropped — they never reach the dispatcher silently.

A bare `+` / `-` sigil with no follow-up content (no `name:`, no
other head field, no validation property) is treated as an empty
chunk and dropped with `CodeInvalidAnnotation`. The minimum useful
chunk carries at least a `name:` and an `in:`.

## <a id="responses-grammar"></a>§responses-grammar — the `Responses:` line grammar

A `Responses:` body is one line per response. Each line has the
shape:

```
<code>: <token>*
```

where `<code>` is `default` (case-insensitive) or a decimal HTTP
status code. Tokens on the right of the colon are either
`tag:value` for `tag` in `{body, response, description}` or
untagged.

Tagged tokens:

- `body:Foo` — the response carries a body that references the
  model type `Foo`. `[]Foo` / `[][]Foo` wraps the body in N
  arrays (the leading `[]` prefixes are stripped and counted onto
  `ResponseDecl.Arrays`).
- `response:Foo` — the response is the named `swagger:response`
  `Foo`. Same array-prefix handling as `body:`.
- `description:Foo bar baz` — everything from the `description:`
  token through the rest of the line is the response's description
  prose. The token's own value (`Foo`) and any subsequent tokens
  are joined with single spaces. A bare `description:` token (no
  value after the colon) does not contribute a leading empty
  segment to the joined description.

Only one of `body:` / `response:` may appear on a single line; a
second occurrence drops the line with `CodeInvalidAnnotation`.

Untagged tokens:

- The first untagged token is the response ref candidate
  (resolved by the orchestrator — see §definition-fallback).
- Subsequent untagged tokens accumulate into the description.

A line whose first untagged token is literally `body` or
`response` (no colon) is treated as a typo for `body:Foo` /
`response:Foo` and dropped with `CodeInvalidAnnotation`. This
prevents the line from being silently parsed as
`response="body"` plus a description.

An empty-value line (`204:` with nothing after the colon)
produces a `ResponseDecl` with `Code` set and every other field
zero. The orchestrator emits the response with an explicitly
empty description.

## <a id="diagnostics"></a>§diagnostics — what routebody emits, and what it drops

routebody emits a single diagnostic code, `CodeInvalidAnnotation`,
on every recoverable parse error. `emitDiagf` is the shared
funnel — when the caller passes a nil `diag` sink, diagnostics
are dropped without affecting the parsed output. The dispatcher
seam accepts the same optional-sink posture, so a nil-sink call
flows through cleanly.

Positions on emitted diagnostics are line-offset from the
`basePos` the caller supplies (the source position of the
`parameters:` / `responses:` keyword head). routebody does not
track precise column information within the body — `Column` is
inherited from `basePos`.

Future diagnostic codes (shape mismatches, unresolved refs)
should land as sibling helpers in `diag.go` so call sites stay
explicit about which code they ride on.

## <a id="definition-fallback"></a>§definition-fallback — untagged refs and the definitions map

The first untagged token on a response line is reported as
`ResponseDecl.ResponseRef`. The orchestrator
(`builders/routes`) resolves the ref against the spec's
`responses` map first; on miss it falls back to the
`definitions` map and treats the hit as a body ref. The fallback
makes `200: User` work the same way whether `User` is a named
response or a model type — the author does not need to remember
the distinction.

Unresolvable refs (neither a `responses` entry nor a
`definitions` entry) emit `CodeInvalidAnnotation` at the
orchestrator level.

## <a id="quirks-open"></a>§quirks-open — deferred follow-ups

> **Where open quirks live.** This section documents caveats *of this package*.
> The project-wide register of what is actually open — verified, with the stale
> historical registers called out — is `.claude/plans/quirks-open.md`.

- **Column tracking** (owned by
  `.claude/plans/features/column-precision-unicode.md` — an LSP prerequisite,
  paired there with the multi-byte column question since both touch the same
  `Line.Pos` contract). routebody does not track per-line column
  information; diagnostics inherit `basePos.Column`. If the LSP
  integration needs per-token positions on body sub-language
  diagnostics, the body parser will need to track lex state more
  precisely.
- **`form` alias for `formData`.** The `form` spelling is
  accepted on `in:` and normalised to `formData`. The alias
  preserves a long-standing authoring convenience; a strict
  mode could emit a deprecation diagnostic in a future pass.
- **`collectionFormat:` lax acceptance.** Like the SimpleSchema
  dispatcher's collection-format handler, routebody leaves an
  unknown value typed as `ShapeNone` so the dispatcher's
  string-fallback path can round-trip the raw value onto the
  parameter. A future strict mode could reject values outside
  the OAS v2 vocabulary at parse time.
