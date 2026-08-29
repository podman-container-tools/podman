// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/spec"
)

// Options configures a scan.
//
// The zero value is a valid configuration: every flag defaults to false and every slice/map
// defaults to nil.
//
// # Details
//
// See [§options](./README.md#options) for the field overview, and
// [§descwithref](./README.md#descwithref) and [§diagnostics](./README.md#diagnostics) for the two
// fields with non-trivial semantics (DescWithRef and OnDiagnostic).
type Options struct {
	Packages                []string
	InputSpec               *spec.Swagger
	ScanModels              bool
	WorkDir                 string
	BuildTags               string
	ExcludeDeps             bool
	Include                 []string
	Exclude                 []string
	IncludeTags             []string
	ExcludeTags             []string
	SetXNullableForPointers bool
	RefAliases              bool // aliases result in $ref, otherwise aliases are expanded
	TransparentAliases      bool // aliases are completely transparent, never creating definitions
	// DescWithRef controls description preservation on $ref'd fields in the
	// description-only-decoration case: when a struct field's Go type resolves to a named type ($ref)
	// and its only field-level decoration is a description (no validations, no user-authored
	// extensions).
	//
	//   - false (default): the description is dropped and the field
	//     emits as a bare `{$ref: ...}`.
	//   - true: the description is preserved by wrapping the $ref in
	//     a single-arm `allOf` compound — `{description: "...",
	//     allOf: [{$ref}]}` — the JSON-Schema-draft-4 correct shape
	//     for sibling description.
	//
	// When the field also carries validation overrides (pattern, enum, example, etc.) or user-authored
	// vendor extensions, the allOf compound is mandatory regardless of this flag — the override
	// would be lost otherwise.
	//
	// Deprecated: prefer EmitRefSiblings, which preserves description AND extensions as direct $ref
	// siblings (the modern, lenient shape).
	// DescWithRef is retained with its original semantics (the strict draft-4 single-arm allOf wrap
	// for the description-only case) and remains a no-op when EmitRefSiblings is set.
	//
	// See [§ref-override](../builders/schema/README.md#ref-override).
	DescWithRef bool

	// EmitRefSiblings emits a $ref'd field's description and vendor extensions as DIRECT siblings of
	// the `$ref` (`{$ref, description, x-*}`) instead of wrapping them in an allOf compound.
	//
	// Strict JSON-Schema-draft-4 ignores siblings of `$ref` (hence the default allOf wrap), but
	// OpenAPI 3.1 / JSON Schema 2020-12 and most modern Swagger-UI renderers honour them.
	//
	//   - false (default): description / extensions follow the legacy
	//     wrap behaviour (extensions lift onto a single-arm allOf;
	//     description-only is governed by DescWithRef).
	//   - true: description and extensions ride directly alongside the
	//     `$ref`, no allOf.
	//
	// Validations and externalDocs are NOT siblings-eligible: when present they still force an allOf
	// compound (validations on the override arm), and description / extensions then ride the outer
	// compound.
	// This flag changes only the no-forced-compound cases.
	//
	// See [§ref-override](../builders/schema/README.md#ref-override).
	EmitRefSiblings bool

	// SkipAllOfCompounding disables the allOf-compound rewrite for $ref'd struct fields entirely: no
	// allOf compound is ever emitted.
	//
	//   - false (default): siblings are preserved via the allOf compound
	//     (or, under EmitRefSiblings, as direct $ref siblings).
	//   - true: no compound is produced. Validations and externalDocs —
	//     which can only ride a compound — are DROPPED. Description and
	//     extensions are likewise dropped UNLESS EmitRefSiblings is also
	//     set, in which case they survive as direct `$ref` siblings.
	//     Every drop raises one diagnostic through OnDiagnostic — the
	//     loss is never silent.
	//
	// `required:` is a parent-side concern (it lands on the enclosing object's `required` list, not as
	// a $ref sibling) and is preserved regardless of this flag.
	//
	// Intended for downstream consumers (e.g. go-swagger codegen) that expect a bare `$ref` for a
	// field pointing at a model and do not handle the allOf-compounded shape.
	// See [§ref-override].
	SkipAllOfCompounding bool

	// DefaultAllOfForEmbeds changes how a plain (non-`swagger:allOf`-tagged) struct embed renders:
	// into allOf composition instead of inlined properties.
	//
	// By default codescan inlines an embedded struct's properties into the embedding schema (mirroring
	// Go field promotion), so the "this composes Y" relationship is lost — every embedding struct
	// emits a flat copy of the embedded fields.
	//
	// Downstream client generators that want a reusable base type per embed prefer the composition
	// shape instead.
	//
	//   - false (default): plain embeds inline their properties (existing
	//     behaviour).
	//   - true: a plain embed is treated as if it carried `swagger:allOf` —
	//     it becomes an allOf member ($ref to the embedded type's definition
	//     when that type is a model, otherwise an inline member), and the
	//     embedding struct's own fields move into a sibling allOf member.
	//
	// Scope and precedence:
	//   - Only STRUCT embeds are affected. Interface embeds already compose via
	//     allOf and are unchanged.
	//   - An explicit `swagger:allOf` annotation already produces this shape;
	//     the flag only makes it the default for untagged embeds.
	//   - An embed carrying an explicit json tag name (or `swagger:name`) is a
	//     single named property, not a promotion, so it is left as a nested
	//     property regardless of this flag (go-swagger#2038).
	//   - Pointer embeds are peeled; aliased embeds resolve to their unaliased
	//     type; stdlib specials (`error`, `time.Time`) keep their canonical
	//     recognizer shape — all via the existing allOf path.
	//
	// See [§allof](../builders/schema/README.md#allof).
	DefaultAllOfForEmbeds bool

	SkipExtensions bool // skip generating x-go-* vendor extensions in the spec

	// NameFromTags is the ordered list of struct-tag types consulted to derive the emitted name of a
	// schema property, parameter, or response header from a Go struct field.
	//
	// The first listed tag type that supplies a usable name wins; a tag type that is absent or carries
	// only options (e.g. `,omitempty`) is skipped and the next is tried.
	// When no listed tag names the field, the Go field name is used.
	//
	//   - nil / unset (default): ["json"] — the historic behaviour.
	//   - explicit empty slice: no struct tag is consulted; the name derives
	//     from the Go field name.
	//   - e.g. ["form","json"]: prefer the `form:` name (used by gin), falling
	//     back to `json:` (go-swagger#2912, go-swagger#1391).
	//
	// Only the NAME is sourced this way.
	// The encoding/json directives `-` (exclude), `,omitempty` (→ not required) and `,string` are
	// always read from the `json` tag regardless of this setting.
	//
	// Targeted renames — the `name:` keyword (parameters / response headers) and `swagger:name` /
	// `swagger:model {name}` (schema) — still take precedence over any tag-derived name.
	NameFromTags []string

	// SkipJSONifyInterfaceMethods opts out of the auto-jsonify mangler applied to interface-method
	// property names.
	//
	// An interface method has no "natural" JSON serialization (Go's encoding/json cannot marshal
	// embedded interface methods without a custom marshaler), so codescan invents a default property
	// name by running the swag/mangling ToJSONName transform on the Go method name (`CreatedAt` →
	// `createdAt`, `ID` → `id`).
	//
	// This convention will not always match the author's intent — e.g. an interface already named
	// for its JSON shape, or a codebase with its own canonical-name discipline.
	//
	//   - false (default): interface-method names auto-jsonify (existing
	//     behaviour).
	//   - true: the Go method name is emitted verbatim; the mangler is skipped.
	//
	// A `swagger:name X` override is taken verbatim regardless of this flag — it already bypasses
	// the mangler.
	// This flag only changes the fallback used when no override is present.
	// It does not affect struct-field naming, which mirrors what encoding/json actually produces.
	//
	// See [§interface-naming](../builders/schema/README.md#interface-naming).
	SkipJSONifyInterfaceMethods bool

	// SkipEnumDescriptions controls whether the per-enum-value const-name mapping built from
	// `swagger:enum` (e.g. "FIRST TestEnumFirst") is folded into the property / parameter / header
	// `description`.
	//
	//   - false (default): the mapping is appended to the authored
	//     description AND exposed via the `x-go-enum-desc` vendor extension
	//     (backward-compatible behaviour).
	//   - true: the description is left as the authored prose; the mapping
	//     rides `x-go-enum-desc` only.
	//
	// Independent of SkipExtensions: with SkipExtensions also set, the mapping is suppressed
	// everywhere.
	// See go-swagger/go-swagger#2922.
	SkipEnumDescriptions bool

	// NameConcatBudget tunes the readability cutoff used when the name-identity reduce stage
	// deconflicts colliding definition names by concatenating package segments (b.Test / c.Test ->
	// BTest / CTest).
	//
	// Each candidate concat is scored in [0,1] — lower is more readable (shorter overall, fewer
	// parts, no over-long segment).
	// A collision group whose best concat scores ABOVE the budget is a candidate for the hierarchical
	// fallback (name-identity Stage 3 / K3).
	//
	// The zero value selects the built-in default (0.65).
	// Raise it toward 1.0 to accept longer concats; lower it to fall back sooner.
	NameConcatBudget float64

	// EmitHierarchicalNames enables the hierarchical fail-safe for the rare collision groups whose
	// best flat concat exceeds NameConcatBudget.
	//
	// When set, such a group is emitted as nested container definitions (`#/definitions/<pkg>/<Name>`,
	// with `additionalProperties:true` + `x-go-package` on each container) instead of a long flat
	// concat, and an explanatory diagnostic is raised.
	//
	// Default false — and deliberately so: a nested definition is a deep JSON pointer that only
	// `ExpandSpec` resolves, and a definitions- enumerating consumer (e.g. go-swagger codegen, one
	// model per entry) sees the container nodes rather than the models.
	//
	// The always-correct flat concat stays the default; enable this only when you prefer the nested
	// shape for the over-budget tail.
	EmitHierarchicalNames bool

	// EmitXGoType stamps an `x-go-type` vendor extension on every emitted definition, recording the
	// fully-qualified originating Go type (`<package path>.<type name>`) alongside the existing
	// `x-go-name` / `x-go-package` traceability extensions.
	//
	//   - false (default): no `x-go-type` is emitted for ordinary types
	//     (the extension still appears on the narrow special-type cases
	//     that have always carried it — `error`, the unmodellable
	//     generic-type fallback).
	//   - true: each definition carries `x-go-type`, useful for
	//     round-tripping a generated spec back to its source Go types.
	//
	// Under the SkipExtensions umbrella: with SkipExtensions also set, no vendor extension is emitted
	// regardless.
	// See go-swagger/go-swagger#2924.
	EmitXGoType bool

	// SingleLineCommentAsDescription routes a single-line doc comment to the object's `description`
	// regardless of trailing punctuation, never to `title` / `summary`.
	//
	//   - false (default): the first-sentence convention applies — a
	//     single-line comment ending in punctuation (`.`, `!`, `?`)
	//     becomes the `title` (model / info) or `summary` (operation);
	//     without trailing punctuation it is a `description`.
	//   - true: a single-line comment is always a `description`. Multi-
	//     line comments keep the existing title/description split (the
	//     first line, or the paragraph before the first blank line, is
	//     still the title).
	//
	// See go-swagger/go-swagger#2626.
	SingleLineCommentAsDescription bool

	// AfterDeclComments, when set, lets swagger annotations live INSIDE a declaration (the leading
	// comment of a struct body) or INLINED as a trailing comment, in addition to the doc comment above
	// the declaration.
	//
	// The godoc above the declaration then stays clean and human-facing while the swagger machinery
	// lives out of the published documentation.
	// The scanner folds the located comments into the comment source the builders already consume —
	// same annotation grammar, no new syntax.
	//
	// Default false.
	//
	// v0.36 scope: type declarations (swagger:model / swagger:parameters / swagger:response) —
	// struct inside-body leading comments and the trailing comment of an alias / non-struct type.
	// Routes / operations are already position-agnostic.
	// Struct fields and const enums are follow-ups.
	AfterDeclComments bool

	// CleanGoDoc rewrites godoc-specific syntax that reads as noise when a title / description is
	// carried from a Go doc comment into the spec.
	//
	// It applies ONLY to godoc-derived prose — author-written swagger:title / swagger:description
	// overrides are never touched.
	//
	//   - false (default): godoc prose is emitted verbatim (existing
	//     behaviour; output is byte-identical).
	//   - true: godoc doc-link brackets are removed and the identifier is
	//     humanized (`[CustName]` → "cust name"); reference-style link
	//     definition lines (`[text]: url`) are dropped; and when a doc-link
	//     resolves to an emitted schema, it is recomposed to the name that
	//     schema is actually exposed under (so the prose stays true to the
	//     generated definitions). The first identifier of a title /
	//     description is restored to sentence case.
	//
	CleanGoDoc bool

	// PruneUnusedModels, when set together with ScanModels, drops every discovered definition that is
	// not transitively referenced from a path, a shared response, a shared parameter, or a definition
	// supplied via InputSpec.
	//
	// It is the middle ground between the two default modes:
	//
	//   - without ScanModels: only route-reachable models are emitted;
	//   - with ScanModels (`-m`): every swagger:model type is emitted, reachable
	//     or not;
	//   - with ScanModels + PruneUnusedModels: swagger:model discovery runs, then
	//     the unreachable definitions are pruned again — useful when scanning a
	//     large shared library where only the $ref'd subset is wanted.
	//
	// Pruning runs BEFORE definition-name reduction, so an unused model can no longer force a spurious
	// cross-package name collision on a model that IS used (the survivor keeps its clean short name).
	// Definitions supplied via InputSpec are pinned: they are never pruned and seed the reachability
	// roots.
	//
	// Each pruned definition raises a scan.pruned-unused Hint through OnDiagnostic — the loss is
	// never silent.
	//
	// Without ScanModels this flag is a no-op (the emitted set is already reachable-only); setting it
	// alone raises one Hint.
	// Default false.
	//
	// Note: a discriminator base references its subtypes by mapping string, not by $ref, so a subtype
	// reachable only through a discriminator could be pruned. codescan does not auto-wire
	// discriminator subtypes today; revisit if it ever does.
	// See go-swagger/go-swagger#2639.
	PruneUnusedModels bool

	// Debug is deprecated and has no effect.
	//
	// It formerly enabled verbose debug logging to stderr during scanning.
	// That logger was retired: scan-time observations now flow exclusively through OnDiagnostic (which
	// the caller routes to a logger of their choice), and codescan no longer writes to stdout/stderr
	// — keeping it usable from a TUI or a WASI/WASM host.
	//
	// Deprecated: wire OnDiagnostic instead.
	// This field is retained for API compatibility and is ignored.
	Debug bool

	// OnDiagnostic, when non-nil, is invoked for every diagnostic the builder layer records
	// (lexer/parser warnings, semantic-validation failures from the validations package, etc.).
	//
	// The callback fires once per diagnostic in source order; diagnostics never block the build —
	// invalid constructs are silently dropped from the output spec while their explanation flows
	// through this channel.
	//
	// Experimental: the public API surface for diagnostics is subject to change while LSP integration
	// matures.
	// See [§diagnostics](./README.md#diagnostics).
	OnDiagnostic func(grammar.Diagnostic)

	// OnProvenance, when non-nil, is invoked once per anchor node in the produced spec, carrying its
	// JSON pointer and the source position of the Go construct that produced it (see [Provenance]).
	//
	// Anchors are code-detail nodes (type decls, fields, values, route/meta blocks); finer nodes
	// resolve to their nearest anchored ancestor at the consumer.
	// The callback never blocks the build.
	//
	// Experimental: the cross-ref surface may change while LSP / TUI integration matures.
	OnProvenance func(Provenance)
}
