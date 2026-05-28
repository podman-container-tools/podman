// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"fmt"
	"go/ast"
	"slices"

	"github.com/go-openapi/codescan/internal/parsers"
	oaispec "github.com/go-openapi/spec"
)

func schemaTaggers(schema, ps *oaispec.Schema, nm string) []parsers.TagParser {
	schemeType, err := ps.Type.MarshalJSON()
	if err != nil {
		return nil
	}
	scheme := &oaispec.SimpleSchema{Type: string(schemeType)}

	return []parsers.TagParser{
		// Match-only: claim `in: <location>` lines so they do not leak into the
		// schema description. `in:` only matters for parameter/response dispatch;
		// if it reaches a schema field (e.g. via the alias-expand path), it is
		// still metadata, not prose.
		parsers.NewSingleLineTagParser("in", parsers.NewMatchIn()),
		parsers.NewSingleLineTagParser("maximum", parsers.NewSetMaximum(schemaValidations{ps})),
		parsers.NewSingleLineTagParser("minimum", parsers.NewSetMinimum(schemaValidations{ps})),
		parsers.NewSingleLineTagParser("multipleOf", parsers.NewSetMultipleOf(schemaValidations{ps})),
		parsers.NewSingleLineTagParser("minLength", parsers.NewSetMinLength(schemaValidations{ps})),
		parsers.NewSingleLineTagParser("maxLength", parsers.NewSetMaxLength(schemaValidations{ps})),
		parsers.NewSingleLineTagParser("pattern", parsers.NewSetPattern(schemaValidations{ps})),
		parsers.NewSingleLineTagParser("minItems", parsers.NewSetMinItems(schemaValidations{ps})),
		parsers.NewSingleLineTagParser("maxItems", parsers.NewSetMaxItems(schemaValidations{ps})),
		parsers.NewSingleLineTagParser("unique", parsers.NewSetUnique(schemaValidations{ps})),
		parsers.NewSingleLineTagParser("enum", parsers.NewSetEnum(schemaValidations{ps})),
		parsers.NewSingleLineTagParser("default", parsers.NewSetDefault(scheme, schemaValidations{ps})),
		parsers.NewSingleLineTagParser("type", parsers.NewSetDefault(scheme, schemaValidations{ps})),
		parsers.NewSingleLineTagParser("example", parsers.NewSetExample(scheme, schemaValidations{ps})),
		parsers.NewSingleLineTagParser("required", parsers.NewSetRequiredSchema(schema, nm)),
		parsers.NewSingleLineTagParser("readOnly", parsers.NewSetReadOnlySchema(ps)),
		parsers.NewSingleLineTagParser("discriminator", parsers.NewSetDiscriminator(schema, nm)),
		parsers.NewMultiLineTagParser("YAMLExtensionsBlock", parsers.NewYAMLParser(
			parsers.WithExtensionMatcher(),
			parsers.WithSetter(schemaVendorExtensibleSetter(ps)),
		), true),
	}
}

func refSchemaTaggers(schema *oaispec.Schema, name string) []parsers.TagParser {
	return []parsers.TagParser{
		parsers.NewSingleLineTagParser("required", parsers.NewSetRequiredSchema(schema, name)),
	}
}

func itemsTaggers(items *oaispec.Schema, level int) []parsers.TagParser {
	schemeType, err := items.Type.MarshalJSON()
	if err != nil {
		return nil
	}

	scheme := &oaispec.SimpleSchema{Type: string(schemeType)}
	opts := []parsers.PrefixRxOption{parsers.WithItemsPrefixLevel(level)}

	return []parsers.TagParser{
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMaximum", level), parsers.NewSetMaximum(schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMinimum", level), parsers.NewSetMinimum(schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMultipleOf", level), parsers.NewSetMultipleOf(schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMinLength", level), parsers.NewSetMinLength(schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMaxLength", level), parsers.NewSetMaxLength(schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dPattern", level), parsers.NewSetPattern(schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMinItems", level), parsers.NewSetMinItems(schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dMaxItems", level), parsers.NewSetMaxItems(schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dUnique", level), parsers.NewSetUnique(schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dEnum", level), parsers.NewSetEnum(schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dDefault", level), parsers.NewSetDefault(scheme, schemaValidations{items}, opts...)),
		parsers.NewSingleLineTagParser(fmt.Sprintf("items%dExample", level), parsers.NewSetExample(scheme, schemaValidations{items}, opts...)),
	}
}

func parseArrayTypes(taggers []parsers.TagParser, expr ast.Expr, items *oaispec.SchemaOrArray, level int) ([]parsers.TagParser, error) {
	if items == nil || items.Schema == nil {
		return taggers, nil
	}

	switch iftpe := expr.(type) {
	case *ast.ArrayType:
		eleTaggers := itemsTaggers(items.Schema, level)
		otherTaggers, err := parseArrayTypes(slices.Concat(eleTaggers, taggers), iftpe.Elt, items.Schema.Items, level+1)
		if err != nil {
			return nil, err
		}

		return otherTaggers, nil

	case *ast.Ident:
		var identTaggers []parsers.TagParser
		if iftpe.Obj == nil {
			identTaggers = itemsTaggers(items.Schema, level)
		}

		otherTaggers, err := parseArrayTypes(taggers, expr, items.Schema.Items, level+1)
		if err != nil {
			return nil, err
		}

		return slices.Concat(identTaggers, otherTaggers), nil

	case *ast.StarExpr:
		return parseArrayTypes(taggers, iftpe.X, items, level)

	case *ast.SelectorExpr:
		// qualified name (e.g. time.Time): terminal leaf, register items-level validations.
		return slices.Concat(itemsTaggers(items.Schema, level), taggers), nil

	case *ast.StructType, *ast.InterfaceType, *ast.MapType:
		// anonymous struct / interface / map element: no further items-level
		// validations apply; the element type itself carries its schema.
		return taggers, nil

	default:
		return nil, fmt.Errorf("unknown field type element: %w", ErrSchema)
	}
}
