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

func MustBeAType(tpe types.TypeAndValue) {
	if tpe.IsType() {
		return
	}

	panic(fmt.Errorf("declaration is not a type: %v: %w", tpe, ErrInternal))
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

func IsStdError(o *types.TypeName) bool {
	return o.Pkg() == nil && o.Name() == "error"
}

func IsStdJSONRawMessage(o *types.TypeName) bool {
	return o.Pkg() != nil && o.Pkg().Path() == "encoding/json" && o.Name() == "RawMessage"
}

func IsAny(o *types.TypeName) bool {
	return o.Pkg() == nil && o.Name() == "any"
}

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
