// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"encoding/json"
	"go/ast"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/spec"
)

type SetEnum struct {
	builder ifaces.ValidationBuilder
	rx      *regexp.Regexp
}

func NewSetEnum(builder ifaces.ValidationBuilder, opts ...PrefixRxOption) *SetEnum {
	rx := rxEnumValidation
	for _, apply := range opts {
		rx = apply(rxEnumFmt)
	}

	return &SetEnum{
		builder: builder,
		rx:      rx,
	}
}

func (se *SetEnum) Matches(line string) bool {
	return se.rx.MatchString(line)
}

func (se *SetEnum) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	matches := se.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		se.builder.SetEnum(matches[1])
	}

	return nil
}

func parseValueFromSchema(s string, schema *spec.SimpleSchema) (any, error) {
	if schema == nil {
		return s, nil
	}

	switch strings.Trim(schema.TypeName(), "\"") {
	case "integer", "int", "int64", "int32", "int16":
		return strconv.Atoi(s)
	case "bool", "boolean":
		return strconv.ParseBool(s)
	case "number", "float64", "float32":
		return strconv.ParseFloat(s, 64)
	case "object":
		var obj map[string]any
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			return s, nil //nolint:nilerr // fallback: return raw string when JSON is invalid
		}
		return obj, nil
	case "array":
		var slice []any
		if err := json.Unmarshal([]byte(s), &slice); err != nil {
			return s, nil //nolint:nilerr // fallback: return raw string when JSON is invalid
		}
		return slice, nil
	default:
		return s, nil
	}
}

func parseEnumOld(val string, s *spec.SimpleSchema) []any {
	list := strings.Split(val, ",")
	interfaceSlice := make([]any, len(list))
	for i, d := range list {
		v, err := parseValueFromSchema(d, s)
		if err != nil {
			interfaceSlice[i] = d
			continue
		}

		interfaceSlice[i] = v
	}
	return interfaceSlice
}

func ParseEnum(val string, s *spec.SimpleSchema) []any {
	// obtain the raw elements of the list to latter process them with the parseValueFromSchema
	var rawElements []json.RawMessage
	if err := json.Unmarshal([]byte(val), &rawElements); err != nil {
		log.Print("WARNING: item list for enum is not a valid JSON array, using the old deprecated format")
		return parseEnumOld(val, s)
	}

	interfaceSlice := make([]any, len(rawElements))

	for i, d := range rawElements {
		ds, err := strconv.Unquote(string(d))
		if err != nil {
			ds = string(d)
		}

		v, err := parseValueFromSchema(ds, s)
		if err != nil {
			interfaceSlice[i] = ds
			continue
		}

		interfaceSlice[i] = v
	}

	return interfaceSlice
}

func GetEnumBasicLitValue(basicLit *ast.BasicLit) any {
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

func GetEnumDesc(extensions spec.Extensions) (desc string) {
	desc, _ = extensions.GetString(extEnumDesc)
	return desc
}

func EnumDescExtension() string {
	return extEnumDesc
}
