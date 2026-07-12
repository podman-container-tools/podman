// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-openapi/inflect"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/swag"
)

func asJSON(data any) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func asPrettyJSON(data any) (string, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func pluralizeFirstWord(arg string) string {
	sentence := strings.Split(arg, " ")
	if len(sentence) == 1 {
		return inflect.Pluralize(arg)
	}

	return inflect.Pluralize(sentence[0]) + " " + strings.Join(sentence[1:], " ")
}

func dropPackage(str string) string {
	parts := strings.Split(str, ".")
	return parts[len(parts)-1]
}

// return true if the GoType str contains pkg. For example "model.MyType" -> true, "MyType" -> false.
func containsPkgStr(str string) bool {
	dropped := dropPackage(str)
	return dropped != str
}

func padSurround(entry, padWith string, i, ln int) string {
	res := make([]string, 0, i+max(ln-i-1, 0)+1)

	if i > 0 {
		for range i {
			res = append(res, padWith)
		}
	}

	res = append(res, entry)

	if ln > i {
		tot := ln - i - 1
		for range tot {
			res = append(res, padWith)
		}
	}

	return strings.Join(res, ",")
}

func padComment(str string, pads ...string) string {
	// pads specifes padding to indent multi line comments.Defaults to one space
	pad := " "
	lines := strings.Split(str, "\n")
	if len(pads) > 0 {
		pad = strings.Join(pads, "")
	}
	return (strings.Join(lines, "\n//"+pad))
}

func blockComment(str string) string {
	return strings.ReplaceAll(str, "*/", "[*]/")
}

func pascalize(arg string) string {
	runes := []rune(arg)
	switch len(runes) {
	case 0:
		return "Empty"
	case 1: // handle special case when we have a single rune that is not handled by swag.ToGoName
		switch runes[0] {
		case '+', '-', '#', '_', '*', '/', '=': // those cases are handled differently than swag utility
			return prefixForName(arg)
		}
	}
	return swag.ToGoName(swag.ToGoName(arg)) // want to remove spaces
}

func prefixForName(arg string) string {
	first := []rune(arg)[0]
	if len(arg) == 0 || unicode.IsLetter(first) {
		return ""
	}
	switch first {
	case '+':
		return "Plus"
	case '-':
		return "Minus"
	case '#':
		return "HashTag"
	case '*':
		return "Asterisk"
	case '/':
		return "ForwardSlash"
	case '=':
		return "EqualSign"
		// other cases ($,@ etc..) handled by swag.ToGoName
	}
	return "Nr"
}

func replaceSpecialChar(in rune) string {
	switch in {
	case '.':
		return "-Dot-"
	case '+':
		return "-Plus-"
	case '-':
		return "-Dash-"
	case '#':
		return "-Hashtag-"
	}
	return string(in)
}

func cleanupEnumVariant(in string) string {
	var replaced strings.Builder

	for _, char := range in {
		replaced.WriteString(replaceSpecialChar(char))
	}

	return replaced.String()
}

func dict(values ...any) (map[string]any, error) {
	const pair = 2

	if len(values)%pair != 0 {
		return nil, fmt.Errorf("expected even number of arguments, got %d", len(values))
	}

	dict := make(map[string]any, len(values)/pair)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("expected string key, got %+v", values[i])
		}
		dict[key] = values[i+1]
	}

	return dict, nil
}

func isInteger(arg any) bool {
	// is integer determines if a value may be represented by an integer
	switch val := arg.(type) {
	case int8, int16, int32, int, int64, uint8, uint16, uint32, uint, uint64:
		return true
	case *int8, *int16, *int32, *int, *int64, *uint8, *uint16, *uint32, *uint, *uint64:
		v := reflect.ValueOf(arg)
		return !v.IsNil()
	case float64:
		return math.Round(val) == val
	case *float64:
		return val != nil && math.Round(*val) == *val
	case float32:
		return math.Round(float64(val)) == float64(val)
	case *float32:
		return val != nil && math.Round(float64(*val)) == float64(*val)
	case string:
		_, err := strconv.ParseInt(val, 10, 64)
		return err == nil
	case *string:
		if val == nil {
			return false
		}
		_, err := strconv.ParseInt(*val, 10, 64)
		return err == nil
	default:
		return false
	}
}

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

func httpStatus(code int) string {
	if name, ok := runtime.Statuses[code]; ok {
		return name
	}
	// non-standard codes deserve some name
	return fmt.Sprintf("Status %d", code)
}

func gt0(in *int64) bool {
	// gt0 returns true if the *int64 points to a value > 0
	// NOTE: plain {{ gt .MinProperties 0 }} just refuses to work normally
	// with a pointer
	return in != nil && *in > 0
}

func errorPath(in any) (string, error) {
	// For schemas:
	// errorPath returns an empty string litteral when the schema path is empty.
	// It provides a shorthand for template statements such as:
	// {{ if .Path }}{{ .Path }}{{ else }}" "{{ end }},
	// which becomes {{ path . }}
	//
	// When called for a GenParameter, GenResponse or GenOperation object, it just
	// returns Path.
	//
	// Extra behavior for schemas, when the generation option RootedErroPath is enabled:
	// In the case of arrays with an empty path, it adds the type name as the path "root",
	// so consumers of reported errors get an idea of the originator.

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

const mdNewLine = "</br>"

var (
	mdNewLineReplacer = strings.NewReplacer("\r\n", mdNewLine, "\n", mdNewLine, "\r", mdNewLine)
	interfaceReplacer = strings.NewReplacer("interface {}", "any")
)

func markdownBlock(in string) string {
	in = strings.TrimSpace(in)

	return mdNewLineReplacer.Replace(in)
}
