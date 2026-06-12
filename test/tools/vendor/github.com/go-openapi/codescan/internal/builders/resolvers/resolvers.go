// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package resolvers

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-openapi/codescan/internal/ifaces"
	"golang.org/x/tools/go/ast/astutil"
)

const (
	// Go builtin type names used for type-to-schema mapping.
	goTypeByte    = "byte"
	goTypeFloat64 = "float64"
	goTypeInt     = "int"
	goTypeInt16   = "int16"
	goTypeInt32   = "int32"
	goTypeInt64   = "int64"
)

// SwaggerSchemaForType maps all Go builtin types that have Json representation to Swagger/Json types.
//
// See https://golang.org/pkg/builtin/ and http://swagger.io/specification/
func SwaggerSchemaForType(typeName string, prop ifaces.SwaggerTypable) error {
	switch typeName {
	case "bool":
		prop.Typed("boolean", "")
	case goTypeByte:
		prop.Typed("integer", "uint8")
	case "complex128", "complex64":
		return fmt.Errorf("unsupported builtin %q (no JSON marshaller): %w", typeName, ErrResolver)
	case "error":
		// Proposal for enhancement: error is often marshalled into a string but not always (e.g. errors package creates
		// errors that are marshalled into an empty object), this could be handled the same way
		// custom JSON marshallers are handled (future)
		prop.Typed("string", "")
	case "float32":
		prop.Typed("number", "float")
	case goTypeFloat64:
		prop.Typed("number", "double")
	case goTypeInt:
		prop.Typed("integer", goTypeInt64)
	case goTypeInt16:
		prop.Typed("integer", goTypeInt16)
	case goTypeInt32:
		prop.Typed("integer", goTypeInt32)
	case goTypeInt64:
		prop.Typed("integer", goTypeInt64)
	case "int8":
		prop.Typed("integer", "int8")
	case "rune":
		prop.Typed("integer", goTypeInt32)
	case "string":
		prop.Typed("string", "")
	case "uint":
		prop.Typed("integer", "uint64")
	case "uint16":
		prop.Typed("integer", "uint16")
	case "uint32":
		prop.Typed("integer", "uint32")
	case "uint64":
		prop.Typed("integer", "uint64")
	case "uint8":
		prop.Typed("integer", "uint8")
	case "uintptr":
		prop.Typed("integer", "uint64")
	case "object":
		prop.Typed("object", "")
	default:
		return fmt.Errorf("unsupported type %q: %w", typeName, ErrResolver)
	}
	return nil
}

var unsupportedTypes = map[string]struct{}{ //nolint:gochecknoglobals // immutable lookup table
	"complex64":  {},
	"complex128": {},
}

func UnsupportedBuiltinType(tpe types.Type) bool {
	unaliased := types.Unalias(tpe)

	switch ftpe := unaliased.(type) {
	case *types.Basic:
		return UnsupportedBasic(ftpe)
	case *types.TypeParam:
		return true
	case *types.Chan:
		return true
	case *types.Signature:
		return true
	case ifaces.Objecter:
		return UnsupportedBuiltin(ftpe)
	default:
		return false
	}
}

func UnsupportedBuiltin(tpe ifaces.Objecter) bool {
	o := tpe.Obj()
	if o == nil {
		return false
	}

	if o.Pkg() != nil {
		if o.Pkg().Path() == "unsafe" {
			return true
		}

		return false // not a builtin type
	}

	_, found := unsupportedTypes[o.Name()]

	return found
}

func UnsupportedBasic(tpe *types.Basic) bool {
	if tpe.Kind() == types.UnsafePointer {
		return true
	}

	_, found := unsupportedTypes[tpe.Name()]

	return found
}

func FindASTField(file *ast.File, pos token.Pos) *ast.Field {
	ans, _ := astutil.PathEnclosingInterval(file, pos, pos)
	for _, an := range ans {
		if at, valid := an.(*ast.Field); valid {
			return at
		}
	}
	return nil
}

type tagOptions []string

func (t tagOptions) Contain(option string) bool {
	for i := 1; i < len(t); i++ {
		if t[i] == option {
			return true
		}
	}
	return false
}

func (t tagOptions) Name() string {
	return t[0]
}

func ParseJSONTag(field *ast.Field) (name string, ignore, isString, omitEmpty bool, err error) {
	if len(field.Names) > 0 {
		name = field.Names[0].Name
	}
	if field.Tag == nil || len(strings.TrimSpace(field.Tag.Value)) == 0 {
		return name, false, false, false, nil
	}

	tv, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return name, false, false, false, err
	}

	if strings.TrimSpace(tv) != "" {
		st := reflect.StructTag(tv)
		jsonParts := tagOptions(strings.Split(st.Get("json"), ","))

		if jsonParts.Contain("string") {
			// Need to check if the field type is a scalar. Otherwise, the
			// ",string" directive doesn't apply.
			isString = IsFieldStringable(field.Type)
		}

		omitEmpty = jsonParts.Contain("omitempty")

		switch jsonParts.Name() {
		case "-":
			return name, true, isString, omitEmpty, nil
		case "":
			return name, false, isString, omitEmpty, nil
		default:
			return jsonParts.Name(), false, isString, omitEmpty, nil
		}
	}
	return name, false, false, false, nil
}
