// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/types"

	"github.com/go-openapi/codescan/internal/ifaces"
)

// resolveRefOr looks tio up in the model index.
//
// On hit, emits a $ref to it via the inherited MakeRef.
// On miss, runs orElse (or returns nil when orElse is nil).
// Used by every named-shape leaf that follows the "FindModel → MakeRef, else fallback" pattern.
func (s *Builder) resolveRefOr(tio *types.TypeName, tgt ifaces.SwaggerTypable, orElse func() error) error {
	if decl, ok := s.Ctx.GetModel(tio.Pkg().Path(), tio.Name()); ok {
		return s.MakeRef(decl, tgt)
	}
	if orElse == nil {
		return nil
	}
	return orElse()
}

// resolveRefOrErr is the strict counterpart of resolveRefOr: the FindModel miss is treated as a
// missingSource error rather than a silent fallback. errTpe is the type to format into the error
// message (typically the underlying rather than the *types.Named, so the diagnostic points at the
// structural shape the caller actually expected to resolve).
func (s *Builder) resolveRefOrErr(tio *types.TypeName, tgt ifaces.SwaggerTypable, errTpe types.Type) error {
	if decl, ok := s.Ctx.GetModel(tio.Pkg().Path(), tio.Name()); ok {
		return s.MakeRef(decl, tgt)
	}
	return missingSource(errTpe)
}
