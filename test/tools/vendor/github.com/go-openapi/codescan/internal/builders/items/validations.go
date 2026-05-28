// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package items

import (
	"github.com/go-openapi/codescan/internal/parsers"
	oaispec "github.com/go-openapi/spec"
)

type Validations struct {
	current *oaispec.Items
}

func (sv Validations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}

func (sv Validations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}
func (sv Validations) SetMultipleOf(val float64)      { sv.current.MultipleOf = &val }
func (sv Validations) SetMinItems(val int64)          { sv.current.MinItems = &val }
func (sv Validations) SetMaxItems(val int64)          { sv.current.MaxItems = &val }
func (sv Validations) SetMinLength(val int64)         { sv.current.MinLength = &val }
func (sv Validations) SetMaxLength(val int64)         { sv.current.MaxLength = &val }
func (sv Validations) SetPattern(val string)          { sv.current.Pattern = val }
func (sv Validations) SetUnique(val bool)             { sv.current.UniqueItems = val }
func (sv Validations) SetCollectionFormat(val string) { sv.current.CollectionFormat = val }
func (sv Validations) SetEnum(val string) {
	sv.current.Enum = parsers.ParseEnum(val, &oaispec.SimpleSchema{Type: sv.current.Type, Format: sv.current.Format})
}
func (sv Validations) SetDefault(val any) { sv.current.Default = val }
func (sv Validations) SetExample(val any) { sv.current.Example = val }
