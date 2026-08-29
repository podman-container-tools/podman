// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package resolvers

import (
	"github.com/go-openapi/codescan/internal/builders/validations"
	"github.com/go-openapi/codescan/internal/ifaces"
	oaispec "github.com/go-openapi/spec"
)

// ItemsTypable adapts an *oaispec.Items target to ifaces.SwaggerTypable so the parameters/responses
// builders can recurse through nested array layers (SimpleSchema's items chain).
//
// One adapter, two callers (parameters, responses); lives here because it's pure ifaces-glue with
// no builder-specific logic.
// The schema builder uses its own *oaispec.Schema.Items chain and has no need for this adapter.
type ItemsTypable struct {
	items *oaispec.Items
	level int
	in    string
}

func NewItemsTypable(items *oaispec.Items, level int, in string) ItemsTypable {
	return ItemsTypable{items: items, level: level, in: in}
}

func (pt ItemsTypable) In() string { return pt.in }

func (pt ItemsTypable) Level() int { return pt.level }

func (pt ItemsTypable) Typed(tpe, format string) { pt.items.Typed(tpe, format) }

func (pt ItemsTypable) SetRef(ref oaispec.Ref) { pt.items.Ref = ref }

func (pt ItemsTypable) Schema() *oaispec.Schema { return nil }

func (pt ItemsTypable) Items() ifaces.SwaggerTypable { //nolint:ireturn // polymorphic by design
	if pt.items.Items == nil {
		pt.items.Items = new(oaispec.Items)
	}
	pt.items.Type = "array"
	return ItemsTypable{pt.items.Items, pt.level + 1, pt.in}
}

func (pt ItemsTypable) AddExtension(key string, value any) {
	pt.items.AddExtension(key, value)
}

func (pt ItemsTypable) WithEnum(values ...any) {
	pt.items.WithEnum(values...)
}

func (pt ItemsTypable) WithEnumDescription(_ string) {
	// items levels carry no description channel; the enclosing parameter / header description already
	// absorbs the enum doc.
}

// SimpleSchemaShape satisfies schema.SimpleSchemaProbe: an items level in a non-body parameter /
// response header is itself a Swagger 2.0 SimpleSchema.
//
// Exposing it lets the schema builder's catch-at-exit validator inspect array-element shapes, not
// just the top-level target (go-swagger#1088).
func (pt ItemsTypable) SimpleSchemaShape() *oaispec.SimpleSchema {
	return &pt.items.SimpleSchema
}

// HasRef satisfies schema.SimpleSchemaProbe.
//
// SimpleSchema forbids a $ref, including on array items: a named object element ([]Ele under in:
// query) otherwise resolves to `items: {$ref}`, which the Swagger 2.0 editor rejects
// (go-swagger#1088).
func (pt ItemsTypable) HasRef() bool {
	return pt.items.Ref.String() != ""
}

// ResetForViolation satisfies schema.SimpleSchemaProbe.
//
// Dissolves an illegal $ref / shape on this items level back to empty, mirroring
// paramTypable.ResetForViolation one level down.
func (pt ItemsTypable) ResetForViolation() {
	pt.items.SimpleSchema = oaispec.SimpleSchema{}
	pt.items.Ref = oaispec.Ref{}
}

// ItemsValidations wraps an *oaispec.Items as a ValidationBuilder / OperationValidationBuilder
// target.
//
// Used by parameters' and responses' grammar items-level Walker dispatch.
type ItemsValidations struct {
	current *oaispec.Items
}

func NewItemsValidations(it *oaispec.Items) ItemsValidations {
	return ItemsValidations{current: it}
}

func (sv ItemsValidations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}

func (sv ItemsValidations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}

func (sv ItemsValidations) SetMultipleOf(val float64)      { sv.current.MultipleOf = &val }
func (sv ItemsValidations) SetMinItems(val int64)          { sv.current.MinItems = &val }
func (sv ItemsValidations) SetMaxItems(val int64)          { sv.current.MaxItems = &val }
func (sv ItemsValidations) SetMinLength(val int64)         { sv.current.MinLength = &val }
func (sv ItemsValidations) SetMaxLength(val int64)         { sv.current.MaxLength = &val }
func (sv ItemsValidations) SetPattern(val string)          { sv.current.Pattern = val }
func (sv ItemsValidations) SetUnique(val bool)             { sv.current.UniqueItems = val }
func (sv ItemsValidations) SetCollectionFormat(val string) { sv.current.CollectionFormat = val }
func (sv ItemsValidations) SetEnum(val string) {
	sv.current.Enum = validations.CoerceEnum(val, &oaispec.SimpleSchema{Type: sv.current.Type, Format: sv.current.Format})
}
func (sv ItemsValidations) SetDefault(val any) { sv.current.Default = val }
func (sv ItemsValidations) SetExample(val any) { sv.current.Example = val }
