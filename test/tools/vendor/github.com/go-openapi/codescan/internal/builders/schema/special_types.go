// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/types"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/ifaces"
)

// buildFromTextMarshal renders a TextMarshaler-implementing type as a string.
//
// Six-step pipeline; user-annotation first, implicit recognizers next, generic fallback last.
//
// # Details
//
// See [§textmarshal-order](./README.md#textmarshal-order).
func (s *Builder) buildFromTextMarshal(tpe types.Type, target ifaces.SwaggerTypable) error {
	if typePtr, ok := tpe.(*types.Pointer); ok {
		return s.buildFromTextMarshal(typePtr.Elem(), target)
	}
	// Aliases route through buildAlias so RefAliases / TransparentAliases stay in charge of the alias
	// indirection.
	if typeAlias, ok := tpe.(*types.Alias); ok {
		return s.buildAlias(typeAlias, target)
	}

	typeNamed, ok := tpe.(*types.Named)
	if !ok {
		target.Typed("string", "")
		return nil
	}
	tio := typeNamed.Obj()

	// Explicit user annotation wins over implicit recognizers.
	if s.classifierTextMarshal(typeNamed, target) {
		return nil
	}
	// Implicit recognizers in priority order.
	if applySpecialType(tio, target, s.skipExtensions, recognizeError, recognizeTime, recognizeRawMessage, recognizeUUID) {
		return nil
	}
	// Generic fallback: x-go-type carries pkg.Name, so PkgForType-miss must bail (can't produce the
	// extension without the package).
	if _, found := s.Ctx.PkgForType(tpe); !found {
		return nil
	}
	target.Typed("string", "")
	target.AddExtension("x-go-type", tio.Pkg().Path()+"."+tio.Name())
	return nil
}

type recognizeType uint8

const (
	recognizedNone recognizeType = iota
	recognizeTime
	recognizeAny
	recognizeError
	recognizeRawMessage
	// recognizeUUID is a fuzzy name-only match (case-insensitive "uuid").
	//
	// Caller-gated — opt in only where the type is guaranteed to render as text.
	// See [§special-types](./README.md#special-types).
	recognizeUUID
)

// applyStdlibSpecials runs the canonical safe set of identity-based recognizers (any / time.Time /
// error / json.RawMessage).
//
// Safe at every call site that handles a *types.TypeName.
//
// # Details
//
// See [§special-types](./README.md#special-types).
func applyStdlibSpecials(obj *types.TypeName, target ifaces.SwaggerTypable, skipExt bool) bool {
	return applySpecialType(obj, target, skipExt,
		recognizeAny, recognizeTime, recognizeError, recognizeRawMessage)
}

// applySpecialType iterates wanted recognizers in order and applies the first match to target,
// returning resolved=true.
//
// Recognizers are identity-based except recognizeUUID, which is fuzzy (caller-gated). skipExt gates
// vendor-extension writes.
//
// # Details
//
// See [§special-types](./README.md#special-types), [§user-overrides](./README.md#user-overrides)
// (skipExt plumbing) and [§quirks](./README.md#quirks) (per-recognizer rationale).
func applySpecialType(obj *types.TypeName, target ifaces.SwaggerTypable, skipExt bool, wanted ...recognizeType) (resolved bool) {
	for _, typeKey := range wanted {
		switch typeKey {
		case recognizeTime: // special case of the "time.Time" type
			if resolvers.IsStdTime(obj) {
				target.Typed("string", "date-time")

				return true
			}

		case recognizeAny: // e.g type X any or type X interface{}
			if resolvers.IsAny(obj) {
				_ = target.Schema()

				return true
			}

		case recognizeError: // predeclared error; see [§quirks](./README.md#quirks) for x-go-type rationale.
			if resolvers.IsStdError(obj) {
				if !skipExt {
					target.AddExtension("x-go-type", obj.Name())
				}
				target.Typed("string", "")
				return true
			}

		case recognizeRawMessage: // json.RawMessage; see [§quirks](./README.md#quirks) for the "any" rationale.
			if resolvers.IsStdJSONRawMessage(obj) {
				_ = target.Schema()
				return true
			}

		case recognizeUUID: // fuzzy — see [§special-types](./README.md#special-types).
			if obj != nil && strings.ToLower(obj.Name()) == "uuid" {
				target.Typed("string", "uuid")
				return true
			}

		default:
			// ignored
		}
	}

	return false
}
