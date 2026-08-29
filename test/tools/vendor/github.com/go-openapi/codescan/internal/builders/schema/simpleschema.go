// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	oaispec "github.com/go-openapi/spec"
)

// inFormData is the `in:` parameter-location value that enables the `file` type under SimpleSchema.
//
// Lifted into a constant to satisfy goconst across this file and walker_classifiers.go.
const inFormData = "formData"

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

	typeOK := isAllowedSimpleSchemaType(shape.Type, s.paramIn)
	refViolation := probe.HasRef()
	if typeOK && !refViolation {
		return
	}

	reason := simpleSchemaViolationReason(shape.Type, refViolation, s.paramIn)
	s.RecordDiagnostic(grammar.Warnf(
		s.Ctx.PosOf(s.Decl.Spec.Pos()),
		grammar.CodeUnsupportedInSimpleSchema,
		"non-body parameter / response header (in=%q) cannot be represented as an OAS v2 SimpleSchema: %s; target reset to empty {}",
		s.paramIn, reason,
	))
	probe.ResetForViolation()
	if refViolation {
		// MakeRef discovered the now-dissolved target's decl; drop it so it doesn't linger as an orphan
		// definition (go-swagger#1088).
		s.ResetPostDeclarations()
	}
}

// isAllowedSimpleSchemaType reports whether t is a SimpleSchema-legal type given the caller's `in`
// location.
//
// Empty string means "any" and is accepted (e.g. json.RawMessage recognizer emits an empty schema).
// `file` is only valid for in == inFormData.
func isAllowedSimpleSchemaType(t, in string) bool {
	switch t {
	case "", "string", "number", "integer", "boolean", "array":
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
		// Should never get here — empty type is accepted by isAllowedSimpleSchemaType.
		// Defensive only.
		return "empty type unexpectedly rejected"
	}
	return "type " + t + " is not in the allowed SimpleSchema set"
}
