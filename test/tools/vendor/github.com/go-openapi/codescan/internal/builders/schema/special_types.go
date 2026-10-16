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
	// Implicit recognizers in priority order: certain (identity) before guessed (fuzzy name).
	//
	// recognizeMathBig belongs here as much as in the canonical set: a *big.Int / *big.Float / *big.Rat
	// field satisfies TextMarshaler, so it is diverted here before any call site reaches
	// [ApplyStdlibSpecials]. Without it the same math/big type renders one way through a pointer and
	// another by value. See [§math-big](./README.md#math-big).
	if applySpecialType(tio, target, s.skipExtensions,
		recognizeError, recognizeTime, recognizeRawMessage, recognizeStdUUID, recognizeMathBig,
		recognizeUUID) {
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
	// recognizeStdUUID is an identity match on the go1.27 stdlib uuid.UUID.
	//
	// Safe everywhere, so it lives in the canonical set applied by [ApplyStdlibSpecials].
	// See [§special-types](./README.md#special-types).
	recognizeStdUUID
	// recognizeUUID is a fuzzy name-only match (case-insensitive "uuid").
	//
	// Caller-gated — opt in only where the type is guaranteed to render as text.
	// See [§special-types](./README.md#special-types).
	recognizeUUID
	// recognizeOpaqueStream is an identity match on the known byte-stream carriers (io.Reader and
	// friends, multipart.File, runtime.NamedReadCloser).
	//
	// Safe everywhere, so it lives in the canonical set applied by [ApplyStdlibSpecials].
	// See [§opaque-streams](./README.md#opaque-streams).
	recognizeOpaqueStream

	// recognizeMathBig is an identity match on the arbitrary-precision numbers big.{Int,Float,Rat}.
	//
	// Safe everywhere, so it lives in the canonical set applied by [ApplyStdlibSpecials] — and, unlike
	// the rest of that set, it is ALSO passed by [Builder.buildFromTextMarshal], because all three
	// types are reached through the TextMarshaler shortcut whenever the field is a pointer.
	// See [§math-big](./README.md#math-big).
	recognizeMathBig
)

// ApplyStdlibSpecials runs the canonical safe set of identity-based recognizers.
//
// Examples: any, [time.Time], [error], [json.RawMessage], [uuid.UUID] (go1.27), the opaque byte
// streams and the math/big numbers.
//
// Safe at every call site that handles a *types.TypeName. Exported for the parameters and responses
// builders, which reach it from their own field arms: each used to carry a hand-rolled subset of
// these, differing per function, and the subsets drifted.
//
// Call it BEFORE any declaration lookup. Every recognizer here answers from the object's identity
// alone, so requiring a declaration first subordinates a rule that needs nothing to one that can
// fail — and for the predeclared `error` it always fails, since a predeclared object has no package
// and therefore no declaring source to find.
//
// What this set does NOT hold is as much a decision as what it does. A recognizer asserts that a type
// means one thing on the wire wherever it appears, and it may only be added where that is true of
// every use. time.Duration is the standing counter-example: it is a count of nanoseconds, and whether
// an API means nanoseconds, seconds or an ISO 8601 string is the author's choice rather than a
// property of the type — which is why go-openapi publishes strfmt.Duration, carrying a stated wire
// format, instead of teaching this set to guess at one. io.Writer is kept out of the opaque-stream
// table for the same reason.
//
// So a consumed type with no recognizer is reported rather than resolved by assumption: the scanner
// says its declaration could not be read, and the author says what they meant with a strfmt type or
// an annotation.
//
// # Details
//
// See [§special-types](./README.md#special-types).
func ApplyStdlibSpecials(obj *types.TypeName, target ifaces.SwaggerTypable, skipExt bool) bool {
	return applySpecialType(obj, target, skipExt,
		recognizeAny, recognizeTime, recognizeError, recognizeRawMessage, recognizeStdUUID,
		recognizeOpaqueStream, recognizeMathBig)
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
		if resolved := applySpecialTypeForKey(obj, target, skipExt, typeKey); resolved {
			return true
		}
	}

	return false
}

func applySpecialTypeForKey(obj *types.TypeName, target ifaces.SwaggerTypable, skipExt bool, typeKey recognizeType) (resolved bool) {
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

	case recognizeStdUUID: // identity — go1.27 stdlib uuid.UUID.
		if resolvers.IsStdUUID(obj) {
			target.Typed("string", "uuid")
			return true
		}

	case recognizeMathBig: // identity — see [§math-big](./README.md#math-big).
		// Each answer is the shape encoding/json actually produces and accepts, which is not the same
		// for the three: big.Int has a MarshalJSON (a bare numeric literal, and json.Marshaler beats
		// the MarshalText it also carries), while Float and Rat have only MarshalText and therefore
		// travel quoted — "3.5" and "5/3". Reading them as numbers because they are named Float and
		// Rat would publish a spec that round-trips through nothing.
		//
		// x-go-type in every arm, because each rendering erases the type: `integer` cannot say the
		// value is unbounded rather than an int64, and `string` cannot tell a decimal float from a
		// quotient. That is the `recognizeError` criterion — see
		// [§traceability](./README.md#traceability).
		//
		// No format in either arm: the precision is unbounded by construction, so int64 / double
		// would be a narrower claim than the type makes.
		var bigType string
		switch {
		case resolvers.IsStdBigInt(obj):
			bigType = "integer"
		case resolvers.IsStdBigFloat(obj), resolvers.IsStdBigRat(obj):
			bigType = "string"
		default:
			return false
		}

		if !skipExt {
			target.AddExtension("x-go-type", obj.Pkg().Path()+"."+obj.Name())
		}
		target.Typed(bigType, "")

		return true

	case recognizeOpaqueStream: // identity — see [§opaque-streams](./README.md#opaque-streams).
		if resolvers.IsOpaqueStream(obj) {
			// x-go-type records which stream this was, because neither answer below can: `byte` says
			// base64 bytes and `file` says an upload, and every recognized type collapses onto the
			// same schema either way. That is the `recognizeError` criterion — stamp when the
			// rendering erases the type — and not the `time.Time` / uuid one, where the format IS
			// the type. See [§traceability](./README.md#traceability).
			if !skipExt {
				target.AddExtension("x-go-type", obj.Pkg().Path()+"."+obj.Name())
			}
			// The only two answers a stream can honestly take, chosen by what the position permits
			// rather than by anything in the declaration. `file` is legal on a formData parameter and
			// nowhere else in OAS 2.0, and it is the canonical upload shape; everywhere else the
			// stream is base64 in a string, which is how OAS 2.0 spells "opaque bytes".
			if target.In() == inFormData {
				target.Typed("file", "")
				return true
			}
			target.Typed("string", "byte")
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

	return false
}
