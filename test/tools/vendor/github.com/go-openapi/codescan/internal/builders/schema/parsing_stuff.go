// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/ast"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
)

func (s *Builder) InferNames() {
	s.inferNames()
}

// findAnnotation returns the first Block in the ParseBlocks slice
// matching kind, or nil. Used by discovery sites (e.g. inferNames)
// where the relevant annotation may not be the first in source
// order — multi-annotation comments like
// `swagger:type … / swagger:model objectStruct` need ParseAll's
// per-annotation Block to surface the model override.
//
//nolint:ireturn // Block interface is the documented return type.
func (s *Builder) findAnnotation(cg *ast.CommentGroup, kind grammar.AnnotationKind) grammar.Block {
	for _, b := range s.ParseBlocks(cg) {
		if b.AnnotationKind() == kind {
			return b
		}
	}
	return nil
}

func (s *Builder) inferNames() {
	if s.GoName != "" {
		return
	}

	goName := s.Decl.Ident.Name
	s.GoName = goName
	s.Name = goName

	// Read the model annotation directly off the cached parsed Blocks. findAnnotation walks every
	// annotation in the comment group, so multi-annotation comments (e.g. `swagger:type` +
	// `swagger:model objectStruct`) still surface the model override regardless of source order.
	//
	// AnnotationArg() carries the IDENT_NAME when one was given; bare `swagger:model` keeps the Go
	// identifier as the schema name.
	model := s.findAnnotation(s.Decl.Comments, grammar.AnnModel)
	if model == nil {
		return
	}

	s.annotated = true
	// A same-package duplicate has its override suppressed (D-4): keep the Go name so the x-go-name
	// extension and definition key stay consistent with EntityDecl.Names/DefKey.
	if override, ok := model.AnnotationArg(); ok && !s.Decl.ModelOverrideSuppressed() {
		s.Name = override
	}
}
