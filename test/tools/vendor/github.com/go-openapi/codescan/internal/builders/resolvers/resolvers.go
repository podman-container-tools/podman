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

// SwaggerSchemaForType maps all Go builtin types that have Json representation to Swagger/Json
// types.
//
// See https://golang.org/pkg/builtin/ and http://swagger.io/specification/.
func SwaggerSchemaForType(typeName string, prop ifaces.SwaggerTypable) error {
	switch typeName {
	case "bool":
		prop.Typed("boolean", "")
	case goTypeByte:
		prop.Typed("integer", "uint8")
	case "complex128", "complex64":
		return fmt.Errorf("unsupported builtin %q (no JSON marshaller): %w", typeName, ErrResolver)
	case "error":
		// Proposal for enhancement: error is often marshalled into a string but not always (e.g. errors
		// package creates errors that are marshalled into an empty object), this could be handled the
		// same way custom JSON marshallers are handled (future)
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
	// Canonical OAS-2 scalar type names, accepted as `swagger:type` arguments alongside the Go-builtin
	// spellings above (quirk F3).
	//
	// No implied format — `string`/`integer`/`number`/`boolean` carry only their type; a format may
	// still be supplied via swagger:strfmt (applied when format-compatible — see
	// validations.IsFormatCompatible).
	//
	// The Go-basic resolution path never passes these names (a *types.Basic stringifies as
	// `int64`/`string`/…), so this only widens the swagger:type surface.
	case "integer":
		prop.Typed("integer", "")
	case "number":
		prop.Typed("number", "")
	case "boolean":
		prop.Typed("boolean", "")
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

// UnsupportedBuiltin returns true when tpe is unsafe.Pointer.
//
// Other "unsupported builtins" (complex64, complex128) cannot reach this function: they surface as
// *types.Basic, which does not satisfy [ifaces.Objecter].
// Those are caught one layer down by [UnsupportedBasic] / [UnsupportedBuiltinType] when the
// *types.Basic surfaces directly.
//
// Supported builtins:
//
//   - error
func UnsupportedBuiltin(tpe ifaces.Objecter) (skip bool) {
	o := tpe.Obj()
	if o == nil || o.Pkg() == nil {
		return false
	}

	return o.Pkg().Path() == "unsafe"
}

func UnsupportedBasic(tpe *types.Basic) bool {
	if tpe.Kind() == types.UnsafePointer {
		return true
	}

	_, found := unsupportedTypes[tpe.Name()]

	return found
}

// FindASTFieldFor returns the struct field or interface method declaring obj in file, or nil when there is none.
//
// Position first, and for a package read from source that is the whole story: the object and the syntax came out of
// the same type-check, so obj.Pos() indexes straight into file.
//
// A package whose types were read from compiled export data and whose source was parsed separately
// is the case the name fallback exists for.
//
// The two halves reach the same FileSet by different routes and get a [token.File] each,
// so the same declaration holds two unrelated [token.Pos] values and the position lookup finds nothing at all.
// Without the fallback every field of such a type is silently skipped,
// which renders the type as an empty object rather than failing.
//
// Export data preserves the filename and the LINE, not the column. The importer fabricates a line table
// in which every position sits in column 1, so a column comparison is always wrong.
//
// Line plus the object's own name identifies the field, since a struct cannot declare a name twice and the file is
// already known.
//
// NOTE: the pathological miss is two types declaring the same field name on one physical line,
// which costs one field the prose from the other.
//
// Reached only when the position lookup fails, so an ordinary scan never walks the file twice.
func FindASTFieldFor(file *ast.File, obj types.Object, posOf func(token.Pos) token.Position) *ast.Field {
	// The object and the syntax share a token.File whenever they came out of the same type-check.
	if fld := findASTField(file, obj.Pos()); fld != nil {
		return fld
	}

	if file == nil || posOf == nil {
		return nil
	}

	// The syntax is here, its positions just do not index the object's. Fall back to filename, line and name.
	want := posOf(obj.Pos())
	if !want.IsValid() {
		return nil
	}

	var found *ast.Field
	ast.Inspect(file, func(n ast.Node) bool {
		if found != nil {
			return false
		}

		fld, isField := n.(*ast.Field)
		if !isField || !fieldDeclares(fld, obj.Name(), want.Line, posOf) {
			return true
		}
		found = fld

		return false
	})

	return found
}

// findASTField returns the struct or interface field declared at pos in file, or nil when there is none.
//
// A nil file input is legitimate and just returns nil.
// Typically, a package whose types were read from compiled export data has none,
// so it yields no field rather than panicking.
func findASTField(file *ast.File, pos token.Pos) *ast.Field {
	if file == nil {
		// Not defensive padding: astutil.PathEnclosingInterval dereferences its root to bound the search.
		return nil
	}

	ans, _ := astutil.PathEnclosingInterval(file, pos, pos)
	for _, an := range ans {
		if at, valid := an.(*ast.Field); valid {
			return at
		}
	}

	return nil
}

// fieldDeclares reports whether fld declares name on line.
//
// An embedded field has no name of its own — the field IS its type — and go/types names the object after that type,
// so the comparison has to reach into the type expression to find the identifier the two halves share.
func fieldDeclares(fld *ast.Field, name string, line int, posOf func(token.Pos) token.Position) bool {
	if len(fld.Names) == 0 {
		return embeddedFieldName(fld.Type) == name && posOf(fld.Pos()).Line == line
	}

	for _, ident := range fld.Names {
		if ident.Name == name && posOf(ident.Pos()).Line == line {
			return true
		}
	}

	return false
}

// embeddedFieldName is the name an embedded field is known by: the last identifier of its type expression, with the
// pointer, qualifier and type-argument layers peeled off (`*pkg.T[int]` embeds `T`).
func embeddedFieldName(expr ast.Expr) string {
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.SelectorExpr:
			return e.Sel.Name
		case *ast.Ident:
			return e.Name
		default:
			return ""
		}
	}
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

// jsonTagIgnores reports whether a json struct tag skips the field entirely.
//
// encoding/json compares the WHOLE tag to "-": `json:"-"` ignores the field, while `json:"-,"` and
// `json:"-,omitempty"` name it literally "-". Splitting on the comma first conflates the two, which
// dropped a field Go does marshal.
func jsonTagIgnores(st reflect.StructTag) bool {
	return st.Get("json") == "-"
}

// ParseFieldTag derives the emitted name and the encoding/json directives for a struct field.
//
// The name is sourced from the first struct-tag type in nameTags that supplies a usable name — a
// non-empty name-part that isn't "-"; a tag type that is absent or carries only options (e.g.
// `,omitempty`) is skipped and the next type is tried. nameTags is typically Options.NameFromTags,
// defaulting to ["json"].
//
// When nameTags is empty, or none of the listed tags name the field, the name falls back to goName
// — the field's Go identifier as reported by go/types, authoritative because a single AST field
// group may declare several names (`R, G, B, A uint8`), each a distinct go/types field promoted to
// its own property (go-swagger#2638).
//
// When goName is empty the first AST name is used.
//
// The `-` (ignore), `,omitempty` and `,string` directives are ALWAYS read from the `json` tag,
// independent of nameTags — they describe the encoding/json wire shape, not the Swagger name.
// `json:"-"` ignores the field.
//
// A rename can only name a single field, so it is dropped for a multi-name group — each member
// keeps its own Go name — while the directives still apply to every member.
func ParseFieldTag(field *ast.Field, goName string, nameTags []string) (name string, ignore, isString, omitEmpty bool, err error) {
	name = goName
	if name == "" && len(field.Names) > 0 {
		name = field.Names[0].Name
	}
	if field.Tag == nil || len(strings.TrimSpace(field.Tag.Value)) == 0 {
		return name, false, false, false, nil
	}

	tv, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return name, false, false, false, err
	}
	if strings.TrimSpace(tv) == "" {
		return name, false, false, false, nil
	}

	st := reflect.StructTag(tv)

	// Directives are encoding/json-specific: always read them from the json tag, whatever nameTags
	// selects for the name.
	jsonParts := tagOptions(strings.Split(st.Get("json"), ","))
	if jsonParts.Contain("string") {
		// The ",string" directive only applies to scalar field types.
		isString = IsFieldStringable(field.Type)
	}
	omitEmpty = jsonParts.Contain("omitempty")
	if jsonTagIgnores(st) {
		return name, true, isString, omitEmpty, nil
	}

	// The name comes from the first tag type that yields a usable name.
	// A rename can't name N members of a multi-name group, so each keeps its own Go name.
	if len(field.Names) <= 1 {
		for _, tagType := range nameTags {
			candidate := tagOptions(strings.Split(st.Get(tagType), ",")).Name()
			if candidate == "" {
				continue
			}
			// "-" is a legitimate name only from the json tag, and only because the whole-tag ignore case
			// returned above: `json:"-,"` names the field literally "-". For any other tag type there is
			// no encoding rule to appeal to, so "-" stays "no usable name".
			if candidate == "-" && tagType != "json" {
				continue
			}
			name = candidate

			break
		}
	}

	return name, false, isString, omitEmpty, nil
}

// ExplicitJSONName returns the name set in a field's json struct tag — the part before the first
// comma — or "" when the field has no json tag, the tag sets no name (`json:",omitempty"`), or
// the tag skips the field (`json:"-"`).
//
// Go's encoding/json treats an *embedded* struct field carrying an explicit json name as a regular
// named field: the embedded value nests under that name instead of being promoted.
// Callers use a non-empty result to distinguish a nesting embed from a flattening one
// (go-swagger#2038).
func ExplicitJSONName(field *ast.Field) string {
	if field == nil || field.Tag == nil || len(strings.TrimSpace(field.Tag.Value)) == 0 {
		return ""
	}
	tv, err := strconv.Unquote(field.Tag.Value)
	if err != nil || strings.TrimSpace(tv) == "" {
		return ""
	}
	st := reflect.StructTag(tv)
	if jsonTagIgnores(st) {
		return ""
	}

	// A remaining "-" is the literal name (`json:"-,"`), not an ignore.
	return tagOptions(strings.Split(st.Get("json"), ",")).Name()
}
