// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parameters

import (
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/builders/schema"
	"github.com/go-openapi/codescan/internal/ifaces"
	oaispec "github.com/go-openapi/spec"
)

type paramTypable struct {
	param   *oaispec.Parameter
	skipExt bool
}

func (pt paramTypable) In() string { return pt.param.In }

func (pt paramTypable) Level() int { return 0 }

func (pt paramTypable) Typed(tpe, format string) {
	pt.param.Typed(tpe, format)
}

func (pt paramTypable) SetRef(ref oaispec.Ref) {
	pt.param.Ref = ref
}

func (pt paramTypable) Items() ifaces.SwaggerTypable { //nolint:ireturn // polymorphic by design
	bdt, schema := schema.BodyTypable(pt.param.In, pt.param.Schema, pt.skipExt)
	if bdt != nil {
		pt.param.Schema = schema
		return bdt
	}

	if pt.param.Items == nil {
		pt.param.Items = new(oaispec.Items)
	}
	pt.param.Type = "array"
	return resolvers.NewItemsTypable(pt.param.Items, 1, pt.param.In)
}

func (pt paramTypable) Schema() *oaispec.Schema {
	if pt.param.In != inBody {
		return nil
	}
	if pt.param.Schema == nil {
		pt.param.Schema = new(oaispec.Schema)
	}
	return pt.param.Schema
}

func (pt paramTypable) AddExtension(key string, value any) {
	if pt.param.In == inBody {
		pt.Schema().AddExtension(key, value)
	} else {
		pt.param.AddExtension(key, value)
	}
}

func (pt paramTypable) WithEnum(values ...any) {
	pt.param.WithEnum(values...)
}

func (pt paramTypable) WithEnumDescription(desc string) {
	if desc == "" {
		return
	}
	pt.param.AddExtension(resolvers.ExtEnumDesc, desc)
}

// SimpleSchemaShape satisfies schema.SimpleSchemaProbe.
//
// See [§typable](./README.md#typable).
func (pt paramTypable) SimpleSchemaShape() *oaispec.SimpleSchema {
	return &pt.param.SimpleSchema
}

// HasRef satisfies schema.SimpleSchemaProbe.
//
// SimpleSchema forbids $ref; a non-empty Ref signals a violation.
func (pt paramTypable) HasRef() bool {
	return pt.param.Ref.String() != ""
}

// ResetForViolation satisfies schema.SimpleSchemaProbe.
//
// Wipes SimpleSchema and Ref back to empty.
func (pt paramTypable) ResetForViolation() {
	pt.param.SimpleSchema = oaispec.SimpleSchema{}
	pt.param.Ref = oaispec.Ref{}
}
