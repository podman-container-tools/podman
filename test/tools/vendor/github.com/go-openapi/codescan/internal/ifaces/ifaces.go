// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package ifaces defines the internal interfaces that decouple the
// comment-parsing pipeline from the concrete Swagger spec builders.
//
// These interfaces allow the parsers (in internal/parsers) to remain
// independent of the specific spec objects they write into (Schema,
// Parameter, Header, Items), and let the builders (in internal/builders)
// provide type-specific implementations.
package ifaces

import (
	"go/types"

	"github.com/go-openapi/spec"
)

// SwaggerTypable is a write target for Swagger type assignments.
//
// When the scanner resolves a Go type to its Swagger representation
// (e.g. int64 -> "integer"/"int64", or a named struct -> a $ref),
// it writes the result through this interface. The four production
// implementations adapt SwaggerTypable to the Swagger spec object
// shapes: [spec.Schema], [spec.Parameter], [spec.Response]/[spec.Header],
// and [spec.Items].
//
// Items returns a nested SwaggerTypable for the element type of arrays,
// enabling recursive descent into multi-level array types. In reports the
// parameter location (body, query, path, header, formData) so the pipeline
// can branch: body parameters use schemas, while others use simple types.
// Level reports the current nesting depth for array items.
type SwaggerTypable interface {
	Typed(swaggerType string, format string)
	SetRef(ref spec.Ref)
	Items() SwaggerTypable
	Schema() *spec.Schema
	Level() int
	AddExtension(key string, value any)
	WithEnum(values ...any)
	WithEnumDescription(desc string)
	In() string
}

// ValidationBuilder is a write target for Swagger validation constraints.
//
// When the comment parser encounters a validation directive such as
// "Maximum: 100" or "Pattern: ^[a-z]+$", it extracts the value and
// calls the corresponding setter on this interface. This decouples
// the parsing logic (in internal/parsers) from the spec-object
// structure: each builder implementation (for Schema, Parameter,
// Header, or Items) knows how to write the constraint onto its
// specific spec type.
type ValidationBuilder interface { //nolint:interfacebloat // mirrors the full set of Swagger validation properties
	SetMaximum(maxium float64, isExclusive bool)
	SetMinimum(minimum float64, isExclusive bool)
	SetMultipleOf(multiple float64)

	SetMinItems(minItems int64)
	SetMaxItems(maxItems int64)

	SetMinLength(minLength int64)
	SetMaxLength(maxLength int64)
	SetPattern(pattern string)

	SetUnique(isUniqueItems bool)
	SetEnum(enumValue string)
	SetDefault(defaultValue any)
	SetExample(example any)
}

// ValueParser is the fundamental unit of comment-line parsing.
//
// Each implementation recognizes (via Matches) and extracts data from
// (via Parse) one kind of swagger annotation or validation directive
// in a Go comment block. ValueParsers are composed into TagParser
// wrappers and fed to the SectionedParser, which iterates over
// comment lines and dispatches each line to the first matching parser.
type ValueParser interface {
	Parse(commentlines []string) error
	Matches(commentLine string) bool
}

// OperationValidationBuilder extends [ValidationBuilder] with
// SetCollectionFormat, which applies only to operation parameters,
// response headers, and array items — not to schema definitions.
//
// The narrower interface enforces at the type level that collection
// format (csv, ssv, tsv, pipes, multi) cannot be accidentally set on
// a schema. Schema validations implement only [ValidationBuilder].
type OperationValidationBuilder interface {
	ValidationBuilder
	SetCollectionFormat(collectionFormat string)
}

// Objecter abstracts over Go type objects that carry a [types.TypeName].
//
// It is used during type resolution to detect unsupported builtin types
// (complex64, complex128, unsafe.Pointer) that have no JSON/Swagger
// representation. Both [*types.Named] and [scanner.EntityDecl] satisfy
// this interface, giving the resolver a uniform way to extract the
// underlying type name without a type switch on every concrete type.
type Objecter interface {
	Obj() *types.TypeName
}
