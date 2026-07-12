// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codescan

import (
	"go/ast"
	"strconv"
	"strings"

	"github.com/go-openapi/spec"
)

func getEnumBasicLitValue(basicLit *ast.BasicLit) any {
	switch basicLit.Kind.String() {
	case "INT":
		if result, err := strconv.ParseInt(basicLit.Value, 10, 64); err == nil {
			return result
		}
	case "FLOAT":
		if result, err := strconv.ParseFloat(basicLit.Value, 64); err == nil {
			return result
		}
	default:
		return strings.Trim(basicLit.Value, "\"")
	}
	return nil
}

const extEnumDesc = "x-go-enum-desc"

func getEnumDesc(extensions spec.Extensions) (desc string) {
	desc, _ = extensions.GetString(extEnumDesc)
	return desc
}
