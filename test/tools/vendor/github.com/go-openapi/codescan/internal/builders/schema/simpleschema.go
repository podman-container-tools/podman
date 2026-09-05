// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/types"

	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	oaispec "github.com/go-openapi/spec"
)

// inFormData is the `in:` parameter-location value that enables the `file` type under SimpleSchema.
//
// Lifted into a constant to satisfy goconst across this file and walker_classifiers.go.
const inFormData = "formData"

// inBody is the `in:` value that marks a full-Schema position: the one place an empty schema ("any
// JSON") is a legal answer, and therefore the one this file's repairs must leave alone.
const inBody = "body"

// SimpleSchemaProbe is the schema-builder-side contract a SimpleSchema target must satisfy.
//
// Implemented structurally by `paramTypable` and (M2) `headerTypable`.
//
// # Details
//
// See [§simple-schema-mode](./README.md#simple-schema-mode) — full interface contract, reset
// semantics, and the catch-at-exit validator's role.
type SimpleSchemaProbe interface {
	SimpleSchemaShape() *oaispec.SimpleSchema
	HasRef() bool
	ResetForViolation()
}

// validateSimpleSchemaOutcome runs the "catch at exit" contract: inspect the resolved target,
// accept SimpleSchema-legal outcomes, emit a diagnostic and reset on violation.
//
// # Details
//
// See [§simple-schema-mode](./README.md#simple-schema-mode) for the allowed-type set, the
// file/formData special case, and the honest-over-lossy reset rationale.
func (s *Builder) validateSimpleSchemaOutcome() {
	probe, ok := s.target.(SimpleSchemaProbe)
	if !ok {
		// Target doesn't expose a SimpleSchema shape — caller chose the SimpleSchema mode for a target
		// that can't surface a violation.
		// Trust the caller; emit no diagnostic.
		return
	}

	shape := probe.SimpleSchemaShape()
	if shape == nil {
		return
	}

	refViolation := probe.HasRef()

	// An empty type is not a resolution, it is the absence of one — and OAS v2 requires a type on every
	// non-body parameter and response header, so emitting nothing is invalid rather than permissive.
	// `any` and `json.RawMessage` are the usual sources: "any JSON" answers a body or a definition and
	// answers nothing here.
	if shape.Type == "" && !refViolation {
		s.RecordDiagnostic(grammar.Warnf(
			s.Ctx.PosOf(s.Decl.Pos()),
			grammar.CodeUnderspecifiedInSimpleSchema,
			"non-body parameter / response header (in=%q) resolved to no type, which OAS v2 does not allow; "+
				"defaulted to {type: string}",
			s.paramIn,
		))
		EnsureSimpleSchemaTyped(s.target, s.inputType, s.skipExtensions)

		return
	}

	if isAllowedSimpleSchemaType(shape.Type, s.paramIn) && !refViolation {
		return
	}

	reason := simpleSchemaViolationReason(shape.Type, refViolation, s.paramIn)
	s.RecordDiagnostic(grammar.Warnf(
		s.Ctx.PosOf(s.Decl.Pos()),
		grammar.CodeUnsupportedInSimpleSchema,
		"non-body parameter / response header (in=%q) cannot be represented as an OAS v2 SimpleSchema: %s; "+
			"target defaulted to {type: string}",
		s.paramIn, reason,
	))
	probe.ResetForViolation()
	if refViolation {
		// MakeRef discovered the now-dissolved target's decl; drop it so it doesn't linger as an orphan
		// definition (go-swagger#1088).
		s.ResetPostDeclarations()
	}
	EnsureSimpleSchemaTyped(s.target, s.inputType, s.skipExtensions)
}

// EnsureSimpleSchemaTyped gives a non-body parameter or a response header a type when its resolution
// left it without one, and reports whether it had to.
//
// OAS v2 makes `type` required on both, so an empty outcome is not a permissive answer there but an
// invalid one. `string` is the only type that can carry an unknown payload where the transport is
// text: a query string, a path segment and a header are percent-encoded text, so `binary` would claim
// octets the position cannot hold and `byte` a base64 framing nobody applied.
//
// No format is invented — the whole reason for being here is that the Go type said nothing — but an
// already-resolved one survives, so a `swagger:strfmt` that landed a format without a type keeps it.
// x-go-type records what the fallback erased, since `string` cannot tell a JSON document from a plain
// one; it is an extension like any other and stays under the SkipExtensions umbrella.
//
// Exported because the exit validator is not the only place a SimpleSchema target is resolved: the
// parameters and responses builders answer for a recognized stdlib type in their own field arms and
// return without ever entering [Builder.Build], so the catch-at-exit contract never sees them. `any`
// and `json.RawMessage` reach a parameter that way.
//
// The caller reports — it owns the position it is building and can say which field is affected.
//
// # Details
//
// See [§simple-schema-mode](./README.md#simple-schema-mode).
func EnsureSimpleSchemaTyped(target ifaces.SwaggerTypable, inputType types.Type, skipExt bool) (repaired bool) {
	if target == nil || target.In() == inBody {
		return false
	}

	probe, ok := target.(SimpleSchemaProbe)
	if !ok {
		return false
	}

	shape := probe.SimpleSchemaShape()
	if shape == nil || shape.Type != "" {
		return false
	}

	if !skipExt {
		if goType := simpleSchemaGoType(inputType); goType != "" {
			target.AddExtension("x-go-type", goType)
		}
	}
	target.Typed("string", shape.Format)

	return true
}

// simpleSchemaGoType names a Go type in the `x-go-type` spelling the recognizers use: `<pkg path>.<name>`
// for a declared type and the bare name for a predeclared one (`any`), matching what `recognizeError`
// records for `error`.
//
// Returns "" when there is no name worth recording — a composite or a literal, where the extension would
// say less than the schema already does.
func simpleSchemaGoType(t types.Type) string {
	var obj *types.TypeName

	switch tpe := t.(type) {
	case *types.Pointer:
		return simpleSchemaGoType(tpe.Elem())
	case *types.Alias:
		// Before Unalias: `any` is an alias whose object carries the name worth recording, and peeling it
		// would leave an anonymous empty interface.
		obj = tpe.Obj()
	case *types.Named:
		obj = tpe.Obj()
	default:
		return ""
	}

	if obj.Pkg() == nil {
		return obj.Name()
	}

	return obj.Pkg().Path() + "." + obj.Name()
}

// isAllowedSimpleSchemaType reports whether t is a SimpleSchema-legal type given the caller's `in`
// location.
//
// The empty string is NOT accepted: OAS v2 makes `type` required on a non-body parameter and on a
// response header, so "any" has no spelling here. Its callers handle empty ahead of this check and
// default it, see [Builder.validateSimpleSchemaOutcome].
//
// `file` is only valid for in == inFormData.
func isAllowedSimpleSchemaType(t, in string) bool {
	switch t {
	case "string", "number", "integer", "boolean", "array":
		return true
	case "file":
		return in == inFormData
	}
	return false
}

// simpleSchemaViolationReason produces a short human-readable cause for the diagnostic message.
func simpleSchemaViolationReason(t string, refViolation bool, in string) string {
	switch {
	case refViolation:
		return "$ref / model reference is forbidden under SimpleSchema"
	case t == "file" && in != inFormData:
		return `type "file" is only valid when in: formData`
	case t == "object":
		return `type "object" is forbidden under SimpleSchema`
	case t == "":
		// Should never get here — an empty type is handled by its own branch, ahead of this one.
		// Defensive only.
		return "no type could be resolved"
	}
	return "type " + t + " is not in the allowed SimpleSchema set"
}
