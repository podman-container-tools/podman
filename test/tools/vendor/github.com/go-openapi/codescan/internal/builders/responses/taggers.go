// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package responses

import (
	"go/ast"
	"slices"

	"github.com/go-openapi/codescan/internal/builders/items"
	"github.com/go-openapi/codescan/internal/parsers"
	oaispec "github.com/go-openapi/spec"
)

// baseResponseHeaderTaggers configures taggers for a response header field.
func baseResponseHeaderTaggers(header *oaispec.Header) []parsers.TagParser {
	return []parsers.TagParser{
		// Match-only: claim `in: header` so it does not leak into the header's description.
		parsers.NewSingleLineTagParser("in", parsers.NewMatchIn()),
		parsers.NewSingleLineTagParser("maximum", parsers.NewSetMaximum(headerValidations{header})),
		parsers.NewSingleLineTagParser("minimum", parsers.NewSetMinimum(headerValidations{header})),
		parsers.NewSingleLineTagParser("multipleOf", parsers.NewSetMultipleOf(headerValidations{header})),
		parsers.NewSingleLineTagParser("minLength", parsers.NewSetMinLength(headerValidations{header})),
		parsers.NewSingleLineTagParser("maxLength", parsers.NewSetMaxLength(headerValidations{header})),
		parsers.NewSingleLineTagParser("pattern", parsers.NewSetPattern(headerValidations{header})),
		parsers.NewSingleLineTagParser("collectionFormat", parsers.NewSetCollectionFormat(headerValidations{header})),
		parsers.NewSingleLineTagParser("minItems", parsers.NewSetMinItems(headerValidations{header})),
		parsers.NewSingleLineTagParser("maxItems", parsers.NewSetMaxItems(headerValidations{header})),
		parsers.NewSingleLineTagParser("unique", parsers.NewSetUnique(headerValidations{header})),
		parsers.NewSingleLineTagParser("enum", parsers.NewSetEnum(headerValidations{header})),
		parsers.NewSingleLineTagParser("default", parsers.NewSetDefault(&header.SimpleSchema, headerValidations{header})),
		parsers.NewSingleLineTagParser("example", parsers.NewSetExample(&header.SimpleSchema, headerValidations{header})),
	}
}

func setupResponseHeaderTaggers(header *oaispec.Header, name string, afld *ast.Field) ([]parsers.TagParser, error) {
	// TODO(claude): don't understand why we need this step. Isn't it handled by the recursion already?
	if ftped, ok := afld.Type.(*ast.ArrayType); ok {
		taggers, err := items.ParseArrayTypes([]parsers.TagParser{}, name, ftped.Elt, header.Items, 0)
		if err != nil {
			return nil, err
		}

		return slices.Concat(taggers, baseResponseHeaderTaggers(header)), nil
	}

	return baseResponseHeaderTaggers(header), nil
}
