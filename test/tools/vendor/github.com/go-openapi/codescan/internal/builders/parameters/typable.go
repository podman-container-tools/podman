// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parameters

import (
	"github.com/go-openapi/codescan/internal/builders/items"
	"github.com/go-openapi/codescan/internal/builders/schema"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/parsers"
	oaispec "github.com/go-openapi/spec"
)

var _ ifaces.OperationValidationBuilder = &paramValidations{}

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
	return items.NewTypable(pt.param.Items, 1, pt.param.In)
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
	pt.param.AddExtension(parsers.EnumDescExtension(), desc)
}

type paramValidations struct {
	current *oaispec.Parameter
}

func (sv paramValidations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}

func (sv paramValidations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}
func (sv paramValidations) SetMultipleOf(val float64)      { sv.current.MultipleOf = &val }
func (sv paramValidations) SetMinItems(val int64)          { sv.current.MinItems = &val }
func (sv paramValidations) SetMaxItems(val int64)          { sv.current.MaxItems = &val }
func (sv paramValidations) SetMinLength(val int64)         { sv.current.MinLength = &val }
func (sv paramValidations) SetMaxLength(val int64)         { sv.current.MaxLength = &val }
func (sv paramValidations) SetPattern(val string)          { sv.current.Pattern = val }
func (sv paramValidations) SetUnique(val bool)             { sv.current.UniqueItems = val }
func (sv paramValidations) SetCollectionFormat(val string) { sv.current.CollectionFormat = val }
func (sv paramValidations) SetEnum(val string) {
	sv.current.Enum = parsers.ParseEnum(val, &oaispec.SimpleSchema{Type: sv.current.Type, Format: sv.current.Format})
}
func (sv paramValidations) SetDefault(val any) { sv.current.Default = val }
func (sv paramValidations) SetExample(val any) { sv.current.Example = val }
