// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validations

import "fmt"

// IsFormatCompatible reports whether a `format` is meaningful on a schema whose resolved Swagger
// `type` is already fixed (e.g. by a `swagger:type` override).
//
// It answers the question "may this strfmt format ride on top of this type?" — the
// supplementary-format rule for the swagger:type + swagger:strfmt combination, where swagger:type
// wins on the type axis and the format applies only when it is consistent with that type.
//
// Rules (see README §format-compat):
//
//   - type "string": any format applies (strfmt is string-oriented).
//   - type "integer": the integer formats int{n} and the swagger-extension
//     uint{n} apply.
//   - type "number": the integer formats, the uint{n} extension, AND the
//     float widths apply.
//   - any other type (boolean / object / array / file): no format applies.
//
// An empty format is trivially compatible (there is nothing to apply).
// An empty/unknown schemaType is accepted best-effort, mirroring [IsLegalForType] — the caller
// has no type to check against.
// A false result carries a human-readable hint suitable for a diagnostic message.
func IsFormatCompatible(schemaType, format string) (ok bool, hint string) {
	if format == "" {
		return true, ""
	}
	switch schemaType {
	case "", "string":
		return true, ""
	case "integer":
		if isIntegerFormat(format) {
			return true, ""
		}
	case "number":
		if isIntegerFormat(format) || isFloatFormat(format) {
			return true, ""
		}
	}
	return false, fmt.Sprintf("format %q is not compatible with type %q", format, schemaType)
}

// isIntegerFormat reports whether format is one of the integer-width spellings: the Go int widths
// (int, int8…int64) and the swagger-extension unsigned widths (uint, uint8…uint64). int32 /
// int64 are the only OAS-2-official integer formats; the rest round-trip back to Go.
func isIntegerFormat(format string) bool {
	switch format {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
}

// isFloatFormat reports whether format is one of the float-width spellings: the OAS-2-official
// float / double, plus the Go-spelled float32 / float64.
func isFloatFormat(format string) bool {
	switch format {
	case "float", "double", "float32", "float64":
		return true
	default:
		return false
	}
}
