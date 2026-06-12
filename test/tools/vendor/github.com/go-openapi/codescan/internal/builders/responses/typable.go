// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package responses

import (
	"github.com/go-openapi/codescan/internal/builders/items"
	"github.com/go-openapi/codescan/internal/builders/schema"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/parsers"
	oaispec "github.com/go-openapi/spec"
)

var _ ifaces.ValidationBuilder = &headerValidations{}

type responseTypable struct {
	in       string
	header   *oaispec.Header
	response *oaispec.Response
	skipExt  bool
}

func (ht responseTypable) In() string { return ht.in }

func (ht responseTypable) Level() int { return 0 }

func (ht responseTypable) Typed(tpe, format string) {
	ht.header.Typed(tpe, format)
}

func (ht responseTypable) Items() ifaces.SwaggerTypable { //nolint:ireturn // polymorphic by design
	bdt, schema := schema.BodyTypable(ht.in, ht.response.Schema, ht.skipExt)
	if bdt != nil {
		ht.response.Schema = schema
		return bdt
	}

	if ht.header.Items == nil {
		ht.header.Items = new(oaispec.Items)
	}

	ht.header.Type = "array"

	return items.NewTypable(ht.header.Items, 1, "header")
}

func (ht responseTypable) SetRef(ref oaispec.Ref) {
	// having trouble seeing the usefulness of this one here
	ht.Schema().Ref = ref
}

func (ht responseTypable) Schema() *oaispec.Schema {
	if ht.response.Schema == nil {
		ht.response.Schema = new(oaispec.Schema)
	}

	return ht.response.Schema
}

func (ht responseTypable) AddExtension(key string, value any) {
	ht.response.AddExtension(key, value)
}

func (ht responseTypable) WithEnum(values ...any) {
	ht.header.WithEnum(values)
}

func (ht responseTypable) WithEnumDescription(_ string) {
	// no
}

type headerValidations struct {
	current *oaispec.Header
}

func (sv headerValidations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}

func (sv headerValidations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}

func (sv headerValidations) SetMultipleOf(val float64) {
	sv.current.MultipleOf = &val
}

func (sv headerValidations) SetMinItems(val int64) {
	sv.current.MinItems = &val
}

func (sv headerValidations) SetMaxItems(val int64) {
	sv.current.MaxItems = &val
}

func (sv headerValidations) SetMinLength(val int64) {
	sv.current.MinLength = &val
}

func (sv headerValidations) SetMaxLength(val int64) {
	sv.current.MaxLength = &val
}

func (sv headerValidations) SetPattern(val string) {
	sv.current.Pattern = val
}

func (sv headerValidations) SetUnique(val bool) {
	sv.current.UniqueItems = val
}

func (sv headerValidations) SetCollectionFormat(val string) {
	sv.current.CollectionFormat = val
}

func (sv headerValidations) SetEnum(val string) {
	sv.current.Enum = parsers.ParseEnum(val, &oaispec.SimpleSchema{Type: sv.current.Type, Format: sv.current.Format})
}

func (sv headerValidations) SetDefault(val any) { sv.current.Default = val }

func (sv headerValidations) SetExample(val any) { sv.current.Example = val }
