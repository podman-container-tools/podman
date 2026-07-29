# godoclink — maintainer notes

This document is the long-form companion to the `godoclink` package.

`godoclink` implements the **`Options.CleanGoDoc`** feature: when a title /
description is carried **from a Go doc comment** into the emitted spec, it cleans
godoc-specific syntax that reads as bracket noise, and recomposes resolvable
doc-links to the name the referenced schema is actually **exposed under**.

It is applied **only to godoc-derived prose** — never to author-written
`swagger:title` / `swagger:description` overrides, which flow through a separate
path (`common.Builder.HarvestOverrides`) and are deliberate.

The recognizer regexes are adapted from
[`github.com/fredbi/go-fred-mcp/pkg/doc-filters/godoc-filter`]; the key
difference is that this package **rewrites** the prose, whereas that tool
**redacts** (length-preserving blanking) for masking.

---

## Table of contents

- [§transforms](#transforms) — what is cleaned, and the recognizer
- [§seam](#seam) — why recomposition is split across two build phases
- [§markers](#markers) — the marker format and the round-trip contract
- [§wiring](#wiring) — how builders call in, and the consumption sites
- [§deferred](#deferred) — intentionally-deferred follow-ups

---

## <a id="transforms"></a>§transforms — what is cleaned

With `CleanGoDoc` on, godoc-derived prose is run through `Clean`:

1. **Reference-definition lines dropped** — a line like `[text]: https://…`
   (optionally indented) is godoc/markdown link plumbing carrying no prose; the
   whole line is removed and the blank run it leaves is folded.
2. **Doc-link spans rewritten** — `[Widget]`, `[pkg.Type]`, `[Order.Field]`
   (a leading `*` tolerated). The brackets are stripped and the span replaced by
   either the referenced schema's exposed name (see [§seam](#seam)) or, when it
   does not resolve, the **humanized** leaf identifier
   (`mangling.NameMangler.ToHumanNameLower`, e.g. `[CustName]` → "cust name").
3. **Leading self-name recomposed** — a declaration's godoc conventionally opens
   with its own name (`// Widget does things`). With a `SelfRef`, that leading
   word is recomposed to the decl's own exposed name.
4. **Sentence-initial titleizing** — the first identifier of the prose is
   restored to sentence case (first rune upper), whatever the exposed name's
   actual case (`Widget`+`swagger:model gizmo` → "Gizmo …").

**Conservative recognizer.** `docLinkRE` matches only a dotted chain
(`[pkg.Type]`) or an uppercase-led single (`[Widget]`). Ordinary prose brackets
are left intact by construction: `[]byte` (empty), `[0]` (digit-led),
`[see notes]` (spaces), bare-lowercase `[id]` (a lowercase single never names an
exported schema in this phase).

## <a id="seam"></a>§seam — the two-phase split

Recomposition has two halves that want **opposite** timing:

- **Which prose is godoc-derived** is known only **here**, at the consumption
  seam — `block.PreambleTitle/PreambleDescription/Prose()` are godoc, while
  overrides arrive via `HarvestOverrides`. After the build, a `description` is
  just a string; provenance is gone. So cleaning must happen at consumption.
- **A referenced schema's final exposed name** is fixed only by the spec
  builder's `reduceDefinitionNames()`, which runs **last** (collision renames
  shorten the fully-qualified discovery keys). So the final name is unknown at
  consumption.

The bridge: at consumption a resolvable doc-link is replaced by a **marker**
carrying the referenced type's fully-qualified definition key; a post-reduce
pass (`spec.Builder.substituteGodocMarkers` → `SubstituteMarkers`) rewrites each
marker to the final name. This mirrors the existing
`defOrigins → FlushDefOrigins(finalName)` pattern in the scanner: buffer keyed
by fq-key during the build, re-point to final names after reduce.

## <a id="markers"></a>§markers — format and round-trip

A marker is NUL / Unit-Separator delimited — neither rune can occur in a Go
source comment, so a marker never collides with real prose:

```
\x00gl\x1f<defKey>\x1f<suffix>\x1f<fallback>\x1f<0|1>\x00
```

- `defKey` — the referenced type's fully-qualified definition key (the same key
  `EntityDecl.DefKey()` produces, so `swagger:model` overrides are honored).
- `suffix` — the exposed field-chain for a member reference (`.customer_name`),
  or empty for a bare type.
- `fallback` — the humanized leaf, used when the key turns out **not** to be an
  emitted definition (pruned / unresolved).
- titleize bit — sentence-initial position.

`SubstituteMarkers(text, finalName)` resolves each marker: `finalName+suffix`
when `finalName(defKey)` succeeds, else `fallback`; the titleize bit upper-cases
the first rune. It **guarantees no marker survives** — an unmatched marker
collapses to its fallback. With `CleanGoDoc` off no marker is ever produced, and
`SubstituteMarkers` short-circuits on marker-free text (`HasMarkers`).

The round-trip (emit via `Clean` with a `Resolver` → `SubstituteMarkers`,
including the pruned-key fallback and a collision rename) is unit-tested in
`markers_test.go`.

## <a id="wiring"></a>§wiring — how builders call in

`Clean(text, Options{Mangler, Resolver, Self})`. The two callers are on the
builder side, gated by `Ctx.CleanGoDoc()`:

- `common.Builder.CleanGoDoc` / `CleanGoDocSelf` (the latter passes a `Self`,
  used for a declaration's title/description; the former for field / member
  prose). `common.Builder.godocResolver` builds the `Resolver` from the active
  `EntityDecl` (reusing `ScanCtx.GetModel`; same-package + imported lookup;
  field → exposed property name via `resolvers.ParseFieldTag` + `NameFromTags`).
- `spec.Builder.cleanGoDoc` — a sibling for the `swagger:meta` Info site (the
  spec builder does not embed `common.Builder`); resolution-free there (info
  prose rarely names models).

The nine godoc-prose consumption sites it is wired at: `swagger:meta` Info
title/description; route + inline operation summary/description; response and
response-header description; parameter description; model title/description;
field description (plain and `$ref`-override paths).

A nil `Resolver` (or nil `Self`) selects resolution-free cleanup — the behavior
for sites without a usable decl context.

## <a id="deferred"></a>§deferred — follow-ups

- **Field-level leading self-name.** Only a declaration's own leading name is
  recomposed; a field's leading Go name (`// Holder …`) is left as-is.
- **Nested member chains.** `[Type.A.B]` resolves only the first member level;
  deeper chains fall back to humanizing the leaf.
- **Dot-imports.** A `.`-imported package is skipped in import resolution, so a
  `[Type]` actually referring to a dot-imported schema is humanized.

None of these is wrong today — each falls back to the humanized leaf.

[`github.com/fredbi/go-fred-mcp/pkg/doc-filters/godoc-filter`]: https://github.com/fredbi/go-fred-mcp
