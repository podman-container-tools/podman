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

	// A miss with nothing to fall back on is the whole definition going missing — the property comes out
	// bare, carrying its name and nothing else. Worth more than a Hint when the cause is a package the
	// load did not read: under an ordinary scan this same type is discovered and $ref'd, so unlike the
	// thinner-schema cases the difference here is a definition that exists or does not.
	//
	// Only the no-fallback arm asks. Where orElse renders the type structurally the definition is not
	// lost, and the miss is the ordinary "this type is not a model, inline it" outcome that fires on
	// every scan.
	if orElse == nil {
		s.SourcelessFallback(tio)

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
	// An interface with no declaration to read renders as the open schema, which is what its structural
	// form says on its own: any object satisfies it. Strictly less than the $ref a readable declaration
	// would have produced, and strictly more useful than losing the document.
	if s.SourcelessFallback(tio) {
		return nil
	}

	return missingSource(errTpe)
}
