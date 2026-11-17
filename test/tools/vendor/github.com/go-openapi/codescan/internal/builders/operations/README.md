# `internal/builders/operations` — maintainers' guide

Builds OAS v2 operation entries for `swagger:operation` annotations
— Summary, Description, and the YAML body content. One `Builder`
per annotation; one grammar parse per operation.

## Sections

- [§overview](#overview) — package shape and per-file responsibilities
- [§builder](#builder) — `Builder`, `Build`, the orchestrator entry
- [§path-operation-slot](#path-operation-slot) — `setPathOperation` reuse semantics
- [§walker](#walker) — `applyBlockToOperation` and the YAML body contract
- [§quirks-open](#quirks-open) — deferred follow-ups

---

## <a id="overview"></a>§overview — files and responsibilities

| File | Contents |
|------|----------|
| `operations.go` | `Builder` (embeds `*common.Builder`), `NewBuilder`, top-level `Build`, `setPathOperation` slot helper |
| `walker.go` | `applyBlockToOperation` — grammar Block → operation Summary/Description/YAML body |
| `errors.go` | `ErrOperations` sentinel |

The builder embeds `*common.Builder` (Ctx, ParseBlocks cache,
diagnostic sink). `Decl` is nil — operations build off a path
annotation, not a declaration; `MakeRef` / Decl-anchored helpers
must not be called.

Per-keyword body parsing for routes (`schemes`, `consumes`,
`security`, ...) lives in the `routes` package. The operations
package only handles the operation header (Summary/Description) and
the YAML body for `swagger:operation`.

## <a id="builder"></a>§builder — top-level `Build`

`Build(tgt *spec.Paths)` resolves the path-item slot on `tgt`,
allocates or reuses the operation for the HTTP verb via
`setPathOperation`, attaches the operation's `Tags`, then dispatches
into `applyBlockToOperation` for the header content and the YAML body.

The path-item slot may be missing on `tgt` (`tgt.Paths == nil`); the
builder lazily initialises the map before writing the result back.

## <a id="path-operation-slot"></a>§path-operation-slot — `setPathOperation` reuse

`setPathOperation` lands an `*oaispec.Operation` on the HTTP-verb
slot of a `*oaispec.PathItem`. When the slot already holds an
operation with the same ID, the existing one is kept — so a
partially-built operation accumulates work across the scanner's two
passes (route discovery then operation discovery). Otherwise the
incoming operation replaces what was there.

When the incoming operation is nil, a fresh operation is allocated with the given ID.

Unrecognised methods leave the path item untouched and return the incoming operation verbatim.

The public re-export `SetPathOperation` exists for consumers in
sibling packages (the `routes` builder uses it to allocate or reuse
the same operation slot from the route side).

## <a id="walker"></a>§walker — `applyBlockToOperation`

`applyBlockToOperation` parses `path.Remaining` through the grammar
parser and writes Summary / Description / YAML body content onto
the operation.

The grammar lexer already classifies prose into `TokenTitle` /
`TokenDesc` and isolates `---` fenced bodies into `TokenOpaqueYaml`,
so the bridge collapses to three direct reads:

1. `op.Summary` ← `block.Title()` — first title paragraph.
2. `op.Description` ← `block.Description()` — remaining prose.
3. The first body from `block.YAMLBlocks()` is fed through
   `yaml.UnmarshalBody` → `op.UnmarshalJSON`. Exactly one fenced
   body is consumed per operation; subsequent bodies are ignored.

   Parameters already bound to the operation by a `swagger:parameters`
   struct (placed there by `buildParameters`, which runs before
   `buildOperations`) are snapshotted and the slice cleared before the
   unmarshal, then merged back via `mergeBoundParameters`. Without this,
   `encoding/json` reuses the existing slice elements when decoding the
   inline `parameters:` array and welds a bound body's `$ref` onto an
   inline parameter (go-swagger#2651). Merge-back skips a bound
   parameter that collides with an inline one — same `(name, in)`, or
   both the singleton body parameter — so inline declarations win.

`path.Remaining` is the `*ast.CommentGroup` AFTER the
`swagger:operation` header line has been stripped by
`parsers.ParseOperationPathAnnotation`, so the grammar sees it as an
`UnboundBlock` whose `Title` / `Description` / `YAMLBlocks` all
behave identically to a properly-anchored block.

## <a id="quirks-open"></a>§quirks-open — deferred follow-ups

- **single fenced body per operation.** The walker consumes the
  first `YAMLBlocks` entry only. Multiple `---` blocks in a single
  `swagger:operation` annotation are silently dropped beyond the
  first; a future strict-mode option could emit a diagnostic.
