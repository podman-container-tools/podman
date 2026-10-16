# yaml sub-parser — maintainer notes

This document is the long-form companion to the `yaml` sub-parser
code. The source files keep godoc concise; complex invariants, design
trade-offs, and intentionally-deferred follow-ups live here.

`internal/parsers/yaml/` is a thin wrapper around `go.yaml.in/yaml/v3`
that consumes the `RawYAML` bodies isolated by
`internal/parsers/grammar/` between `---` fences, plus the
typed-extensions service the grammar lexer calls for `extensions:`
raw blocks.

---

## Table of contents

- [§importers](#importers) — who calls in and the grammar carve-out
- [§typed-extensions](#typed-extensions) — `TypedExtensions` contract
  and the YAML → JSON normalisation rationale
- [§unmarshal-body](#unmarshal-body) — godoc → YAML → JSON pipeline
  for operation / meta bodies
- [§dedent](#dedent) — leading-indent normalisation, first-line vs
  common-prefix strategies, recognised whitespace tokens
- [§sibling-sub-parsers](#sibling-sub-parsers) — the
  `internal/parsers/<name>/` seam this subpackage establishes
- [§quirks-open](#quirks-open) — deferred follow-ups

---

## <a id="importers"></a>§importers — who calls in

Two importer surfaces:

- **The builder layer** — bridge taggers that decide when to parse a
  given `RawYAML` body (the `operations` bridge for
  `swagger:operation` YAML, the `meta` bridge for
  `securityDefinitions`, `infoExtensions`, and `extensions` raw
  blocks).
- **`internal/parsers/grammar/`** — calls `TypedExtensions` from its
  extensions raw-block lexer so `Extension.Value` ships typed.

The grammar import is the one carve-out from the "grammar stays
YAML-free" architecture rule. Every other parser-layer module owes
zero dependencies on a YAML decoder; the carve-out is scoped to the
`extensions:` raw block because the alternative is shipping
stringly-typed extension values to every consumer and re-parsing
downstream.

## <a id="typed-extensions"></a>§typed-extensions — `TypedExtensions` contract

`TypedExtensions(body)` parses the body of an `extensions:` raw block
and returns its top-level entries as JSON-typed values
(`bool` / `float64` / `string` / `[]any` / `map[string]any`).

Two shapes are supported uniformly:

- **Flat scalar form** —
  ```
  extensions:
    x-tag: foo
    x-priority: 5
  ```
- **Nested / typed YAML form** —
  ```
  extensions:
    x-config:
      enabled: true
      threshold: 0.5
      tags: [a, b, c]
  ```

### Why YAML → JSON normalisation

`yaml.v3` yields `map[any]any` for nested mappings; downstream
consumers (vendor-extension targets, code generators, the spec
types' `AddExtension` surface) all expect `map[string]any` with
concrete leaf types. JSON unmarshalling is the cheapest way to
enforce that shape — the round-trip through
`swag/yamlutils.YAMLToJSON` is the canonical normalisation step.

### Why the body is dedented

The grammar lexer preserves each line's original whitespace prefix
(it needs godoc-level indentation to survive for nested YAML to
remain structurally valid). YAML in turn refuses tab indentation
and treats leading whitespace as structural. The dedent therefore
lives downstream of the lexer: strip the common leading-whitespace
prefix shared by every non-blank line, then substitute any residual
leading tabs with two spaces. Both petstore's `\t`-indented
Extensions block and the typed-nested test case using `  `
indentation parse identically through this pipeline.

### No name filtering at the wrapper

The wrapper applies no `x-*` filtering. Each consumer decides
whether to accept only `x-*` keys (via
`classify.IsAllowedExtension`) or to consume the full mapping. The
schema builder's call site applies the filter; the grammar lexer's
call site leaves it to the eventual Walker.Extension consumer.

### Error model

A malformed YAML body propagates as a wrapped `fmt.Errorf("yaml: %w")`
error. The grammar layer surfaces the failure as a
`CodeInvalidYAMLExtensions` diagnostic rather than a silent drop.
Empty body returns `(nil, nil)`.

## <a id="unmarshal-body"></a>§unmarshal-body — godoc → YAML → JSON for operation / meta bodies

`UnmarshalBody(body, unmarshal)` runs a raw godoc-comment YAML body
through the standard pipeline expected by every Swagger target that
consumes JSON-shape input:

1. `RemoveIndent` — strip the common indent godoc adds to every line
   and turn leading tabs into two-space sequences.
2. `yaml.Unmarshal` into a generic `map[any]any`.
3. `yamlutils.YAMLToJSON` — coerce the `map[any]any` soup into
   JSON-shaped values with concrete leaf types.
4. Hand the resulting JSON bytes to the caller's callback, typically
   a `*spec.<Target>.UnmarshalJSON` or a `json.Unmarshal` into a
   caller-provided struct.

Empty body returns nil — the caller's target is left untouched.

Used by the operations bridge (`swagger:operation` YAML body), the
meta bridge (`securityDefinitions`, `infoExtensions`, `extensions`
raw blocks), and any future target that needs the same shape.

## <a id="dedent"></a>§dedent — leading-indent normalisation

Two dedent strategies coexist in this package, chosen per call site:

- **`RemoveIndent` (operation/meta path)** — expand-then-first-line
  dedent. Pass 1 expands the leading tabs of every non-blank line to
  two spaces (so tab- and space-indented lines are comparable); pass 2
  strips the first non-blank line's indent length from every line. The
  first-line strip width is preserved because the existing operation
  goldens depend on it. The up-front tab expansion (vs the older
  retab-only-the-post-strip-remainder) preserves the nesting of a
  gofmt-canonical swagger:operation body — 1-space prose keys
  interleaved with tab-prefixed value blocks; see `dedent.go`.
- **`normaliseExtensionBody` (typed-extensions path)** — common-prefix
  dedent. Strips the longest leading-whitespace run shared by every
  non-blank line. Required because extension bodies arrive with the
  full godoc indent preserved on every line (the lexer keeps `Token.Raw`
  for `yamlBody` blocks instead of `Token.Text`).

Both passes then call `retabLeading` / `replaceLeadingTabs` to
substitute residual leading tabs with two spaces — YAML refuses tab
indentation.

### Recognised whitespace tokens

`leadingIndent` recognises:

- space (` `)
- tab (`\t`)
- leading `/` characters that survive when the lexer hasn't stripped
  a godoc comment marker yet (`//`, `///`).

Unicode space separators (`\p{Zs}`) are not recognised: real Go
source uses ASCII whitespace. If a corpus surfaces that depends on
Unicode whitespace, reintroduce the branch in `isIndentSpace`.

## <a id="sibling-sub-parsers"></a>§sibling-sub-parsers — the `internal/parsers/<name>/` seam

This subpackage establishes a pattern: any future sub-language (enum
variant forms, richer example syntax, private-comment bodies, …)
gets its own `internal/parsers/<name>/` subpackage following the
same seam — narrow public surface, no transitive dependency from
`internal/parsers/grammar/` unless it is a deliberate carve-out
(documented in the importer's godoc).

## <a id="quirks-open"></a>§quirks-open — deferred follow-ups

- **Per-entry positions on extensions.** Today every Extension in a
  block shares the same `Pos` (the `extensions:` keyword's). LSP-grade
  per-entry positions ("`x-foo` at line 47 has malformed value")
  require decoding into `*yaml.Node`, walking the top level, and
  translating `node.Line` / `node.Column` (1-indexed relative to
  body) into absolute `token.Position`. Pick this up when LSP per-
  entry extension diagnostics become a real requirement.
- **Unicode whitespace in `leadingIndent`.** Reintroduce the
  `\p{Zs}` branch in `isIndentSpace` if a real corpus surfaces that
  depends on it.
