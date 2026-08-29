// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"go/ast"
	"strconv"
	"strings"
)

// enumBasicLitValue converts the RHS of a `const Foo Kind = "bar"` declaration into its runtime
// value — int64 / float64 / unquoted string — for emission as an enum entry on the Swagger
// schema the scanner is building.
//
// Returns nil when the literal kind is INT or FLOAT but the textual value fails to parse (rare —
// Go's own parser would have caught it upstream, but the safety net is cheap).
func enumBasicLitValue(basicLit *ast.BasicLit) any {
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
