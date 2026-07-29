// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package handlers

import (
	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/builders/validations"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	oaispec "github.com/go-openapi/spec"
)

var (
	_ ifaces.OperationValidationBuilder = &paramValidations{}
	_ ifaces.ValidationBuilder          = &headerValidations{}
)

// paramValidations adapts [oaispec.Parameter] to [ifaces.OperationValidationBuilder] so the
// SimpleSchema handler family can write level-0 parameter validations through a uniform target.
//
// The adapter is unexported — DispatchParamLevel0 is the entry point.
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
	sv.current.Enum = validations.ParseEnumValues(val, sv.current.Type, sv.current.Format)
}
func (sv paramValidations) SetDefault(val any) { sv.current.Default = val }
func (sv paramValidations) SetExample(val any) { sv.current.Example = val }

// headerValidations adapts *oaispec.Header to ifaces.ValidationBuilder for the response-header
// SimpleSchema dispatch.
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
func (sv headerValidations) SetMultipleOf(val float64)      { sv.current.MultipleOf = &val }
func (sv headerValidations) SetMinItems(val int64)          { sv.current.MinItems = &val }
func (sv headerValidations) SetMaxItems(val int64)          { sv.current.MaxItems = &val }
func (sv headerValidations) SetMinLength(val int64)         { sv.current.MinLength = &val }
func (sv headerValidations) SetMaxLength(val int64)         { sv.current.MaxLength = &val }
func (sv headerValidations) SetPattern(val string)          { sv.current.Pattern = val }
func (sv headerValidations) SetUnique(val bool)             { sv.current.UniqueItems = val }
func (sv headerValidations) SetCollectionFormat(val string) { sv.current.CollectionFormat = val }
func (sv headerValidations) SetEnum(val string) {
	sv.current.Enum = validations.ParseEnumValues(val, sv.current.Type, sv.current.Format)
}
func (sv headerValidations) SetDefault(val any) { sv.current.Default = val }
func (sv headerValidations) SetExample(val any) { sv.current.Example = val }

// paramRequiredBool returns a Walker.Bool callback that writes the `required:` keyword straight
// onto a parameter target.
//
// Parameter- specific — schema writes required onto the enclosing schema keyed by name, headers
// don't carry `required:` at all.
func paramRequiredBool(param *oaispec.Parameter) func(grammar.Property, bool) {
	return func(pr grammar.Property, val bool) {
		if !pr.IsTyped() {
			return
		}
		if pr.Keyword.Name == grammar.KwRequired {
			param.Required = val
		}
	}
}

// DispatchParamLevel0 routes every level-0 Property in block onto param via the grammar Walker.
//
// Handler wiring is the SimpleSchema surface:
// Number/Integer/UniqueBool+RequiredBool/Pattern+CollectionFmt/ Raw-with-errSink/Extension.
//
// firstErr captures the first parse error from default/example coercion; callers surface it as a
// build error. diag may be nil (parser diagnostics dropped when so).
func DispatchParamLevel0(block grammar.Block, param *oaispec.Parameter, diag func(grammar.Diagnostic)) error {
	valid := paramValidations{param}
	scheme := &param.SimpleSchema
	var firstErr error

	block.Walk(grammar.Walker{
		FilterDepth: 0,
		Number:      Number(valid),
		Integer:     Integer(valid, diag),
		Bool: ComposeBool(
			UniqueBool(valid),
			paramRequiredBool(param),
		),
		String: ComposeString(
			PatternString(valid),
			CollectionFormatString(valid),
			UnsupportedSimpleSchemaString(diag),
		),
		Raw: Raw(valid, scheme, func(err error) bool {
			firstErr = err
			return true
		}, diag),
		Extension:  Extension(param),
		Diagnostic: diag,
	})

	return firstErr
}

// DispatchHeaderLevel0 routes every level-0 Property in block onto header via the grammar Walker.
//
// Mirrors DispatchParamLevel0 minus `required:` (headers don't carry it) and wires errSink=nil so
// malformed default/example values on a header don't fail the build — see
// [§raw-errsink](./README.md#raw-errsink).
//
// diag may be nil; when nil, parser diagnostics are dropped.
func DispatchHeaderLevel0(block grammar.Block, header *oaispec.Header, diag func(grammar.Diagnostic)) {
	valid := headerValidations{header}
	scheme := &header.SimpleSchema

	block.Walk(grammar.Walker{
		FilterDepth: 0,
		Number:      Number(valid),
		Integer:     Integer(valid, diag),
		Bool:        UniqueBool(valid),
		String: ComposeString(
			PatternString(valid),
			CollectionFormatString(valid),
			UnsupportedSimpleSchemaString(diag),
		),
		Raw:        Raw(valid, scheme, nil, diag),
		Extension:  Extension(header),
		Diagnostic: diag,
	})
}

// DispatchItemsLevel dispatches Property entries at the given items depth onto target via the
// resolvers.ItemsValidations adapter.
//
// The shape is identical between parameter and response-header items chains — both write to
// *oaispec.Items via the items adapter — so a single function serves both consumers.
func DispatchItemsLevel(block grammar.Block, target *oaispec.Items, depth int, diag func(grammar.Diagnostic)) {
	valid := resolvers.NewItemsValidations(target)
	scheme := &target.SimpleSchema

	block.Walk(grammar.Walker{
		FilterDepth: depth,
		Number:      Number(valid),
		Integer:     Integer(valid, diag),
		Bool:        UniqueBool(valid),
		String: ComposeString(
			PatternString(valid),
			CollectionFormatString(valid),
			UnsupportedSimpleSchemaString(diag),
		),
		Raw:        Raw(valid, scheme, nil, diag),
		Diagnostic: diag,
	})
}
