// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/ifaces"
	oaispec "github.com/go-openapi/spec"
)

type Typable struct {
	schema  *oaispec.Schema
	level   int
	skipExt bool
}

func NewTypable(schema *oaispec.Schema, level int, skipExt bool) *Typable {
	return &Typable{
		schema:  schema,
		level:   level,
		skipExt: skipExt,
	}
}

func (st Typable) In() string { return "body" }

func (st Typable) Typed(tpe, format string) {
	st.schema.Typed(tpe, format)
}

func (st *Typable) SetRef(ref oaispec.Ref) {
	st.schema.Ref = ref
}

func (st Typable) Schema() *oaispec.Schema {
	return st.schema
}

//nolint:ireturn // polymorphic by design
func (st *Typable) Items() ifaces.SwaggerTypable {
	if st.schema.Items == nil {
		st.schema.Items = new(oaispec.SchemaOrArray)
	}
	if st.schema.Items.Schema == nil {
		st.schema.Items.Schema = new(oaispec.Schema)
	}

	st.schema.Typed("array", "")
	return &Typable{st.schema.Items.Schema, st.level + 1, st.skipExt}
}

func (st Typable) AdditionalProperties() ifaces.SwaggerTypable {
	if st.schema.AdditionalProperties == nil {
		st.schema.AdditionalProperties = new(oaispec.SchemaOrBool)
	}
	if st.schema.AdditionalProperties.Schema == nil {
		st.schema.AdditionalProperties.Schema = new(oaispec.Schema)
	}

	st.schema.Typed("object", "")
	return &Typable{st.schema.AdditionalProperties.Schema, st.level + 1, st.skipExt}
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
	st.AddExtension(resolvers.ExtEnumDesc, desc)
}

func BodyTypable(in string, schema *oaispec.Schema, skipExt bool) (ifaces.SwaggerTypable, *oaispec.Schema) { //nolint:ireturn // polymorphic by design
	if in != "body" {
		return nil, nil // notice that nil,nil does not correspond to a "nil error", but rather to a nil schema.
	}

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

	return &Typable{schema.Items.Schema, 1, skipExt}, schema
}
