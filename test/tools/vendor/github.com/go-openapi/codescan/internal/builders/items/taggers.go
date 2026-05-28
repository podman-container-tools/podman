// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package items

import (
	"fmt"
	"go/ast"
	"slices"

	"github.com/go-openapi/codescan/internal/parsers"
	"github.com/go-openapi/spec"
)

// Taggers builds tag parsers for array items at a given nesting level.
func Taggers(items *spec.Items, level int) []parsers.TagParser {
	return itemsTaggers(items, level)
}

func itemsTaggers(items *spec.Items, level int) []parsers.TagParser {
	opts := []parsers.PrefixRxOption{parsers.WithItemsPrefixLevel(level)}

	return []parsers.TagParser{
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMaximum", level), parsers.NewSetMaximum(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMinimum", level), parsers.NewSetMinimum(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMultipleOf", level), parsers.NewSetMultipleOf(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMinLength", level), parsers.NewSetMinLength(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMaxLength", level), parsers.NewSetMaxLength(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dPattern", level), parsers.NewSetPattern(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dCollectionFormat", level), parsers.NewSetCollectionFormat(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMinItems", level), parsers.NewSetMinItems(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMaxItems", level), parsers.NewSetMaxItems(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dUnique", level), parsers.NewSetUnique(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dEnum", level), parsers.NewSetEnum(Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dDefault", level), parsers.NewSetDefault(&items.SimpleSchema, Validations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dExample", level), parsers.NewSetExample(&items.SimpleSchema, Validations{items}, opts...)),
	}
}

// ParseArrayTypes recursively builds tag parsers for nested array types.
func ParseArrayTypes(taggers []parsers.TagParser, name string, expr ast.Expr, items *spec.Items, level int) ([]parsers.TagParser, error) {
	return parseArrayTypes(taggers, name, expr, items, level)
}

func parseArrayTypes(taggers []parsers.TagParser, name string, expr ast.Expr, items *spec.Items, level int) ([]parsers.TagParser, error) {
	if items == nil {
		return taggers, nil
	}

	switch iftpe := expr.(type) {
	case *ast.ArrayType:
		eleTaggers := itemsTaggers(items, level)
		return parseArrayTypes(slices.Concat(eleTaggers, taggers), name, iftpe.Elt, items.Items, level+1)

	case *ast.SelectorExpr:
		return parseArrayTypes(taggers, name, iftpe.Sel, items.Items, level+1)

	case *ast.Ident:
		var identTaggers []parsers.TagParser
		if iftpe.Obj == nil {
			identTaggers = itemsTaggers(items, level)
		}

		otherTaggers, err := parseArrayTypes(taggers, name, expr, items.Items, level+1)
		if err != nil {
			return nil, err
		}

		return slices.Concat(identTaggers, otherTaggers), nil

	case *ast.StarExpr:
		return parseArrayTypes(taggers, name, iftpe.X, items, level)

	default:
		return nil, fmt.Errorf("unknown field type element for %q: %w", name, ErrItems)
	}
}
