// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
)

func resolvedDocCollectionFormat(cf string, child *GenItems) string {
	if child == nil {
		return cf
	}
	ccf := cf
	if ccf == "" {
		ccf = "csv"
	}
	rcf := resolvedDocCollectionFormat(child.CollectionFormat, child.Child)
	if rcf == "" {
		return ccf
	}
	return ccf + "|" + rcf
}

func resolvedDocType(tn, ft string, child *GenItems) string {
	if tn == array {
		if child == nil {
			return "[]any"
		}
		return "[]" + resolvedDocType(child.SwaggerType, child.SwaggerFormat, child.Child)
	}

	if ft != "" {
		if doc, ok := docFormat[ft]; ok {
			return doc
		}
		return fmt.Sprintf("%s (formatted %s)", ft, tn)
	}

	return tn
}

func resolvedDocSchemaType(tn, ft string, child *GenSchema) string {
	if tn == array {
		if child == nil {
			return "[]any"
		}
		return "[]" + resolvedDocSchemaType(child.SwaggerType, child.SwaggerFormat, child.Items)
	}

	if tn == object {
		if child == nil || child.ElemType == nil {
			return "map of any"
		}
		if child.IsMap {
			return "map of " + resolvedDocElemType(child.SwaggerType, child.SwaggerFormat, &child.resolvedType)
		}

		return child.GoType
	}

	if ft != "" {
		if doc, ok := docFormat[ft]; ok {
			return doc
		}
		return fmt.Sprintf("%s (formatted %s)", ft, tn)
	}

	return tn
}

func resolvedDocElemType(tn, ft string, schema *resolvedType) string {
	if schema == nil {
		return ""
	}
	if schema.IsMap {
		return "map of " + resolvedDocElemType(schema.ElemType.SwaggerType, schema.ElemType.SwaggerFormat, schema.ElemType)
	}

	if schema.IsArray {
		return "[]" + resolvedDocElemType(schema.ElemType.SwaggerType, schema.ElemType.SwaggerFormat, schema.ElemType)
	}

	if ft != "" {
		if doc, ok := docFormat[ft]; ok {
			return doc
		}
		return fmt.Sprintf("%s (formatted %s)", ft, tn)
	}

	return tn
}

func errorPath(in any) (string, error) {
	var pth string
	rooted := func(schema GenSchema) string {
		if schema.WantsRootedErrorPath && schema.Path == "" && (schema.IsArray || schema.IsMap) {
			return `"[` + schema.Name + `]"`
		}

		return schema.Path
	}

	switch schema := in.(type) {
	case GenSchema:
		pth = rooted(schema)
	case *GenSchema:
		if schema == nil {
			break
		}
		pth = rooted(*schema)
	case GenDefinition:
		pth = rooted(schema.GenSchema)
	case *GenDefinition:
		if schema == nil {
			break
		}
		pth = rooted(schema.GenSchema)
	case GenParameter:
		pth = schema.Path

	// unchanged Path if called with other types
	case *GenParameter:
		if schema == nil {
			break
		}
		pth = schema.Path
	case GenResponse:
		pth = schema.Path
	case *GenResponse:
		if schema == nil {
			break
		}
		pth = schema.Path
	case GenOperation:
		pth = schema.Path
	case *GenOperation:
		if schema == nil {
			break
		}
		pth = schema.Path
	case GenItems:
		pth = schema.Path
	case *GenItems:
		if schema == nil {
			break
		}
		pth = schema.Path
	case GenHeader:
		pth = schema.Path
	case *GenHeader:
		if schema == nil {
			break
		}
		pth = schema.Path
	default:
		return "", fmt.Errorf("errorPath should be called with GenSchema or GenDefinition, but got %T", schema)
	}

	if pth == "" {
		return `""`, nil
	}

	return pth, nil
}
