// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"fmt"
	"go/token"
)

// Severity classifies a Diagnostic's seriousness.
//
// The parser never aborts; callers (analyzers, LSP, the CLI) decide policy by severity.
// See README §diagnostics.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityHint
)

// String renders a Severity for logs and CLI output.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityHint:
		return "hint"
	default:
		return fmt.Sprintf("severity(%d)", int(s))
	}
}

// Code is a stable identifier for a class of Diagnostic.
type Code string

// Diagnostic codes.
//
// The `parse.*` prefix marks lexer/parser-level observations; `validate.*` marks
// semantic-validation observations emitted by the builder layer (typically through the
// internal/builders/validations package); `scan.*` marks scan-environment observations (package
// loading, recovered panics) raised by the scanner / spec builder rather than the grammar parser.
const (
	CodeInvalidNumber     Code = "parse.invalid-number"
	CodeInvalidInteger    Code = "parse.invalid-integer"
	CodeInvalidBoolean    Code = "parse.invalid-boolean"
	CodeInvalidEnumOption Code = "parse.invalid-enum-option"
	CodeContextInvalid    Code = "parse.context-invalid"
	CodeInvalidExtension  Code = "parse.invalid-extension-name"
	// CodeInvalidYAMLExtensions fires when the body of an `extensions:` raw block fails YAML parsing.
	//
	// The block is skipped (no Extension entries emitted) and a warning is raised.
	CodeInvalidYAMLExtensions Code = "parse.invalid-yaml-extensions"
	CodeUnterminatedYAML      Code = "parse.unterminated-yaml"
	CodeInvalidAnnotation     Code = "parse.invalid-annotation"
	CodeInvalidTypeRef        Code = "parse.invalid-type-ref"
	CodeUnexpectedToken       Code = "parse.unexpected-token"
	CodeMalformedOperation    Code = "parse.malformed-operation"
	CodeMissingRequiredArg    Code = "parse.missing-required-arg"

	// CodeShapeMismatch fires when a keyword is applied to a schema whose resolved Swagger type
	// doesn't match the keyword's domain (e.g. `pattern: ^a$` on an integer field).
	//
	// Emitted by internal/builders/validations.IsLegalForType callers.
	CodeShapeMismatch Code = "validate.shape-mismatch"

	// CodeAmbiguousEmbed fires when two embedded types of a parent struct (or struct embed-chains at
	// the same depth) both promote a property with the same JSON name but different Go names.
	//
	// Go's own rule is to not promote such ambiguous fields; codescan currently emits a
	// last-write-wins schema regardless.
	// The diagnostic surfaces the case so authors can disambiguate.
	CodeAmbiguousEmbed Code = "validate.ambiguous-embed"

	// CodeUnsupportedInSimpleSchema fires when the schema builder running in SimpleSchema mode
	// produces an outcome that OAS v2 does not allow on a parameter/header (object type, $ref, allOf,
	// properties, …).
	//
	// The diagnostic is emitted at exit and the target is reset to empty `{}` — honest over lossy.
	// Reaching this code path typically means a non-body parameter or response header was typed as a
	// struct or interface that the recognizer cascade couldn't reduce to a primitive.
	CodeUnsupportedInSimpleSchema Code = "validate.unsupported-in-simple-schema"

	// CodeUnsupportedType fires when a `swagger:type` argument cannot be resolved to an inline schema:
	// an unknown type name, a `file` override (use swagger:file instead), or a keyword used where it
	// is not valid (e.g. `inline`/`array` as an array element).
	//
	// The override is dropped and the subject falls through to its Go type.
	// See the F3 reconciliation in .claude/plans/quirks-F-series-fix.md.
	CodeUnsupportedType Code = "validate.unsupported-type"

	// CodeDeprecated fires when an accepted-but-deprecated annotation or keyword value is used (the
	// input is still processed).
	//
	// Carries a migration hint in the message — e.g. `swagger:type array` → `inline`, or the
	// deprecated `swagger:alias` annotation.
	CodeDeprecated Code = "validate.deprecated"

	// CodeUnsupportedGoType fires when a Go type, a `go/types` kind, or a builtin cannot be translated
	// to a Swagger 2.0 construct and is therefore dropped from the spec.
	//
	// The scanner runs on arbitrary user code, so an unmodeled construct must not panic — it is
	// skipped and surfaced as a Warning (real data loss, but the scan continues).
	// The message names the construct (and the dispatch site) so a future go/types evolution surfaces
	// one grep-able diagnostic instead of vanishing behind a silent default.
	CodeUnsupportedGoType Code = "validate.unsupported-go-type"

	// CodeDuplicateModelName fires when two distinct Go types in the SAME package claim the same
	// definition name (necessarily via a `swagger:model <name>` override, since Go type names are
	// unique per package).
	//
	// The first declaration keeps the name; later ones fall back to their Go type name.
	// See the name-identity design D-4.
	CodeDuplicateModelName Code = "validate.duplicate-model-name"

	// CodeCollidingModelName fires when the same definition name is declared across SEVERAL packages.
	//
	// The reduce stage keeps each distinct by qualifying the colliding ones with a PascalCase
	// package-prefix concat (e.g. b.Test / c.Test -> BTest / CTest); the author can force a specific
	// name with `swagger:model <name>`.
	// See the name-identity design (D-8).
	CodeCollidingModelName Code = "validate.colliding-model-name"

	// CodeHierarchicalModelName fires when a colliding definition name's best flat concat exceeds the
	// readability budget and the caller enabled EmitHierarchicalNames: the reduce stage emits nested
	// container definitions (`#/definitions/<pkg>/<Name>`) instead of a long flat concat.
	//
	// The author can force a flat name with `swagger:model <name>`.
	// See the name-identity design (rung 3 / W2).
	CodeHierarchicalModelName Code = "validate.hierarchical-model-name"

	// CodeAmbiguousTypeName fires when a type-name keyword argument (swagger:type,
	// swagger:additionalProperties, swagger:patternProperties) names a bare leaf that, after failing
	// to resolve in the builder's own package, matches a discovered model in SEVERAL packages.
	//
	// The reference is ambiguous so it is dropped; the author can disambiguate with a same-package
	// type or a swagger:model override.
	// See the name-identity leaf-resolution design.
	CodeAmbiguousTypeName Code = "validate.ambiguous-type-name"

	// CodeDegradedLoad fires when `packages.Load` returns a degraded result.
	//
	// It is tiered by what is still recoverable: an Error (aborting) when nothing usable loaded — no
	// packages matched, a package could not be loaded at all, or its type information (`Types` /
	// `TypesInfo`) is unavailable (the #2874 case where swagger:allOf silently stops resolving); a
	// Warning (non-fatal) when a package carries only parse/type errors but its type information is
	// still usable, so a single non-building package does not sink a whole `./...` scan.
	//
	// See go-swagger/go-swagger#2874.
	CodeDegradedLoad Code = "scan.degraded-load"

	// CodeInternalPanic fires when a builder panics while processing a single declaration.
	//
	// The scan recovers, names the offending source declaration (file:line), and aborts with a located
	// error rather than surfacing a raw Go stack trace.
	// See go-swagger/go-swagger#2886.
	CodeInternalPanic Code = "scan.internal-panic"

	// CodeIgnoredByRules fires when a package is skipped because it does not pass the caller's
	// Include/Exclude package rules.
	//
	// Informational (Hint): the omission is the caller's own configuration, surfaced to aid "why is my
	// package missing" triage.
	CodeIgnoredByRules Code = "scan.ignored-by-rules"

	// CodeIgnoredByTag fires when a route or operation is skipped because its tags do not pass the
	// caller's IncludeTags/ExcludeTags rules.
	//
	// Informational (Hint), like CodeIgnoredByRules.
	CodeIgnoredByTag Code = "scan.ignored-by-tag"

	// CodeDroppedRefSibling fires when SkipAllOfCompounding is set and a $ref'd struct field carries
	// sibling decoration (description, validations, vendor extensions, externalDocs) that cannot ride
	// a bare $ref.
	//
	// With compounding disabled the field emits as a bare $ref and each such sibling is dropped —
	// one diagnostic per dropped keyword so the loss is never silent.
	// See scanner.Options SkipAllOfCompounding.
	CodeDroppedRefSibling Code = "validate.dropped-ref-sibling"

	// CodePrunedUnused fires when PruneUnusedModels is set and a discovered definition is dropped
	// because it is not transitively referenced from any path, shared response, shared parameter or
	// overlay definition.
	//
	// Carries the originating Go type's source position so the loss is never silent.
	// Informational (Hint): the prune is the caller's own opt-in, surfaced to aid "why is my model
	// missing" triage.
	// See scanner.Options PruneUnusedModels.
	CodePrunedUnused Code = "scan.pruned-unused"

	// CodeRenamedDefinition fires when the reduce stage renames a definition to deconflict a
	// cross-package name collision (e.g. b.Test / c.Test -> BTest / CTest), so a consumer that tracks
	// source <-> spec links (the genspec TUI) learns the final name a Go type landed under.
	//
	// Informational (Hint); carries the Go type's source position.
	// The bare-leaf zero-churn case (a globally unique name lifted to its leaf) is NOT reported —
	// only true renames.
	CodeRenamedDefinition Code = "scan.renamed-definition"

	// CodeSharedParameterConflict fires when two `swagger:parameters *` declarations register the same
	// shared parameter short name (#/parameters/{name}).
	//
	// Shared parameters are referenced only by short name, so — unlike definitions — they are
	// never renamed: the first registration is kept, later ones are dropped.
	// Warning, so the shadowed declaration is never lost silently.
	// §2.
	CodeSharedParameterConflict Code = "scan.shared-parameter-conflict"

	// CodeSharedResponseConflict fires when two `swagger:response` declarations register the same
	// top-level response short name (#/responses/{name}).
	//
	// Like shared parameters, responses are referenced only by short name and are never renamed: the
	// first registration is kept, later ones are dropped.
	// Warning.
	//
	// An InputSpec (overlay) response of the same name is not a conflict — a scanned struct extends
	// it.
	CodeSharedResponseConflict Code = "scan.shared-response-conflict"

	// CodeDanglingParameterRef fires when a `swagger:parameters` reference names a shared parameter
	// that no `swagger:parameters *` declaration registered (#/parameters/{name} does not exist).
	//
	// The reference is dropped rather than emitting a dangling $ref.
	// Warning.
	// §1b.
	CodeDanglingParameterRef Code = "scan.dangling-parameter-ref"

	// CodeDanglingResponseRef fires when an operation references a shared response
	// (#/responses/{name}) that no `swagger:response` declaration registered — e.g. a `$ref` in a
	// swagger:operation wholesale-YAML body pointing at an unknown response.
	//
	// The reference is dropped rather than emitting a dangling $ref.
	// Warning.
	CodeDanglingResponseRef Code = "scan.dangling-response-ref"

	// CodeDuplicateTarget fires when a `swagger:parameters * opid …` marker repeats an operation id;
	// the duplicate is dropped.
	//
	// Warning (C1).
	CodeDuplicateTarget Code = "scan.duplicate-target"

	// CodeDuplicateRef fires when a `swagger:parameters {target} name …` reference repeats a
	// shared-parameter name; the duplicate is dropped.
	//
	// Warning (C2).
	CodeDuplicateRef Code = "scan.duplicate-ref"

	// CodeEmptyOverride fires when a `swagger:description` / `swagger:title` override annotation
	// resolves to an empty value (bare marker, or a whitespace/blank-only body).
	//
	// The empty value is still applied — empty is the deliberate godoc-suppression affordance —
	// but the case is flagged in case it was accidental (a leftover marker).
	// Warning.
	// (D7).
	CodeEmptyOverride Code = "scan.empty-override"
)

// Diagnostic is one observation about a comment block.
type Diagnostic struct {
	Pos      token.Position
	Severity Severity
	Code     Code
	Message  string
}

// String renders a Diagnostic in compiler-style one-line form.
func (d Diagnostic) String() string {
	loc := d.Pos.String()
	if loc == "-" || loc == "" {
		loc = "<unknown>"
	}
	return fmt.Sprintf("%s: %s: %s [%s]", loc, d.Severity, d.Message, d.Code)
}

// Errorf builds a SeverityError Diagnostic with a formatted message.
func Errorf(pos token.Position, code Code, format string, args ...any) Diagnostic {
	return Diagnostic{Pos: pos, Severity: SeverityError, Code: code, Message: fmt.Sprintf(format, args...)}
}

// Warnf builds a SeverityWarning Diagnostic.
func Warnf(pos token.Position, code Code, format string, args ...any) Diagnostic {
	return Diagnostic{Pos: pos, Severity: SeverityWarning, Code: code, Message: fmt.Sprintf(format, args...)}
}

// Hintf builds a SeverityHint Diagnostic.
func Hintf(pos token.Position, code Code, format string, args ...any) Diagnostic {
	return Diagnostic{Pos: pos, Severity: SeverityHint, Code: code, Message: fmt.Sprintf(format, args...)}
}
