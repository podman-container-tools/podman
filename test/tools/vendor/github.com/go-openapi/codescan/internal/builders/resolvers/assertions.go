// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package resolvers

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"

	oaispec "github.com/go-openapi/spec"
)

type internalError string

func (e internalError) Error() string {
	return string(e)
}

const (
	ErrInternal internalError = "internal error due to a bug or a mishandling of go types AST. This usually indicates a bug in the scanner"
)

// code assertions to be explicit about the various expectations when entering a function

func MustNotBeABuiltinType(o *types.TypeName) {
	if o != nil && o.Pkg() != nil {
		return
	}

	var name string
	if o != nil {
		name = o.Name()
	} else {
		name = "<nil>"
	}

	panic(fmt.Errorf("type %q expected not to be a builtin: %w", name, ErrInternal))
}

func MustHaveRightHandSide(a *types.Alias) {
	if a.Rhs() != nil {
		return
	}

	panic(fmt.Errorf("type alias %q expected to declare a right-hand-side: %w", a.Obj().Name(), ErrInternal))
}

// IsFieldStringable check if the field type is a scalar.
//
// If the field type is *ast.StarExpr and is pointer type, check if it refers to a scalar.
// Otherwise, the ",string" directive doesn't apply.
func IsFieldStringable(tpe ast.Expr) bool {
	if ident, ok := tpe.(*ast.Ident); ok {
		switch ident.Name {
		case "int", "int8", "int16", "int32", "int64",
			"uint", "uint8", "uint16", "uint32", "uint64",
			"float64", "string", "bool":
			return true
		}
	} else if starExpr, ok := tpe.(*ast.StarExpr); ok {
		return IsFieldStringable(starExpr.X)
	} else {
		return false
	}
	return false
}

// textMarshalerIface is the encoding.TextMarshaler interface:
//
//	interface{ MarshalText() (text []byte, err error) }
//
// It is resolved by type-checking a one-line source snippet rather than by importing the real
// "encoding" package through go/importer's default importer.
//
// The importer reads stdlib export data out of the GOROOT the binary was built against:
// when GOTOOLCHAIN selects a different toolchain at runtime — or the callgin binary simply runs on a machine where
// Go installation lives at a different path than the build machine's — the lookup fails, and the error is silently
// swallowed: every TextMarshaler check returns false.
//
// This snippet imports nothing (the result types []byte and error are Universe types), so the type-checker never
// consults an importer. The construction has no runtime dependency on a working toolchain or GOROOT.
//
// types.Implements compares method sets structurally: the resulting interface is equivalent to the one the "encoding" package supplies.
var textMarshalerIface = mustIfaceFromSource( //nolint:gochecknoglobals // immutable, built once, read-only
	`package p; type T interface{ MarshalText() (text []byte, err error) }`,
)

// mustIfaceFromSource type-checks src: it must declare a single interface type T that imports nothing.
//
// It returns its underlying *types.Interface. It panics on malformed input: src is a compile-time constant
// (a failure is a programming error never a runtime condition).
func mustIfaceFromSource(src string) *types.Interface {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "iface.go", src, 0)
	if err != nil {
		panic(fmt.Errorf("parsing synthetic interface source %q: %w: %w", src, err, ErrInternal))
	}

	// nil importer: the snippet imports nothing, so the type-checker never invokes it.
	pkg, err := new(types.Config).Check("p", fset, []*ast.File{f}, nil)
	if err != nil {
		panic(fmt.Errorf("type-checking synthetic interface source %q: %w: %w", src, err, ErrInternal))
	}

	obj := pkg.Scope().Lookup("T")
	if obj == nil {
		panic(fmt.Errorf("synthetic interface source %q does not declare a type T: %w", src, ErrInternal))
	}

	iface, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		panic(fmt.Errorf("synthetic interface source %q did not declare an interface T: %w", src, ErrInternal))
	}

	return iface
}

func IsTextMarshaler(tpe types.Type) bool {
	return types.Implements(tpe, textMarshalerIface)
}

// IsJSONMapKey reports whether a Go map with this key type marshals to a JSON object under
// encoding/json — i.e. whether the map is representable as {type: object, additionalProperties:
// V}.
//
// The rule mirrors encoding/json's newMapEncoder: the key kind is string or any integer /
// unsigned-integer kind (int, int8…int64, uint, uint8…uint64, uintptr — all stringified), or
// the key implements encoding.TextMarshaler.
//
// Everything else (float, bool, struct without TextMarshaler, interface, func) makes json.Marshal
// fail; json.Marshaler is never consulted for keys.
func IsJSONMapKey(key types.Type) bool {
	if b, ok := key.Underlying().(*types.Basic); ok && b.Info()&(types.IsString|types.IsInteger) != 0 {
		return true
	}
	return IsTextMarshaler(key)
}

func IsStdTime(o *types.TypeName) bool {
	return o.Pkg() != nil && o.Pkg().Name() == "time" && o.Name() == "Time"
}

// IsStdUUID reports whether o is the go1.27 stdlib [uuid.UUID].
//
// Identity-based, so it never misfires on the many third-party types also named UUID
// (github.com/google/uuid, gofrs, strfmt, …) — those keep going through the fuzzy
// name heuristic in the schema builder.
//
// Deliberately NOT behind a go1.27 build tag: codescan compares types harvested from
// *scanned* code, never imports uuid itself, and ships as a binary that may be built by
// an older toolchain than the module it scans.
func IsStdUUID(o *types.TypeName) bool {
	return o.Pkg() != nil && o.Pkg().Path() == "uuid" && o.Name() == "UUID"
}

func IsStdError(o *types.TypeName) bool {
	return o.Pkg() == nil && o.Name() == "error"
}

// IsStdErrorType reports whether t IS the predeclared error, however it is spelled.
//
// [IsStdError] keys on an object, and an alias's object is the alias's own name — so
// `type Wrapped = error` never matches it, and a rule written against the object alone applies to
// one spelling of the same type and not the other.
func IsStdErrorType(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)

	return ok && IsStdError(named.Obj())
}

// opaqueStreamTypes are the named types that mean "a stream of bytes whose framing the declaration
// does not state".
//
// Keyed by package path, then type name — identity, never structure. A structural rule ("anything
// with a Read method") would swallow any user interface that happens to expose one, which is
// exactly the over-reach that makes guessing dangerous here. This list is closed: a type joins it
// because it is a known stream carrier, not because its method set resembles one.
//
// `io.Writer` and the write-only closers are deliberately absent. A sink the caller writes into is
// not something that travels on the wire, so an API type containing one is unknown territory; it
// keeps whatever the structural walk makes of it, and the author says what they meant with
// `swagger:file` or `swagger:type`.
var opaqueStreamTypes = map[string]map[string]struct{}{ //nolint:gochecknoglobals // immutable lookup table, built once
	"io": {
		"Reader":         {},
		"ReadCloser":     {},
		"ReadSeeker":     {},
		"ReadSeekCloser": {},
		"ReadWriter":     {},
		"ReaderAt":       {},
		"ReaderFrom":     {},
		"LimitedReader":  {},
		"ByteReader":     {},
		"ByteScanner":    {},
	},
	"mime/multipart": {
		"File": {},
	},
	"github.com/go-openapi/runtime": {
		"File":            {}, // up to now, a type alias (see below)
		"NamedReadCloser": {},
		"MultipartForm":   {},
	},
	"github.com/go-openapi/swag/fileutils": {
		"File": {},
	},
}

// IsOpaqueStream reports whether o is one of the known byte-stream carriers.
//
// Identity-based, so it answers from the object alone and can run ahead of any declaration lookup —
// which matters, because the drilling these types used to reach invented a `close` property
// of type string out of `Close() error`.
func IsOpaqueStream(o *types.TypeName) bool {
	if o == nil || o.Pkg() == nil {
		return false
	}

	names, found := opaqueStreamTypes[o.Pkg().Path()]
	if !found {
		return false
	}
	_, found = names[o.Name()]

	return found
}

// IsStdJSONRawMessage reports whether o is [encoding/json.RawMessage], in either of the two shapes a
// toolchain gives it.
//
// go1.27 turned RawMessage into an alias of [encoding/json/jsontext.Value], so unaliasing a field
// written as json.RawMessage now lands on jsontext.Value, and a predicate that reads only
// (encoding/json, RawMessage) stopped matching there: the type fell through to its []byte underlying
// and rendered as an array of uint8 instead of "any JSON". Both names mean raw JSON text, so both
// answer yes.
func IsStdJSONRawMessage(o *types.TypeName) bool {
	if o == nil || o.Pkg() == nil {
		return false
	}

	switch o.Pkg().Path() {
	case "encoding/json":
		return o.Name() == "RawMessage"
	case "encoding/json/jsontext":
		return o.Name() == "Value"
	default:
		return false
	}
}

func IsAny(o *types.TypeName) bool {
	return o.Pkg() == nil && o.Name() == "any"
}

// isStdBig reports whether o is the math/big type called name.
func isStdBig(o *types.TypeName, name string) bool {
	return o != nil && o.Pkg() != nil && o.Pkg().Path() == "math/big" && o.Name() == name
}

// IsStdBigInt reports whether o is [math/big.Int], which travels as a JSON *number*: it carries a
// MarshalJSON emitting a bare numeric literal, and encoding/json prefers json.Marshaler over the
// MarshalText the same type also has.
func IsStdBigInt(o *types.TypeName) bool { return isStdBig(o, "Int") }

// IsStdBigFloat reports whether o is [math/big.Float], which travels as a JSON *string* holding a
// decimal float ("3.5"): it has MarshalText and no MarshalJSON, and unmarshalling rejects a number.
func IsStdBigFloat(o *types.TypeName) bool { return isStdBig(o, "Float") }

// IsStdBigRat reports whether o is [math/big.Rat], which travels as a JSON *string* holding a
// quotient ("5/3"): it has MarshalText and no MarshalJSON, and unmarshalling rejects a number.
func IsStdBigRat(o *types.TypeName) bool { return isStdBig(o, "Rat") }

func AddExtension(ve *oaispec.VendorExtensible, key string, value any, skip bool) {
	if skip {
		return
	}

	// handle synonyms
	if (key != "x-nullable" && key != "x-isnullable") || len(ve.Extensions) == 0 {
		ve.AddExtension(key, value)

		return
	}

	if _, ok := ve.Extensions["x-nullable"]; ok {
		return
	}
	if _, ok := ve.Extensions["x-isnullable"]; ok {
		return
	}

	ve.AddExtension(key, value)
}
