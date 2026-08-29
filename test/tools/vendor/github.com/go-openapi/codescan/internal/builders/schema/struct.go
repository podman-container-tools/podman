// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/types"

	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// buildFromStruct emits the schema for a named Go struct.
//
// A first pass scans anonymous embeds for allOf composition; the second pass fills the property map
// from exported fields.
// The user-classifier short-circuit (`swagger:type`) wins outright.
//
// # Details
//
// See [§struct](./README.md#struct) — two-pass shape, the target-vs-schema split when allOf is
// in play, and why the `target.Typed("object", "")` line always fires (no SimpleSchema-style early
// exit yet).
func (s *Builder) buildFromStruct(decl *scanner.EntityDecl, st *types.Struct, schema *oaispec.Schema, nameByJSON map[string]propOwner) error {
	if s.classifierStructPreBuildType(decl.Comments, NewTypable(schema, 0, s.skipExtensions)) {
		return nil
	}

	target, hasAllOf, err := s.scanEmbeddedFields(decl, st, schema, nameByJSON)
	if err != nil {
		return err
	}

	if target == nil {
		if schema != nil {
			target = schema
		} else {
			target = &oaispec.Schema{}
		}
	}

	if target.Properties == nil {
		target.Properties = make(map[string]oaispec.Schema)
	}
	target.Typed("object", "")

	// Cross-ref linkage: when own fields land in a fresh allOf member (target diverges from schema),
	// their pointer is /allOf/{k}/properties/… — not the tracked base — so clear the path and
	// let them resolve to schema's anchor.
	// Plain structs and plain embeds keep target == schema and anchor directly.
	if target != schema {
		defer s.repath("")()
	}

	if err := s.buildStructFields(decl, st, target, nameByJSON); err != nil {
		return err
	}

	if hasAllOf && len(target.Properties) > 0 {
		schema.AllOf = append(schema.AllOf, *target)
	}

	return nil
}

func (s *Builder) buildStructFields(decl *scanner.EntityDecl, st *types.Struct, target *oaispec.Schema, nameByJSON map[string]propOwner) error {
	for fld := range st.Fields() {
		if err := s.processStructField(fld, decl, target, nameByJSON); err != nil {
			return err
		}
	}

	return nil
}

func (s *Builder) processStructField(fld *types.Var, decl *scanner.EntityDecl, target *oaispec.Schema, nameByJSON map[string]propOwner) error {
	c, ok, err := s.structFieldCarrier(fld, decl, target, nameByJSON)
	if err != nil || !ok {
		return err
	}
	return s.applyFieldCarrier(c, target, nameByJSON)
}
