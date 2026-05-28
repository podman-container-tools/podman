// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package resolvers

import (
	"fmt"
	"go/ast"
	"go/importer"
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
	if o.Pkg() != nil {
		return
	}

	panic(fmt.Errorf("type %q expected not to be a builtin: %w", o.Name(), ErrInternal))
}

func MustHaveRightHandSide(a *types.Alias) {
	if a.Rhs() != nil {
		return
	}

	panic(fmt.Errorf("type alias %q expected to declare a right-hand-side: %w", a.Obj().Name(), ErrInternal))
}

// IsFieldStringable check if the field type is a scalar. If the field type is
// *ast.StarExpr and is pointer type, check if it refers to a scalar.
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

func IsTextMarshaler(tpe types.Type) bool {
	encodingPkg, err := importer.Default().Import("encoding")
	if err != nil {
		return false
	}
	// Proposal for enhancement: there should be a better way to check this than hardcoding the TextMarshaler iface.
	obj := encodingPkg.Scope().Lookup("TextMarshaler")
	if obj == nil {
		return false
	}
	ifc, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		return false
	}

	return types.Implements(tpe, ifc)
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

	ve.AddExtension(key, value)
}
