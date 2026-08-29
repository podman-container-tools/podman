# grammar — maintainer notes

This document is the long-form companion to the grammar package code.

The source files keep godoc concise; the full grammar contract,
pipeline rules, keyword tables, body-termination rules, and quirks
live here.

---

## Table of contents

- [§overview](#overview) — what the package parses and what it emits
- [§pipeline](#pipeline) — Preprocess → Lex → Parse stages
- [§preprocess-contract](#preprocess-contract) — comment-marker stripping rules
- [§lexer-contract](#lexer-contract) — line classifier, body accumulator, prose classifier
- [§prose-classification](#prose-classification) — TITLE / DESC split heuristics
- [§raw-block-terminators](#raw-block-terminators) — sibling-terminator rule for raw bodies
- [§yaml-fence-handling](#yaml-fence-handling) — opaque YAML bodies and decorative fences
- [§literal-description](#literal-description) — `swagger:description \|` verbatim markdown body
- [§disambiguation](#disambiguation) — value-shape dispatch (default / enum / type-ref)
- [§parser-contract](#parser-contract) — Block family dispatch, AnnotationKind
- [§block-shapes](#block-shapes) — typed Block kinds and their fields
- [§property-shape](#property-shape) — Property, TypedValue, IsTyped
- [§walker-contract](#walker-contract) — functional-visitor dispatch table
- [§keyword-table](#keyword-table) — closed-vocabulary keyword classification
- [§context-legality](#context-legality) — per-annotation keyword legality
- [§annotation-args](#annotation-args) — per-annotation argument terminals
- [§typed-extensions](#typed-extensions) — `extensions:` body → typed map
- [§security-requirements](#security-requirements) — typed Requirements parsing
- [§contact-license](#contact-license) — typed Contact / License accessors
- [§diagnostics](#diagnostics) — Diagnostic / Code / Severity model
- [§synthetic-block](#synthetic-block) — sub-parser construction factory
- [§quirks-open](#quirks-open) — deferred follow-ups

---

## <a id="overview"></a>§overview — what the package parses

`grammar` is the annotation parser for codescan. It consumes one
Go comment group (`*ast.CommentGroup`) at a time and produces a
typed `Block` carrying:

- the recognised annotation (`swagger:model`, `swagger:route`, …) as
  an `AnnotationKind`;
- per-Block fields for the annotation's positional arguments (e.g.
  `RouteBlock.Method`, `ParametersBlock.Target` / `.Args`);
- `Property` entries for every recognised body keyword (`maximum:`,
  `pattern:`, `consumes:`, …) carrying the keyword's lexer-typed
  value or raw body bytes;
- prose lines split into `Title()` / `Description()` using
  line-shape heuristics;
- diagnostics for malformed inputs (the parser never aborts — it
  emits warnings/errors and continues).

Comment groups without a `swagger:<name>` line surface as an
`UnboundBlock` so the schema builder can still consume the prose
and any field-level annotations.

The annotation vocabulary is the go-swagger convention:
`swagger:model`, `swagger:response`, `swagger:parameters`,
`swagger:route`, `swagger:operation`, `swagger:meta`, plus the
classifier annotations `swagger:strfmt`, `swagger:alias`,
`swagger:name`, `swagger:allOf`, `swagger:enum`, `swagger:ignore`,
`swagger:default`, `swagger:type`, `swagger:file`,
`swagger:additionalProperties`.

`AnnotationPrefix` is the literal `"swagger:"`. It is a constant
rather than configurable today.

---

## <a id="pipeline"></a>§pipeline — Preprocess → Lex → Parse

```
*ast.CommentGroup
     │
     ▼
  Preprocess  → []Line       (comment-marker stripping)
     │
     ▼
     Lex      → []Token      (line classifier + body accumulator + prose classifier)
     │
     ▼
    Parse     → Block        (dispatch by annotation family)
```

Three stages, each a pure function:

1. **Preprocess** strips comment markers and normalises line endings
   ([§preprocess-contract](#preprocess-contract)).
2. **Lex** runs three sub-stages — line classifier, body accumulator,
   prose classifier — producing the terminal token stream
   ([§lexer-contract](#lexer-contract)).
3. **Parse** dispatches the stream to a family-specific parser per
   the recognised `AnnotationKind`
   ([§parser-contract](#parser-contract)).

The Token vocabulary is defined in `token.go` and matches the
documented terminal alphabet for the four annotation families
(schema / operation / meta / classifier) plus the family-shared
keyword vocabulary.

---

## <a id="preprocess-contract"></a>§preprocess-contract — comment-marker stripping

`Preprocess` turns a `*ast.CommentGroup` into a position-tagged
`[]Line`. Per line:

- `Text` has comment markers (`//`, `/*`, `*/`) stripped along with
  godoc continuation decoration (leading whitespace, asterisks,
  slashes, optional markdown table pipe). This is the surface the
  keyword/annotation classifiers consume.
- `Raw` is the same source line with **only** the comment marker
  removed — content whitespace, indentation, and list markers
  survive. The body accumulator reads `Raw` for YAML / nested-map
  indentation fidelity.
- `Pos` points to the first character of `Text` in the source file
  so diagnostics can pinpoint the offending line.

Line endings are normalised before line splitting (`\r\n → \n`,
lone `\r → \n`) so the lexer never sees `\r`.

The `/* */` block-comment form yields one Line per physical source
line; the godoc continuation decoration (`\s*\*\s?`) is stripped
from each line. That strip runs **before** the content-prefix trim
(`stripLine` applies the comment-kind raw-strip first), so the only
leading `*` the content trim can see is a markdown list bullet, never
block-comment decoration.

Leading `-` is preserved on Text so the YAML fence `---` survives
intact.

### Markdown bullet normalisation (go-swagger#1726)

A leading markdown list bullet — `* item` or `+ item` — is normalised
on `Text` to the canonical YAML form `- item`. Doing it here, once, in
the shared preprocess step means every downstream consumer that already
understands `- ` treats markdown-style and YAML-style lists identically:
prose descriptions, `Property.AsList` (consumes / produces / schemes /
tags), and enum bodies. The marker must be followed by a space (a
CommonMark bullet), so `*emphasis*` and `**bold**` prose are untouched.

gofmt performs the **same** `*`/`+` → `-` rewrite on `//` doc-comment
bullets, so this normalisation only changes the result for source that
has not been gofmt'd; gofmt-canonical source already arrives in the dash
form. The two agree by construction. (`Raw` is left untouched, so YAML
bodies — which are strict YAML and use `-` sequences — are unaffected.)

---

## <a id="lexer-contract"></a>§lexer-contract — token production

`Lex` runs three stages:

1. **Line classifier** (`classifyLines` / `lexLine`). Pure function
   on a single line plus an in-fence flag carried between lines.
   Emits one preliminary token per line — `TokenBlank`,
   `tokenYAMLFence`, `tokenRawLine` (inside an active fence),
   `TokenAnnotation`, `tokenKeywordPre`, or `tokenText`.

2. **Body accumulator** (`accumulateBodies`). State machine over
   the line stream, folding multi-line bodies (`OPAQUE_YAML`,
   `RAW_BLOCK_*`, `RAW_VALUE_*`) into single body tokens and
   finalising inline-value keywords with their typed payload. The
   output stream contains only tokens the parser actually consumes;
   internal kinds (`tokenYAMLFence`, `tokenText`, `tokenKeywordPre`,
   `tokenRawLine`, `tokenDirective`) are stripped here.

3. **Prose classifier** (`classifyProse`). Re-types surviving
   `tokenText` tokens as `TokenTitle` or `TokenDesc` per the
   line-shape heuristics in [§prose-classification](#prose-classification).

The output stream terminates with a single `TokenEOF`.

### Recognised annotation prefix

`hasSwaggerPrefix` matches `AnnotationPrefix` with **only the first
character** case-permissive (`[Ss]wagger:`). The rest of the prefix
is verbatim, matching authorial convention.

### Godoc-prefix exception for `swagger:route`

A line of the form `<GoIdent><WS>swagger:route <args>` has the
leading `<GoIdent><WS>` stripped before annotation lexing.
`matchGodocRoutePrefix` implements the check; only `swagger:route`
is granted this exception (the form is a long-standing godoc
convenience — a function or constant identifier on the same line
as the annotation). Other annotation names do not get the
heuristic.

### Go directives are dropped

Lines like `//go:generate`, `//nolint:foo`, `//lint:ignore` are
recognised by `isGoDirective` (lowercase-word + `:` + immediate
non-whitespace argument, no leading whitespace) and dropped from
the prose surface so they never land in TITLE / DESC. The
swagger-prefix check runs first, so `//swagger:model` (legal but
non-idiomatic, no leading space) is not mistaken for a directive.

### Kubernetes marker comments are dropped

Lines whose content begins with `+` followed by a letter —
`+kubebuilder:…`, `+genclient`, `+k8s:…`, `+optional`, the marker
convention emitted by kubebuilder / controller-gen / k8s
code-generation — are recognised by `isDirectiveMarker` and dropped
from the prose surface, so they never leak into model / property
descriptions (go-swagger#2687, the residual of #3007). Requiring a
letter after the `+` keeps ordinary prose (`+1 for …`, markdown `+`
bullets) intact.

Unlike Go directives, this drop happens at **Stage 3**
(`classifyProse`), not at line classification. The inline
`swagger:route` parameters grammar uses `+name:` as a parameter
separator (go-swagger#3100), which matches the marker shape; running
the filter after `accumulateBodies` has folded the route body into
its keyword token means the separator is never seen by the marker
check. Only loose prose `tokenText` lines are filtered.

### First-character case insensitivity on keywords

`Consumes:` and `consumes:` both lex as the `consumes` keyword.
Only the leading character is lowercased before `Lookup(name)`;
the rest of the keyword name is matched verbatim. So `Consumes` →
`consumes` matches, but `CONSUMES` does not.

### `items.` prefix runs

Repeated `items.` segments before a keyword (e.g.
`items.items.maxLength:`) are stripped and counted; `ItemsDepth`
records the depth on the emitted keyword token. Bare `items:`
(no separator) is not stripped — it is a legitimate keyword on its
own (the items chain head). `stripItemsPrefix` implements the
peel.

### Trailing-dot elision

After extracting `Value` (Keyword) or `Args` (Annotation), a
single trailing `.` is stripped. Source preservation lives in
`Raw`. The rule allows authors to write keyword and annotation
lines as English sentences without leaking the period into the
parsed value.

---

## <a id="prose-classification"></a>§prose-classification — TITLE / DESC split

`classifyProse` re-types `tokenText` tokens as `TokenTitle` or
`TokenDesc` using four heuristics applied to the first contiguous
prose run:

1. **Blank inside the run splits title/desc.** A blank line inside
   the run with text after it ends the title and starts the
   description.
2. **First line ends with Unicode punctuation** (`\p{Po}`) — first
   line is title, the rest is description.
3. **First line is a markdown ATX heading** (`#`+ followed by
   whitespace) — strip the marker, first line is title, the rest
   is description.
4. **Otherwise the entire run is description**, title empty.

Later prose runs (after body / keyword tokens) become description
unconditionally.

Heuristic 1 only fires when there is text after the blank —
trailing blanks are separators between the prose run and the
following non-prose token (annotation / EOF), not an internal
title/desc divide. On a heuristic-1 split, an ATX marker on the
first title line is also stripped so the rendered title doesn't
carry the `#`+ prefix.

The classification fires regardless of whether the block carries
an annotation — `UnboundBlock`-style comments (struct-field
docstrings) still need title/description so the schema builder
can consume them when the type is reached indirectly (e.g. an
embedded interface in a `swagger:model` parent).

The state-machine in `classifyTitleDescRun` walks the run once
and re-types text tokens; blanks stay as `TokenBlank` so consumers
can reproduce paragraph structure.

### `WithSingleLineCommentAsDescription` — demote a lone title

`NewParser(fset, WithSingleLineCommentAsDescription(true))` (driven by
`Options.SingleLineCommentAsDescription`, go-swagger#2626) overrides
heuristic 2/3 for the **single-line** case only: after
`extractTitleDesc`, `finaliseBase` calls `demoteSingleLineTitle`, which
moves a one-line title (non-empty title, empty description, no embedded
newline) into the description. A multi-line comment — anything that
already produced a description, including a heuristic-1 blank split — is
left untouched. The demotion is applied to both the full
title/description and the preamble pair, so the schema builder's
`PreambleTitle` / `PreambleDescription` path observes it too.

---

## <a id="raw-block-terminators"></a>§raw-block-terminators — sibling-terminator rule

Multi-line keyword bodies (`consumes:`, `produces:`, `security:`,
`responses:`, `parameters:`, `extensions:`, `default:`,
`example:`, `enum:`, …) end at the next **sibling structural
item** or EOF. Blank lines never terminate a body — they are
absorbed verbatim into the body content.

A "sibling structural item" is any of:

- another `TokenAnnotation`;
- a `tokenKeywordPre` whose canonical name shares a family
  context with the opening keyword
  ([§context-legality](#context-legality));
- a `tokenYAMLFence` outside an extensions body (the extensions
  body absorbs decorative fences silently — see
  [§yaml-fence-handling](#yaml-fence-handling)).

The family-overlap rule lives in `isSiblingTerminatorFor` and
`familyOf`:

- bodies opened under a **meta/route/operation context** keyword
  (`consumes`, `produces`, `security`, `securityDefinitions`,
  `responses`, `parameters`, `extensions`, `externalDocs`,
  `infoExtensions`, `tos`, `schemes`) terminate on any sibling
  whose family overlaps with the meta/route/operation set;
- bodies opened under a **schema-context** body keyword
  (`default`, `example`, `enum`) terminate on any sibling whose
  family overlaps with the schema set.

A keyword whose name is recognisable but whose family does not
overlap is absorbed as body text — this matches the permissive
shape of nested YAML-like content under e.g. `security:`.

**Indentation override (YAML-bodied blocks only).** Inside a
YAML-bodied block (`extensions`, `infoExtensions`,
`securityDefinitions`, `tags`, `security`), a same-family keyword
indented *strictly deeper* than the block head is treated as a
nested YAML key, not a sibling — e.g. `externalDocs:` under a
`Tags:` list item, where both are meta-family. Such a key is
absorbed so the nested YAML structure survives. Flat raw blocks
(`tos`, `consumes`, …) do **not** apply this: their keyword
indentation is purely
cosmetic (the petstore meta indents `Schemes:`/`Host:` deeper than
a column-0 `Terms Of Service:`, yet they are siblings), so any
sibling-family keyword terminates them regardless of depth.
Indentation is measured from the `Raw` view via
`leadingIndentWidth` (tabs expand to 8-column stops).

### Inline-value capture on raw-block heads

`Consumes: application/json` on a single line carries its value
on the head token (`head.Text`). `collectRawBlock` prepends that
value as the first body line so mixed inline-plus-indented forms
work uniformly. Without this prepend, the post-colon payload
would be silently lost.

### Per-body indentation handling

- `extensions:`, `infoExtensions:`, `securityDefinitions:`,
  `tags:` and `security:` bodies are YAML-parsed downstream
  (`yaml.TypedExtensions` or `yaml.UnmarshalBody`/`UnmarshalListBody`
  via the meta walker, or `security.Parse`), so every body line
  preserves its original indentation — `collectRawBlock` reads the
  `Raw` view (right-trimmed only). The `tags:` body in particular is
  a sequence of mappings whose nesting collapses if per-line indent
  is dropped; a `security:` block with block-style scopes needs the
  same.
- Flat raw blocks (`consumes:`, `produces:`, …) use the `Text`
  view (leading whitespace dropped, keyword lines reformatted via
  `formatKeywordLine`).

Both branches converge on the same `bodyText` slice.

### Single-line raw-value path

`collectRawValue` has a trivial single-line path: when the head
token carries an inline value, one `TokenRawValueBody` is emitted
immediately with the inline value as the whole body. The
multi-line path is reserved for the block-head case (head with
empty inline value).

---

## <a id="yaml-fence-handling"></a>§yaml-fence-handling — opaque YAML bodies

`collectFencedYAML` scans from a `---` opener and emits one
`TokenOpaqueYaml`:

- the body is joined with `\n` into `Body`;
- `Raw` carries the verbatim content (indentation preserved);
- `Truncated = true` is set when EOF is reached without a closing
  `---` — `parser.consumeBodyToken` then emits a
  `CodeUnterminatedYAML` error diagnostic.

Decorative `---` fences inside an `extensions:` body are
**dropped silently** — authors decorate extensions blocks with
fences as a visual separator; the lexer absorbs the fence into
the body and discards the fence markers themselves via
`absorbDecorativeFenceInto`.

`swagger:route` does not allow `OPAQUE_YAML` bodies — only
`swagger:operation` does. The parser flags an OPAQUE_YAML under
route with `CodeUnexpectedToken`.

---

## <a id="literal-description"></a>§literal-description — `swagger:description |` verbatim markdown body

By default a multi-line `swagger:description` folds its body with the
**Option B** rule (`collectDescriptionBody`): contiguous prose lines up
to the first blank line, keyword, or annotation, each `TrimSpace`d. That
is fine for plain prose but loses markdown — blank lines terminate it and
indentation / table pipes are stripped.

A YAML **literal block-scalar marker** opts the body into verbatim
capture. When the annotation line ends with a lone `|`
(`isLiteralDescMarker`), everything below is taken exactly — blank lines,
indentation, table pipes, and `---` all preserved — until the next
annotation or EOF:

```
// swagger:description |
// Widgets support **markdown**:
//
// | name | purpose |
// |------|---------|
//
// - a bullet
// swagger:model Widget
```

The capture is implemented in **two stages**, and stage 1 is load-bearing:

- **Stage 1 (`classifyLines`, `inLiteralDesc`).** On emitting a
  `swagger:description |` annotation, the line classifier enters literal
  mode and emits every following line as a verbatim `tokenRawLine` — **no
  `---` fence toggling, no blank-line or keyword termination**. This must
  happen here: a lone `---` in the body would otherwise flip the global
  `inFence` state ([§yaml-fence-handling](#yaml-fence-handling)) and
  swallow a following annotation as opaque YAML. Literal mode ends when a
  line classifies as an annotation (re-processed normally, so a following
  `swagger:model` survives) or at EOF.
- **Stage 2 (`collectDescriptionLiteral`).** Folds the `tokenRawLine` run
  into the annotation's single `TokenRawValue` arg: joins with `\n`, drops
  the `|` marker, clips trailing blank lines (bare-`|` clip), and strips
  the single godoc `// ` convention space per line. Interior indentation
  and trailing whitespace (markdown hard breaks) are kept.

Because the body becomes the annotation's `AnnotationArg()`, every
`swagger:description` consumer (schema model / field description) gets the
verbatim markdown with no per-builder change.

**Terminator contract.** Only an annotation at the **start of a line**
ends the block; a `swagger:` token mid-line is prose. The comment-prefix
strip removes leading indentation *before* the annotation check, so an
indented line that begins with `swagger:` still terminates — a
`swagger:`-leading line cannot be hidden inside an indented markdown code
block. Plain `swagger:description` (no `|`) keeps Option B unchanged.

This reframes go-swagger#3211: markdown is authored explicitly via
`swagger:description |`, never recovered from ambient godoc prose (the
preamble path still strips the leading table pipe — see
[§preprocess-contract](#preprocess-contract)).

---

## <a id="disambiguation"></a>§disambiguation — value-shape dispatch

`disambiguate.go` centralises the value-shape rules so the lexer
emits already-disambiguated typed tokens. The parser never
re-decides.

### `swagger:default` value

`classifyDefaultValue` tries `JSON_VALUE` first (full JSON
validation via the stdlib decoder), falling back to `RAW_VALUE`.
A leading quote / bracket / brace / sign / digit is the quick
discriminant; `true` / `false` / `null` are also JSON-valid.

### `swagger:enum` arguments

`classifyEnumArgs` implements the four-way dispatch on the
post-name remainder:

- empty → `enumFormEmpty` (multi-line body may follow);
- leading `[` → `enumFormBracketedOnly` (one `JSON_VALUE` arg, no name);
- leading identifier + no rest → `enumFormNameOnly`;
- leading identifier + leading `[` rest → `enumFormNamePlusBracketed`;
- otherwise → `enumFormPlainOnly` or `enumFormNamePlusPlain`.

Bracketed lists are emitted as a single `TokenJSONValue`; plain
lists as a single `TokenCommaListValue`; the name (when present)
as a separate `TokenIdentName`. Downstream parsing of the list
items lives in the analyzer.

### `swagger:type` argument

`looksLikeTypeRef` accepts any **well-formed** type-reference token —
an optionally `[]`-prefixed (array), optionally dot-qualified Go-style
identifier (`string`, `integer`, `int64`, `[]string`, `[][]int64`,
`Custom`, `pkg.Type`, `inline`, …) — as `TokenTypeRef`. The grammar no
longer owns a closed type vocabulary: semantic validity (known keyword /
scanned type, format compatibility, `[]T` element resolution) is the
builder's job, since only it knows the scanned definitions and the
annotated Go type (the F3 reconciliation). A **structurally malformed**
token (embedded spaces, bare `[]`, illegal chars, leading digit) falls
back to `TokenIdentName`, and the parser flags it `CodeInvalidTypeRef`
("not a well-formed type reference").

### HTTP method recognition

`classifyHTTPMethod` matches the closed HTTP-method vocabulary
(`GET` / `POST` / `PUT` / `PATCH` / `HEAD` / `DELETE` /
`OPTIONS` / `TRACE`) case-insensitively, emitting the canonical
upper-case form on `TokenHTTPMethod`.

### URL-path recognition

`looksLikeURLPath` is a coarse check (leading `/`). Full RFC 3986
conformance is left to the analyzer.

---

## <a id="parser-contract"></a>§parser-contract — Block construction

`DefaultParser.Parse` consumes a comment group end-to-end:
preprocess → lex → parse. `ParseAll` returns one Block per
annotation in source order; the partition rule splits at each
`TokenAnnotation` index. The first annotation owns the
pre-annotation prose; later annotations partition from one
annotation header to the next.

`ParseText` and `ParseAs` are entry points for non-CommentGroup
inputs (raw text from sub-parsers like routebody, or synthesised
annotation headers for tests).

The parser interface is a stable seam (`Parser`) so tests can
substitute mocks; the package ships `*DefaultParser` as the only
production implementation.

### Dispatch by family

`parseTokens` finds the first `TokenAnnotation`, looks up its
`AnnotationKind`, and dispatches by family:

| Family       | Annotations                                | Parser entry          |
|---|---|---|
| `familySchema` | `swagger:model`, `swagger:response`, `swagger:parameters`, `swagger:name` | `parseSchemaBlock` |
| `familyOperation` | `swagger:route`, `swagger:operation` | `parseOperationBlock` |
| `familyMeta` | `swagger:meta` | `parseMetaBlock` |
| `familyClassifier` | `swagger:strfmt`, `swagger:alias`, `swagger:allOf`, `swagger:enum`, `swagger:ignore`, `swagger:default`, `swagger:type`, `swagger:file`, `swagger:additionalProperties` | `parseClassifierBlock` |
| `familyUnknown` | unrecognised | `parseUnboundBlock` |

`swagger:name` dispatches through the schema family because its
body accepts the same validation-keyword vocabulary as a schema
field (min length, pattern, required, etc.). Surfacing those body
keywords as Properties — rather than rejecting them as
context-invalid under a classifier block — keeps the field-level
walker uniform.

### Body-token consumption

`consumeBodyToken` is the per-token sink shared across families:

- `TokenKeyword` → typed Property via `emitInlineKeyword`
  (validation against the keyword's shape).
- `TokenRawBlockBody` → raw Property via `emitRawBlock`. For
  `extensions:` / `infoExtensions:` the body is also fed through
  `yaml.TypedExtensions` to populate per-entry typed values
  ([§typed-extensions](#typed-extensions)). For `security:` the
  body is parsed into typed Requirements
  ([§security-requirements](#security-requirements)).
- `TokenRawValueBody` → raw Property via `emitRawValue`.
- `TokenOpaqueYaml` → `RawYAML` entry on the Block; emits
  `CodeUnterminatedYAML` if `Truncated`.
- Stray value-only tokens (`TokenIdentName` outside an owning
  keyword) emit `CodeUnexpectedToken`.

### Context-legality warnings

`contextLegal` reports whether a keyword may legally appear under
the given annotation kind. A mismatch is non-fatal — the parser
emits a `CodeContextInvalid` warning and still records the
property. See [§context-legality](#context-legality).

### parseState scaffolding

`parseState.peek` / `parseState.advance` and the `pos` cursor are
scaffolding for future order-sensitive productions (LSP partial
parses, strict positional checks on EnumDeclBlock's annotation
header → RAW_VALUE_ENUM body). Today's family parsers walk
`s.tokens` via range loops because the token classifier already
serialises the body — order between annotation header and body
items is flat. When order-sensitive productions land, the
per-family parsers will switch to peek/advance.

---

## <a id="block-shapes"></a>§block-shapes — typed Block kinds

Every Block implements the `Block` interface (one consumer
contract for builders + LSP):

- `Pos()`, `Title()`, `Description()`, `Diagnostics()`,
  `AnnotationKind()`;
- `Properties()`, `YAMLBlocks()`, `Extensions()`,
  `SecurityRequirements()`, `Contact()`, `License()`;
- `Walk(w Walker)` — the functional-visitor surface;
- `ProseLines()`, `PreambleLines()`, `PreambleTitle()`,
  `PreambleDescription()`, `Prose()`;
- `Has(name)`, `GetFloat`, `GetInt`, `GetBool`, `GetString`,
  `GetList`;
- `AnnotationArg()` — single-word convergence accessor for the
  annotation's primary positional argument.

Typed Block kinds embed `*baseBlock` and add per-annotation
fields:

| Block | Annotation | Extra fields |
|---|---|---|
| `ModelBlock` | `swagger:model [Name]` | `Name string` |
| `ResponseBlock` | `swagger:response [Name]` | `Name string` |
| `ParametersBlock` | `swagger:parameters <target> [args…]` | `Target ParametersTarget`, `Path string`, `Args []string`, `Dups []string` (+ `OperationIDs()` accessor) |
| `NameBlock` | `swagger:name <ident>` | `Name string` |
| `RouteBlock` | `swagger:route METHOD /path [tags] opID` | `Method, Path string; Tags []string; OpID string` |
| `InlineOperationBlock` | `swagger:operation METHOD /path [tags] opID` | same as `RouteBlock` |
| `MetaBlock` | `swagger:meta` | — |
| `ClassifierBlock` | `swagger:strfmt`, `swagger:type`, … | `Args []Token` |
| `EnumDeclBlock` | `swagger:enum [name] [values…]` | `Name string; InlineForm enumArgsForm; InlineArgs []Token; BodyValues string` |
| `UnboundBlock` | no annotation | — |

### Preamble vs full prose

`PreambleTitle` / `PreambleDescription` / `PreambleLines` cover
only the prose appearing **before** the block's annotation.
Schema's top-level model builder consumes the preamble so
post-annotation text reads as body content rather than as part of
the title/description. Routes / operations / meta consult the
full `Title()` / `Description()` (whole-block prose).

### Prose() — single-string description

`Prose()` returns the entire prose surface (TITLE + DESC tokens
in source order) joined with `\n`, internal blanks preserved as
paragraph breaks, a single trailing blank dropped. Used by
field-level callers (struct-field / interface-method docs) where
the whole prose is the description.

### AnnotationArg — convergence accessor

Returns the first single-word positional identifier argument of
the block's primary annotation, or `("", false)` for bare
annotations / multi-word args. Replaces type-asserting on each
typed Block kind to read its `Name` field. Used by Walker
callbacks that don't care which classifier flavour they are
looking at — only what its `IDENT_NAME`-style argument is.

`ClassifierBlock.AnnotationArg` filters to a single non-empty
word, mirroring the legacy single-word capture: prose lines that
happen to open with `swagger:<kind>` followed by a sentence are
rejected at this layer.

---

## <a id="property-shape"></a>§property-shape — Property and TypedValue

`Property` is one keyword:value (or keyword body) attached to a
Block. Field population varies by shape:

- **Inline-value keywords** (Number / Integer / Bool / String /
  EnumOption / CommaList): `Value` carries the raw string,
  `Typed` carries the lexically-typed form.
- **Body keywords** (RawBlock / RawValue): `Body` holds the
  accumulated body content (joined with `\n`), `Raw` holds the
  verbatim source content (indentation preserved), and
  `Typed.Type` is `ShapeRawBlock` / `ShapeRawValue`.

`ItemsDepth` records the leading `items.*` depth from the keyword
head — `0` for level-0 keywords, `N` for `items.…N` chain depth.

### TypedValue.Op for comparison-bound numbers

A NumberValue may carry a leading comparison operator (`<`, `<=`,
`>`, `>=`, `=`); the lexer strips it to `TypedValue.Op` so the
analyzer can decide inclusive vs exclusive semantics (`maximum:
<5` is exclusive max; `maximum: <=5` is inclusive). The Walker
collapses `<` / `>` to an `exclusive bool` on the Number callback.

### IsTyped — primitive-typed shortcut

`Property.IsTyped()` returns true when `Typed.Type` is one of the
primitive shapes (Number / Integer / Bool / EnumOption) — i.e. a
case where the matching `Typed.<field>` is populated and
authoritative. Returns false otherwise (raw shapes, comma-list,
string, ShapeNone). Consumers use it as a switch shortcut:

```go
if p.IsTyped() {
    // read p.Typed.<field> matching p.Keyword.Shape
} else {
    // coerce p.Value against the resolved schema type
}
```

### AsList — unified list extraction

`Property.AsList()` (also reachable via `Block.GetList(name)`)
unifies every list-shaped surface form:

```
Schemes: http, https            # inline, comma-separated
Schemes:                        # multi-line, indented bare
  http
  https
Schemes:                        # multi-line, YAML `- ` markers
  - http
  - https
Schemes:                        # markdown `* ` / `+ ` bullets (normalised
  * http                        #   to `- ` upstream in preprocess, so they
  * https                       #   reach AsList already as the dash form)
Schemes: http, https            # inline + indented continuation
  - ws
```

Algorithm: treat `Value` (if non-empty) as one input line, then
each line of `Body`. For each line: trim, drop a leading `- `
YAML marker if present, re-trim, comma-split, trim each token,
drop empties. Aggregate. Markdown `*`/`+` bullets need no special
handling here — [§preprocess-contract](#preprocess-contract) has
already normalised them to `- ` (go-swagger#1726).

The helper stops at "simple token lists" — it does **not** handle
enum values (whose elements may be JSON arrays), the `+ name:`
Parameters chunk grammar (routebody-owned), or raw bodies that
need YAML structural parsing (`securityDefinitions`,
`extensions`, `infoExtensions`, `security` — those travel through
`yaml.TypedExtensions` / `json.Unmarshal` / `security.Parse`
directly).

---

## <a id="walker-contract"></a>§walker-contract — functional-visitor dispatch

`Walker` is the functional-visitor surface a Block exposes for
bulk dispatch. Consumers wire only the callbacks they care about;
nil callbacks are silent no-ops.

### Dispatch order

1. Block-level diagnostics fire first (before Title) so consumers
   see them regardless of which property callbacks they wired.
2. `Title` fires once if non-empty.
3. `Description` fires once if non-empty.
4. Properties fire in source order — one callback per Property
   selected by `Keyword.Shape`:

   | Keyword.Shape   | Callback   | Payload                                  |
   |-----------------|------------|------------------------------------------|
   | `ShapeNumber`   | `Number`   | `(p, p.Typed.Number, exclusive)`         |
   | `ShapeInt`      | `Integer`  | `(p, p.Typed.Integer)`                   |
   | `ShapeBool`     | `Bool`     | `(p, p.Typed.Boolean)`                   |
   | `ShapeString`   | `String`   | `(p, p.Value)`                           |
   | `ShapeEnumOption` | `String` | `(p, p.Typed.String)`                    |
   | `ShapeRawBlock` | `Raw`      | `(p)` — caller reads `p.Body` / `p.Raw`  |
   | `ShapeRawValue` | `Raw`      | `(p)`                                    |
   | `ShapeCommaList` | `Raw`     | `(p)` — caller splits via `b.GetList`    |
   | `ShapeNone`     | `Raw`      | `(p)` — fallback                         |

   An **unknown** keyword (Property.Keyword.Name empty) fires the
   `Unknown` callback instead.

5. Extensions fire in source order, one callback per Extension entry.

### Iteration scope

Walker walks one Block per call; ordering across blocks (multiple
declarations, file order, discovery order) is the builder's
concern, not the walker's.

### Shape-based dispatch, not Typed.Type

When the lexer rejects an invalid value (e.g. `maximum:
notanumber`) the parser leaves `Typed.Type` at `ShapeNone` and
emits a `CodeInvalidNumber` diagnostic. Walker still dispatches
based on `Keyword.Shape` — `Number/Integer/Bool` callbacks fire
with the zero value of the payload. Consumers treat the
`Diagnostic` callback as authoritative for malformed values
rather than re-validating.

### FilterDepth — items-chain gating

`FilterDepth` gates property callbacks (Number / Integer / Bool /
String / Raw / Unknown). Title / Description / Extension /
Diagnostic are unaffected.

- `AllDepths` (`-1`) admits every depth — use this explicitly for
  "fire every property" rather than `-1` so the intent reads at
  the call site.
- `0` admits level-0 only — the schema-side default.
- `N` admits depth N only — used by items-chain walkers.

**Zero-value gotcha:** the Go zero value of `FilterDepth` is `0`,
which means "level-0 only". Items callers must explicitly set
`FilterDepth` to the wanted depth; they cannot leave it at the
zero value. Schema-side level-0 walkers can leave it at zero by
accident-and-design.

### Concurrency

`Walk` reads only from the Block — it never mutates the Block or
its properties. `Walk` is safe to call concurrently on the same
Block from multiple goroutines as long as the Walker callbacks
are themselves safe.

---

## <a id="keyword-table"></a>§keyword-table — closed-vocabulary keywords

`keywords.go` defines the authoritative keyword table. Each entry
declares a canonical name, optional aliases, a `ValueShape`, and
the family contexts where it is legal
([§context-legality](#context-legality)).

`Kw*` constants (`KwMaximum`, `KwSchemes`, …) are the single source
of truth for spelling: every Property's `Keyword.Name` compares
equal to exactly one of them. Consumers that switch on
`Keyword.Name` should reference the constants rather than
re-declaring the strings — the schema walker and the bridge
dispatchers in routes / parameters / responses / operations /
items / spec all dispatch on these names.

### ValueShape vocabulary

| Shape | Terminal | Notes |
|---|---|---|
| `ShapeNumber` | NUMBER_VALUE | signed decimal, optional leading comparison operator |
| `ShapeInt` | INT_VALUE | unsigned decimal integer |
| `ShapeBool` | BOOL_VALUE | `true` / `false` (case-insensitive) |
| `ShapeString` | STRING_VALUE | verbatim non-LF text |
| `ShapeCommaList` | COMMA_LIST_VALUE | comma-separated list of strings (trim-stripped) |
| `ShapeEnumOption` | ENUM_OPTION_VALUE | closed-vocab choice (Values lists the allowed set) |
| `ShapeRawBlock` | RAW_BLOCK_\<KW\> | multi-line body terminal — caller reads Body/Raw |
| `ShapeRawValue` | RAW_VALUE_\<KW\> | multi-line OR single-line body terminal |
| `ShapeNone` | — | no value shape (rare; ShapeNone keywords reach Walker's Raw callback) |

`ValueShape.IsBody()` reports whether the shape is a multi-line
body terminal (RawBlock or RawValue) — the lexer's body
accumulator triggers on body shapes.

### Lookup

`Lookup(name)` matches the canonical name or any alias,
case-insensitively. Aliases cover common variants (`max length`,
`max-length`, `maxLen`, `maximum length`, … all match
`KwMaxLength`). The lexer applies first-character case
folding before lookup; alias matching is fully case-insensitive.

`Keywords()` returns a defensive copy of the authoritative table
for tooling that needs to enumerate it.

### Multi-line raw-block keywords

`KwConsumes`, `KwProduces`, `KwSecurity`, `KwSecurityDefinitions`,
`KwResponses`, `KwParameters`, `KwExtensions`, `KwInfoExtensions`,
`KwTOS`, `KwExternalDocs`, `KwTags` are all `ShapeRawBlock`. Their
bodies travel through the lexer's body accumulator and surface on
the Block as raw Properties; downstream sub-parsers (yaml,
routebody, security) consume the body content. `KwTags` carries two
shapes by context: on swagger:meta it is a list of tag **objects**
({name, description, externalDocs, x-*}) populating
`spec.Swagger.Tags`; on swagger:route/operation it is a plain list of
tag-name **strings** unioned onto `op.Tags` (alongside any names on
the route header line). The single keyword, two consumers — the meta
walker unmarshals objects, the route walker reads `AsList`.

### `in:` is a parameter-location directive

`KwIn` is declared as `ShapeEnumOption("query", "path", "header",
"body", "formData")` in `CtxParam`. It is not part of the formal
schema-body grammar; the keyword table recognises it so the lexer
can hand a typed token to the parameters dispatch path. The
schema parser treats it as a context-invalid warning when seen
outside that path.

### `name:` is a name directive

`KwName` is declared as `ShapeString` in `CtxParam, CtxHeader`. Like
`in:`, it is a structural keyword rather than part of the schema-body
grammar. On a `swagger:parameters` struct field it renames the JSON
parameter name; on a `swagger:response` struct field it renames the
response header (the `Headers` map key). Both override the json-tag /
Go-field derivation (the SimpleSchema-side analogue of the
`swagger:name` annotation on a schema field). Recognising it as a
keyword removes it from the description prose; because its contexts are
SimpleSchema field sites, `isFullSchemaOnly` is false for it, so the
parameter / header walkers ignore it silently rather than emitting an
unsupported-keyword warning. The parameters / responses builders read
the value via `Block.GetString(KwName)`.

### `Schemes:` accepts both inline and multi-line

`KwSchemes` uses `ShapeRawBlock` so multi-line bodies
(`Schemes:\n  - http\n  - https`) populate the same way they do
for Consumes/Produces. The inline comma-list form (`Schemes: http,
https`) still works via the inline-value capture in
`collectRawBlock` ([§raw-block-terminators](#raw-block-terminators)).
`Block.GetList` unifies both surfaces.

---

## <a id="context-legality"></a>§context-legality — per-annotation keyword legality

`KeywordContext` enumerates the family-level contexts where a
keyword may appear: `CtxParam`, `CtxHeader`, `CtxSchema`,
`CtxItems`, `CtxRoute`, `CtxOperation`, `CtxMeta`, `CtxResponse`.
Each `Keyword.Contexts` lists the contexts the keyword is legal in.

`parser.allowedContexts(kind)` maps each `AnnotationKind` to the
context set legal under it:

| AnnotationKind | Allowed contexts |
|---|---|
| `AnnModel` | `CtxSchema`, `CtxItems` |
| `AnnParameters` | `CtxParam`, `CtxSchema`, `CtxItems` |
| `AnnResponse` | `CtxResponse`, `CtxSchema`, `CtxHeader`, `CtxItems` |
| `AnnOperation` | `CtxOperation`, `CtxParam`, `CtxSchema`, `CtxHeader`, `CtxItems`, `CtxResponse` |
| `AnnRoute` | `CtxRoute`, `CtxParam`, `CtxSchema`, `CtxHeader`, `CtxItems`, `CtxResponse` |
| `AnnMeta` | `CtxMeta`, `CtxSchema` |
| Classifier kinds & `AnnUnknown` | nil (no parser-layer policy) |

`contextLegal(kw, kind)` returns true when the keyword's contexts
overlap with the kind's allowed contexts. A missing overlap is a
`CodeContextInvalid` warning — the property is still recorded so
the builder can decide policy.

---

## <a id="annotation-args"></a>§annotation-args — argument terminals

Per-annotation argument shapes are classified by
`classifyAnnotationArgs` and emitted as typed `Token`s on
`TokenAnnotation.Args`:

| Kind | Argument tokens |
|---|---|
| `AnnRoute`, `AnnOperation` | `TokenHTTPMethod` + `TokenURLPath` + `TokenIdentName`* (tags + trailing OpID) |
| `AnnDefaultName` | one `TokenJSONValue` or `TokenRawValue` per `classifyDefaultValue` |
| `AnnType` | one `TokenTypeRef` for any well-formed token (or fallback `TokenIdentName` when malformed) per `looksLikeTypeRef` |
| `AnnEnum` | per `classifyEnumArgs` — `TokenIdentName` (name) + `TokenJSONValue` / `TokenCommaListValue` (values), in source order |
| `AnnParameters` | first token: `TokenWildcard` (`*`), `TokenURLPath` (`/path`), or `TokenIdentName` (operation id); trailing `TokenIdentName`* (operation ids or shared-parameter names) |
| `AnnAllOf`, `AnnModel`, `AnnResponse`, `AnnStrfmt`, `AnnName` | one `TokenIdentName` (first identifier only — single-word capture) |
| `AnnAlias`, `AnnIgnore`, `AnnFile`, `AnnMeta`, `AnnUnknown` | trailing fields as `TokenIdentName`* so the parser can diagnose |

### Operation arg extraction

`parseOperationArgs` extracts `METHOD`, `/path`, `[tags…]`,
`OperationID`. The trailing `TokenIdentName` is the OpID; any
preceding `TokenIdentName`s are tags. Missing or invalid pieces
emit `CodeMalformedOperation`.

### Schema-family arg validation

- `AnnParameters` requires a target token (an operation id, the
  shared-namespace `*`, or a `/path`) — empty emits
  `CodeMissingRequiredArg`. The first token classifies the
  `ParametersBlock.Target` (operations / shared / path); `parseParametersArgs`
  collects the remaining tokens into `Args` (de-duplicated; dropped
  duplicates recorded in `Dups`). The definition-vs-reference reading of
  `Args` is the builder's, since it depends on the host declaration.
- `AnnName` requires a single IDENT_NAME — empty emits
  `CodeMissingRequiredArg`.

### Classifier-family arg validation

- `AnnStrfmt` requires a name; empty emits `CodeMissingRequiredArg`.
- `AnnDefaultName` requires a value; missing emits `CodeMissingRequiredArg`.
- `AnnType` requires a `TokenTypeRef`; a missing arg emits
  `CodeMissingRequiredArg` and a structurally malformed token emits
  `CodeInvalidTypeRef`. A well-formed but unknown name is NOT a parser
  error — the builder resolves it (and emits `validate.unsupported-type`
  if it is neither a keyword nor a scanned type).
- `AnnEnum` requires a name and/or value list and/or a body;
  empty across all three emits `CodeMissingRequiredArg`.
- `AnnAllOf`, `AnnIgnore`, `AnnAlias`, `AnnFile` accept optional /
  no args.

---

## <a id="typed-extensions"></a>§typed-extensions — `extensions:` body → typed map

`collectExtensionsFromBody` parses the body of an `extensions:`
or `infoExtensions:` raw block through `yaml.TypedExtensions` and
registers one `Extension` per top-level `x-*` entry, carrying its
YAML-typed value (`bool` / `float64` / `string` / `[]any` /
`map[string]any`).

`Extension.Source` carries the keyword that produced the entry:
`KwExtensions` (top-level vendor extensions) vs
`KwInfoExtensions` (Info-scoped, meta-only). Consumers that route
entries to different targets — meta's `swspec.Extensions` vs
`swspec.Info.Extensions` — switch on this field; consumers that
treat extensions uniformly (routes, operations) can ignore it.

### Drop policy

- Non-`x-*` keys are dropped with a `CodeInvalidAnnotation`
  warning, so authors who typo a vendor-extension key (e.g.
  `invalid-key:` under `Extensions:`) get a signal rather than
  silent loss.
- A YAML parse failure emits a `CodeInvalidYAMLExtensions`
  warning and the block is skipped (no Extension entries
  registered).

### Position fidelity

Every Extension currently shares the `extensions:` keyword's
position. Per-entry positions require `*yaml.Node` walking and
can be added when LSP-grade diagnostics need them — see the YAML
sub-parser package.

### `isExtensionName`

A well-formed extension name starts with `x-` or `X-`, length
≥ 3. The check is local to this package; the JSON encoder layer
applies its own validation.

---

## <a id="security-requirements"></a>§security-requirements — typed Requirements

A `security:` raw block in a meta / route / operation context is
parsed at lex time into a `[]security.Requirement` and made
available via `Block.SecurityRequirements()`. Each entry is a map
from scheme name → scope list (one key per scheme; multiple keys in
one entry are AND-combined), mirroring the shape OAS v2 expects on
`spec.Operation.Security`.

`security.Parse` decodes the body as genuine YAML — the same path
`securityDefinitions` takes — so the standard sequence-of-objects
form (with flow- or block-style scopes), the bare-name shorthand
(`- name`), and the explicit opt-out (`security: []` → non-nil
empty) all work; a legacy bare top-level mapping is still read as
one OR requirement per key. See the `security` package doc for the
full form list.

`parser.emitRawBlock` calls `security.Parse(body)` when the
keyword name is `KwSecurity`. Returns `nil` when no `security:`
keyword appeared on the block (absent → inherit), distinct from the
non-nil empty slice returned for `security: []` (opt-out).

The companion accessor — `Block.Contact()` / `Block.License()` —
exposes the typed shapes parsed from inline `contact:` /
`license:` values ([§contact-license](#contact-license)).

---

## <a id="contact-license"></a>§contact-license — typed Contact / License

`Contact` is the typed shape of a `contact:` inline value on a
swagger:meta block:

```
contact: <Name> <email> <URL>
```

Each part is optional in the order written: `parseContact`
recognises a `Name <email>` head (Go's `net/mail.ParseAddress`
form) followed by an optional URL. A bare email without a name is
accepted. Empty / unrecognised input returns `(Contact{}, nil)`.
A malformed `Name <email>` head returns `(Contact{}, err)` — the
caller decides whether to fail or warn.

`License` is the typed shape of a `license:` inline value:

```
license: <Name> <URL>
```

`parseLicense` splits on the URL prefix; Name may be empty when
the line starts with the URL. Empty input returns
`(License{}, false)`.

`splitURL` recognises the leading URL prefix from a closed set:
`https://`, `http://`, `ftps://`, `ftp://`, `wss://`, `ws://`.

---

## <a id="diagnostics"></a>§diagnostics — Code / Severity model

`Diagnostic` is one observation about a comment block:

- `Pos` — source position;
- `Severity` — `SeverityError`, `SeverityWarning`, or
  `SeverityHint`;
- `Code` — stable identifier (`parse.invalid-number`,
  `validate.shape-mismatch`, …);
- `Message` — human-readable text.

`Errorf` / `Warnf` / `Hintf` are convenience constructors.
`Diagnostic.String()` renders compiler-style one-line form.

### Code prefixes

- `parse.*` — lexer / parser-level observations emitted by the
  grammar package itself.
- `validate.*` — semantic-validation observations emitted by the
  builder layer (typically through `internal/builders/validations`).

### Parser never aborts

The parser emits diagnostics and continues. Callers (analyzers,
LSP, the CLI) decide policy by severity. The parser layer never
returns an error to the consumer; diagnostics are observable on
the returned Block (`Block.Diagnostics()`) and via the
diagnostic-sink option (`WithDiagnosticSink`) for streaming.

### Defined codes

| Code | Description |
|---|---|
| `CodeInvalidNumber` | malformed number value |
| `CodeInvalidInteger` | malformed integer value |
| `CodeInvalidBoolean` | not `true`/`false` |
| `CodeInvalidEnumOption` | not in the closed set |
| `CodeContextInvalid` | keyword not legal under the current annotation |
| `CodeInvalidExtension` | malformed extension name |
| `CodeInvalidYAMLExtensions` | YAML parse failure on extensions body |
| `CodeUnterminatedYAML` | `---` opened, not closed |
| `CodeInvalidAnnotation` | malformed annotation surface |
| `CodeInvalidTypeRef` | structurally malformed `swagger:type` token (semantic validity is the builder's job) |
| `CodeUnexpectedToken` | stray token at body level |
| `CodeMalformedOperation` | missing/invalid HTTP method / path / OpID |
| `CodeMissingRequiredArg` | annotation requires an argument |
| `CodeShapeMismatch` | builder-layer keyword vs schema-type mismatch |
| `CodeAmbiguousEmbed` | builder-layer embed disambiguation diagnostic |
| `CodeUnsupportedInSimpleSchema` | builder-layer SimpleSchema-exit violation |
| `CodeUnsupportedType` | builder-layer unresolvable `swagger:type` (unknown name / `file` / invalid array element) |
| `CodeDeprecated` | builder-layer accepted-but-deprecated annotation/keyword (carries a migration hint) |

---

## <a id="synthetic-block"></a>§synthetic-block — sub-parser construction

`NewSyntheticBlock(pos, title, description, props)` builds a
Block from a manually-curated set of Properties. Used by
sub-parsers (routebody, future input modes) that lower a
non-grammar text surface into the standard Block shape so
consumers can dispatch through the usual Walker.

`title` and `description` become the Block's `Title()` /
`Description()`, also surfaced via `Prose()` with internal blank
separation. `pos` is the source position of the synthetic
block's head — Properties that lack their own `Pos` inherit it
implicitly when consumers build diagnostics.

The returned Block exposes empty `Diagnostics()`,
`AnnotationKind() == AnnUnknown`, no YAML blocks, no extensions,
and no security requirements. `AnnotationArg()` returns
`("", false)`. `Walk` fires Title / Description first when
non-empty, then properties in slice order — the regular Walker
contract.

---

## <a id="quirks-open"></a>§quirks-open — deferred follow-ups

### Body-shape choices retained as-is

`Body` is a single string with embedded `\n`; `Raw` carries
verbatim source indentation. Consumers that prefer a `[]string`
shape call `strings.Split(body, "\n")` themselves. Switching to
`[]string` at the Property level would force every consumer to
re-join; the single-string form pays the cost where it is
needed.

### Position fidelity on multi-entry bodies

Extensions and security requirements share the parent keyword's
position. Per-entry positions require walking the `*yaml.Node`
tree from the YAML parser; LSP-grade diagnostics may want this
in a later pass.

### Closed-vocab annotation prefix

`AnnotationPrefix` is fixed at `"swagger:"`. A configurable
prefix would interact with the first-character case-insensitive
fallback (which is tied to ASCII letter casing). A non-letter
prefix character would not need the fallback. No current
consumer asks for this; the constant is the minimal scaffolding
for a future Option promotion.
