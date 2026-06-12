// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parameters

import (
	"go/ast"
	"slices"

	"github.com/go-openapi/codescan/internal/builders/items"
	"github.com/go-openapi/codescan/internal/parsers"
	oaispec "github.com/go-openapi/spec"
)

func setupParamTaggers(param *oaispec.Parameter, name string, afld *ast.Field, skipExt, debug bool) ([]parsers.TagParser, error) {
	// Parameter-level $ref (e.g. {$ref: "#/parameters/X"}) is not emitted by
	// the scanner today — named struct fields become body params with a
	// schema-level ref (ps.Schema.Ref), never ps.Ref. To support
	// operation-level parameter refs, branch here on
	// `param.Ref.String() != ""` and dispatch to a narrower tagger set
	// (in, required, extensions only).
	return setupInlineParamTaggers(param, name, afld, skipExt, debug)
}

// baseInlineParamTaggers configures taggers for a fully-defined inline parameter.
func baseInlineParamTaggers(param *oaispec.Parameter, skipExt, debug bool) []parsers.TagParser {
	return []parsers.TagParser{
		parsers.NewSingleLineTagParser("in", parsers.NewMatchParamIn(param)),
		parsers.NewSingleLineTagParser("maximum", parsers.NewSetMaximum(paramValidations{param})),
		parsers.NewSingleLineTagParser("minimum", parsers.NewSetMinimum(paramValidations{param})),
		parsers.NewSingleLineTagParser("multipleOf", parsers.NewSetMultipleOf(paramValidations{param})),
		parsers.NewSingleLineTagParser("minLength", parsers.NewSetMinLength(paramValidations{param})),
		parsers.NewSingleLineTagParser("maxLength", parsers.NewSetMaxLength(paramValidations{param})),
		parsers.NewSingleLineTagParser("pattern", parsers.NewSetPattern(paramValidations{param})),
		parsers.NewSingleLineTagParser("collectionFormat", parsers.NewSetCollectionFormat(paramValidations{param})),
		parsers.NewSingleLineTagParser("minItems", parsers.NewSetMinItems(paramValidations{param})),
		parsers.NewSingleLineTagParser("maxItems", parsers.NewSetMaxItems(paramValidations{param})),
		parsers.NewSingleLineTagParser("unique", parsers.NewSetUnique(paramValidations{param})),
		parsers.NewSingleLineTagParser("enum", parsers.NewSetEnum(paramValidations{param})),
		parsers.NewSingleLineTagParser("default", parsers.NewSetDefault(&param.SimpleSchema, paramValidations{param})),
		parsers.NewSingleLineTagParser("example", parsers.NewSetExample(&param.SimpleSchema, paramValidations{param})),
		parsers.NewSingleLineTagParser("required", parsers.NewSetRequiredParam(param)),
		parsers.NewMultiLineTagParser("Extensions", parsers.NewSetExtensions(spExtensionsSetter(param, skipExt), debug), true),
	}
}

func setupInlineParamTaggers(param *oaispec.Parameter, name string, afld *ast.Field, skipExt, debug bool) ([]parsers.TagParser, error) {
	// TODO(claude): don't understand why we need this step. Isn't it handled by the recursion already?
	if ftped, ok := afld.Type.(*ast.ArrayType); ok {
		taggers, err := items.ParseArrayTypes([]parsers.TagParser{}, name, ftped.Elt, param.Items, 0)
		if err != nil {
			return nil, err
		}
		return slices.Concat(taggers, baseInlineParamTaggers(param, skipExt, debug)), nil
	}

	return baseInlineParamTaggers(param, skipExt, debug), nil
}
