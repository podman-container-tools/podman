// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/parsers"
	oaispec "github.com/go-openapi/spec"
)

var _ ifaces.ValidationBuilder = &schemaValidations{}

type Typable struct {
	schema  *oaispec.Schema
	level   int
	skipExt bool
}

func NewTypable(schema *oaispec.Schema, level int, skipExt bool) Typable {
	return Typable{
		schema:  schema,
		level:   level,
		skipExt: skipExt,
	}
}

func (st Typable) In() string { return "body" }

func (st Typable) Typed(tpe, format string) {
	st.schema.Typed(tpe, format)
}

func (st Typable) SetRef(ref oaispec.Ref) { // TODO(fred/claude): isn't it a bug? Setter on non-pointer receiver?
	st.schema.Ref = ref
}

func (st Typable) Schema() *oaispec.Schema {
	return st.schema
}

//nolint:ireturn // polymorphic by design
func (st Typable) Items() ifaces.SwaggerTypable {
	if st.schema.Items == nil {
		st.schema.Items = new(oaispec.SchemaOrArray) // TODO(fred/claude): isn't it a bug? Setter on non-pointer receiver?
	}
	if st.schema.Items.Schema == nil {
		st.schema.Items.Schema = new(oaispec.Schema) // TODO(fred/claude): isn't it a bug? Setter on non-pointer receiver?
	}

	st.schema.Typed("array", "")
	return Typable{st.schema.Items.Schema, st.level + 1, st.skipExt}
}

func (st Typable) AdditionalProperties() ifaces.SwaggerTypable { //nolint:ireturn // polymorphic by design
	if st.schema.AdditionalProperties == nil {
		st.schema.AdditionalProperties = new(oaispec.SchemaOrBool)
	}
	if st.schema.AdditionalProperties.Schema == nil {
		st.schema.AdditionalProperties.Schema = new(oaispec.Schema)
	}

	st.schema.Typed("object", "")
	return Typable{st.schema.AdditionalProperties.Schema, st.level + 1, st.skipExt}
}

func (st Typable) Level() int { return st.level }

func (st Typable) AddExtension(key string, value any) {
	resolvers.AddExtension(&st.schema.VendorExtensible, key, value, st.skipExt)
}

func (st Typable) WithEnum(values ...any) {
	st.schema.WithEnum(values...)
}

func (st Typable) WithEnumDescription(desc string) {
	if desc == "" {
		return
	}
	st.AddExtension(parsers.EnumDescExtension(), desc)
}

func BodyTypable(in string, schema *oaispec.Schema, skipExt bool) (ifaces.SwaggerTypable, *oaispec.Schema) { //nolint:ireturn // polymorphic by design
	if in == "body" {
		// get the schema for items on the schema property
		if schema == nil {
			schema = new(oaispec.Schema)
		}
		if schema.Items == nil {
			schema.Items = new(oaispec.SchemaOrArray)
		}
		if schema.Items.Schema == nil {
			schema.Items.Schema = new(oaispec.Schema)
		}
		schema.Typed("array", "")
		return Typable{schema.Items.Schema, 1, skipExt}, schema
	}

	return nil, nil
}

type schemaValidations struct {
	current *oaispec.Schema
}

func (sv schemaValidations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}

func (sv schemaValidations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}
func (sv schemaValidations) SetMultipleOf(val float64) { sv.current.MultipleOf = &val }
func (sv schemaValidations) SetMinItems(val int64)     { sv.current.MinItems = &val }
func (sv schemaValidations) SetMaxItems(val int64)     { sv.current.MaxItems = &val }
func (sv schemaValidations) SetMinLength(val int64)    { sv.current.MinLength = &val }
func (sv schemaValidations) SetMaxLength(val int64)    { sv.current.MaxLength = &val }
func (sv schemaValidations) SetPattern(val string)     { sv.current.Pattern = val }
func (sv schemaValidations) SetUnique(val bool)        { sv.current.UniqueItems = val }
func (sv schemaValidations) SetDefault(val any)        { sv.current.Default = val }
func (sv schemaValidations) SetExample(val any)        { sv.current.Example = val }
func (sv schemaValidations) SetEnum(val string) {
	var typ string
	if len(sv.current.Type) > 0 {
		typ = sv.current.Type[0]
	}
	sv.current.Enum = parsers.ParseEnum(val, &oaispec.SimpleSchema{Format: sv.current.Format, Type: typ})
}
