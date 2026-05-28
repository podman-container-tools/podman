// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package items

import (
	"github.com/go-openapi/codescan/internal/ifaces"
	oaispec "github.com/go-openapi/spec"
)

type Typable struct {
	items *oaispec.Items
	level int
	in    string
}

func NewTypable(items *oaispec.Items, level int, in string) Typable {
	return Typable{
		items: items,
		level: level,
		in:    in,
	}
}

func (pt Typable) In() string { return pt.in }

func (pt Typable) Level() int { return pt.level }

func (pt Typable) Typed(tpe, format string) {
	pt.items.Typed(tpe, format)
}

func (pt Typable) SetRef(ref oaispec.Ref) {
	pt.items.Ref = ref
}

func (pt Typable) Schema() *oaispec.Schema {
	return nil
}

func (pt Typable) Items() ifaces.SwaggerTypable { //nolint:ireturn // polymorphic by design
	if pt.items.Items == nil {
		pt.items.Items = new(oaispec.Items)
	}
	pt.items.Type = "array"
	return Typable{pt.items.Items, pt.level + 1, pt.in}
}

func (pt Typable) AddExtension(key string, value any) {
	pt.items.AddExtension(key, value)
}

func (pt Typable) WithEnum(values ...any) {
	pt.items.WithEnum(values...)
}

func (pt Typable) WithEnumDescription(_ string) {
	// no
}
