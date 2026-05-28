// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

// Proposal for enhancement(fredbi): we should probably find a way to register most of this dynamically from strfmt.

// map of function calls to be generated to get the zero value of a given type.
var zeroes = map[string]string{ //nolint:gosec // G101 false positive: not credentials, these are zero-value literals for code generation
	"bool":    "false",
	"float32": "0",
	"float64": "0",
	"int":     "0",
	"int8":    "0",
	"int16":   "0",
	"int32":   "0",
	"int64":   "0",
	"string":  "\"\"",
	"uint":    "0",
	"uint8":   "0",
	"uint16":  "0",
	"uint32":  "0",
	"uint64":  "0",
	// Extended formats (23 formats corresponding to the Default registry
	// provided by go-openapi/strfmt)
	"strfmt.Base64":     "strfmt.Base64([]byte(nil))",
	"strfmt.CreditCard": "strfmt.CreditCard(\"\")",
	"strfmt.Date":       "strfmt.Date{}",
	"strfmt.DateTime":   "strfmt.DateTime{}",
	"strfmt.Duration":   "strfmt.Duration(0)",
	"strfmt.Email":      "strfmt.Email(\"\")",
	"strfmt.HexColor":   "strfmt.HexColor(\"#000000\")",
	"strfmt.Hostname":   "strfmt.Hostname(\"\")",
	"strfmt.IPv4":       "strfmt.IPv4(\"\")",
	"strfmt.IPv6":       "strfmt.IPv6(\"\")",
	"strfmt.ISBN":       "strfmt.ISBN(\"\")",
	"strfmt.ISBN10":     "strfmt.ISBN10(\"\")",
	"strfmt.ISBN13":     "strfmt.ISBN13(\"\")",
	"strfmt.MAC":        "strfmt.MAC(\"\")",
	"strfmt.ObjectId":   "strfmt.ObjectId{}",
	"strfmt.Password":   "strfmt.Password(\"\")",
	"strfmt.RGBColor":   "strfmt.RGBColor(\"rgb(0,0,0)\")",
	"strfmt.SSN":        "strfmt.SSN(\"\")",
	"strfmt.URI":        "strfmt.URI(\"\")",
	"strfmt.UUID":       "strfmt.UUID(\"\")",
	"strfmt.UUID3":      "strfmt.UUID3(\"\")",
	"strfmt.UUID4":      "strfmt.UUID4(\"\")",
	"strfmt.UUID5":      "strfmt.UUID5(\"\")",
	"strfmt.ULID":       "strfmt.ULID(\"\")",
	// "file":       "runtime.File",
}

// conversion functions from string representation to a numerical or boolean
// primitive type.
var stringConverters = map[string]string{
	"bool":    "conv.ConvertBool",
	"float32": "conv.ConvertFloat32",
	"float64": "conv.ConvertFloat64",
	"int8":    "conv.ConvertInt8",
	"int16":   "conv.ConvertInt16",
	"int32":   "conv.ConvertInt32",
	"int64":   "conv.ConvertInt64",
	"uint8":   "conv.ConvertUint8",
	"uint16":  "conv.ConvertUint16",
	"uint32":  "conv.ConvertUint32",
	"uint64":  "conv.ConvertUint64",
}

const (
	// generic converters.
	formatFloat = "conv.FormatFloat"
	formatInt   = "conv.FormatInteger"
	formatUint  = "conv.FormatUinteger"
)

// formatting (string representation) functions from a native representation
// of a numerical or boolean primitive type.
var stringFormatters = map[string]string{
	"bool":    "conv.FormatBool",
	"float32": formatFloat,
	"float64": formatFloat,
	"int8":    formatInt,
	"int16":   formatInt,
	"int32":   formatInt,
	"int64":   formatInt,
	"uint8":   formatUint,
	"uint16":  formatUint,
	"uint32":  formatUint,
	"uint64":  formatUint,
}

// typeMapping contains a mapping of type name to go type.
var typeMapping = map[string]string{
	// Standard formats with native, straightforward, mapping
	"string":  "string",
	"boolean": "bool",
	"integer": "int64",
	"number":  "float64",
	// For file producers
	"file": "runtime.File",
}

// swaggerTypeName contains a mapping from go type to swagger type or format.
var swaggerTypeName map[string]string

func init() {
	// build the reverse-lookup index of typeMapping
	swaggerTypeName = make(map[string]string)
	for k, v := range typeMapping {
		swaggerTypeName[v] = k
	}
}

// formatMapping contains a type-specific version of mapping of format to go type.
var formatMapping = map[string]map[string]string{
	"number": {
		"double": "float64",
		"float":  "float32",
		"int":    "int64",
		"int8":   "int8",
		"int16":  "int16",
		"int32":  "int32",
		"int64":  "int64",
		"uint":   "uint64",
		"uint8":  "uint8",
		"uint16": "uint16",
		"uint32": "uint32",
		"uint64": "uint64",
	},
	"integer": {
		"int":    "int64",
		"int8":   "int8",
		"int16":  "int16",
		"int32":  "int32",
		"int64":  "int64",
		"uint":   "uint64",
		"uint8":  "uint8",
		"uint16": "uint16",
		"uint32": "uint32",
		"uint64": "uint64",
	},
	"string": { //nolint:gosec // G101 false positive: not credentials, this maps OpenAPI string formats to Go types
		"char": "rune",
		// Extended format registry from go-openapi/strfmt.
		// Currently, 23 such formats are supported (default strftm registry),
		// plus the following aliases:
		//  - "datetime" alias for the more official "date-time"
		//  - "objectid" and "ObjectId" aliases for "bsonobjectid"
		"binary":       "io.ReadCloser",
		"byte":         "strfmt.Base64",
		"creditcard":   "strfmt.CreditCard",
		"date":         "strfmt.Date",
		"date-time":    "strfmt.DateTime",
		"datetime":     "strfmt.DateTime",
		"duration":     "strfmt.Duration",
		"email":        "strfmt.Email",
		"hexcolor":     "strfmt.HexColor",
		"hostname":     "strfmt.Hostname",
		"ipv4":         "strfmt.IPv4",
		"ipv6":         "strfmt.IPv6",
		"isbn":         "strfmt.ISBN",
		"isbn10":       "strfmt.ISBN10",
		"isbn13":       "strfmt.ISBN13",
		"mac":          "strfmt.MAC",
		"bsonobjectid": "strfmt.ObjectId",
		"objectid":     "strfmt.ObjectId",
		"ObjectId":     "strfmt.ObjectId", // NOTE: does it work with uppercase?
		"password":     "strfmt.Password",
		"rgbcolor":     "strfmt.RGBColor",
		"ssn":          "strfmt.SSN",
		"uri":          "strfmt.URI",
		"uuid":         "strfmt.UUID",
		"uuid3":        "strfmt.UUID3",
		"uuid4":        "strfmt.UUID4",
		"uuid5":        "strfmt.UUID5",
		"ulid":         "strfmt.ULID",
		// For file producers
		"file": "runtime.File",
	},
}

// go primitive types.
var primitives = map[string]struct{}{
	"bool":       {},
	"byte":       {},
	"[]byte":     {},
	"complex64":  {},
	"complex128": {},
	"float32":    {},
	"float64":    {},
	"int":        {},
	"int8":       {},
	"int16":      {},
	"int32":      {},
	"int64":      {},
	"rune":       {},
	"string":     {},
	"uint":       {},
	"uint8":      {},
	"uint16":     {},
	"uint32":     {},
	"uint64":     {},
}

// Formats with a custom formatter.
// Currently, 23 such formats are supported.
var customFormatters = map[string]struct{}{
	"strfmt.Base64":     {},
	"strfmt.CreditCard": {},
	"strfmt.Date":       {},
	"strfmt.DateTime":   {},
	"strfmt.Duration":   {},
	"strfmt.Email":      {},
	"strfmt.HexColor":   {},
	"strfmt.Hostname":   {},
	"strfmt.IPv4":       {},
	"strfmt.IPv6":       {},
	"strfmt.ISBN":       {},
	"strfmt.ISBN10":     {},
	"strfmt.ISBN13":     {},
	"strfmt.MAC":        {},
	"strfmt.ObjectId":   {},
	"strfmt.Password":   {},
	"strfmt.RGBColor":   {},
	"strfmt.SSN":        {},
	"strfmt.URI":        {},
	"strfmt.UUID":       {},
	"strfmt.UUID3":      {},
	"strfmt.UUID4":      {},
	"strfmt.UUID5":      {},
	// the following interfaces do not generate validations
	"io.ReadCloser": {}, // for "format": "binary" (server side)
	"io.Writer":     {}, // for "format": "binary" (client side)
	// NOTE: runtime.File is not a customFormatter
}
