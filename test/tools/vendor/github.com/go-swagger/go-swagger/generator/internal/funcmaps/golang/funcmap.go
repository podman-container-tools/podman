// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package golang provides the Go-specific template function map used by the
// go-swagger code generator. Functions defined here are pure utilities with
// no dependency on the generator's own types (GenSchema, GenOperation, etc.).
package golang

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/Masterminds/sprig/v3"
	"github.com/kr/pretty"

	"github.com/go-openapi/inflect"
	"github.com/go-openapi/runtime"
	"github.com/go-openapi/swag/mangling"
	"github.com/go-openapi/swag/stringutils"
)

// FuncMap returns a template.FuncMap containing all Go-specific template
// functions that are independent of generator types. Callers typically
// merge additional entries (e.g. LanguageOpts-dependent or type-dependent
// functions) on top.
func FuncMap(mangler mangling.NameMangler) template.FuncMap {
	f := sprig.TxtFuncMap()
	pascalize := pascalize(mangler)
	mediaGoName := mediaGoName(mangler)

	extra := template.FuncMap{
		"pascalize":          pascalize,
		"camelize":           mangler.ToJSONName,
		"humanize":           mangler.ToHumanNameLower,
		"dasherize":          mangler.ToCommandName,
		"pluralizeFirstWord": pluralizeFirstWord,
		"json":               asJSON,
		"prettyjson":         asPrettyJSON,
		"hasInsecure": func(arg []string) bool {
			return stringutils.ContainsStringsCI(arg, "http") || stringutils.ContainsStringsCI(arg, "ws")
		},
		"hasSecure": func(arg []string) bool {
			return stringutils.ContainsStringsCI(arg, "https") || stringutils.ContainsStringsCI(arg, "wss")
		},
		"dropPackage":        dropPackage,
		"containsPkgStr":     containsPkgStr,
		"contains":           slices.Contains[[]string, string],
		"padSurround":        padSurround,
		"joinFilePath":       filepath.Join,
		"joinPath":           path.Join,
		"comment":            padComment,
		"blockcomment":       blockComment,
		"inspect":            pretty.Sprint,
		"cleanPath":          path.Clean,
		"mediaTypeName":      mediaMime,
		"mediaGoName":        mediaGoName,
		"dict":               dict,
		"isInteger":          isInteger,
		"hasPrefix":          strings.HasPrefix,
		"stringContains":     strings.Contains,
		"trimSpace":          strings.TrimSpace,
		"mdBlock":            markdownBlock,
		"httpStatus":         httpStatus,
		"cleanupEnumVariant": cleanupEnumVariant,
		"gt0":                gt0,
		"escapeBackticks": func(arg string) string {
			return strings.ReplaceAll(arg, "`", "`+\"`\"+`")
		},
		"flagNameVar": func(in string) string {
			return fmt.Sprintf("flag%sName", pascalize(in))
		},
		"flagValueVar": func(in string) string {
			return fmt.Sprintf("flag%sValue", pascalize(in))
		},
		"flagDefaultVar": func(in string) string {
			return fmt.Sprintf("flag%sDefault", pascalize(in))
		},
		"flagModelVar": func(in string) string {
			return fmt.Sprintf("flag%sModel", pascalize(in))
		},
		"flagDescriptionVar": func(in string) string {
			return fmt.Sprintf("flag%sDescription", pascalize(in))
		},
		"printGoLiteral": func(in any) string {
			return interfaceReplacer.Replace(fmt.Sprintf("%#v", in))
		},
		"fold": func(in string) string {
			return foldReplacer.Replace(in)
		},
	}

	maps.Copy(f, extra)

	return f
}

var foldReplacer = strings.NewReplacer("\n", " ", "\r", "")

// pascalize converts a name to Go PascalCase, handling special prefix characters.
func pascalize(mangler mangling.NameMangler) func(string) string {
	return func(arg string) string {
		runes := []rune(arg)
		switch len(runes) {
		case 0:
			return "Empty"
		case 1:
			switch runes[0] {
			case '+', '-', '#', '_', '*', '/', '=':
				return PrefixForName(arg)
			}
		}

		return mangler.ToGoName(mangler.ToGoName(arg))
	}
}

// PrefixForName returns a human-readable prefix for names starting with
// special characters. It is used as [mangling.PrefixFunc].
func PrefixForName(arg string) string {
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
	case '=':
		return "-Equal-"
	case '!':
		return "-Bang-"
	case '~':
		return "-Tilde-"
	case '>':
		return "-GreaterThan-"
	case '<':
		return "-LessThan-"
	case '*':
		return "-Star-"
	case '/':
		return "-Slash-"
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

// asJSON marshals data to a compact JSON string.
func asJSON(data any) (string, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// asPrettyJSON marshals data to an indented JSON string.
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

// dropPackage returns the last component of a dot-separated name.
func dropPackage(str string) string {
	parts := strings.Split(str, ".")
	return parts[len(parts)-1]
}

// containsPkgStr returns true if str contains a package qualifier (e.g. "model.MyType").
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
	pad := " "
	lines := strings.Split(str, "\n")
	if len(pads) > 0 {
		pad = strings.Join(pads, "")
	}

	return strings.Join(lines, "\n//"+pad)
}

func blockComment(str string) string {
	return strings.ReplaceAll(str, "*/", "[*]/")
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
		dict[key] = values[i+1] // bounds checked by the modulo guard above
	}

	return dict, nil
}

func isInteger(arg any) bool {
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

func httpStatus(code int) string {
	if name, ok := runtime.Statuses[code]; ok {
		return name
	}

	return fmt.Sprintf("Status %d", code)
}

func gt0(in *int64) bool {
	return in != nil && *in > 0
}

const (
	mdNewLine      = "</br>"
	mimeParamParts = 2
)

var (
	mdNewLineReplacer = strings.NewReplacer("\r\n", mdNewLine, "\n", mdNewLine, "\r", mdNewLine)
	interfaceReplacer = strings.NewReplacer("interface {}", "any")
)

func markdownBlock(in string) string {
	in = strings.TrimSpace(in)

	return mdNewLineReplacer.Replace(in)
}

// mediaMime extracts the MIME type from a media type string, stripping
// any parameters after the first semicolon.
func mediaMime(orig string) string {
	return strings.SplitN(orig, ";", mimeParamParts)[0]
}

// mediaGoName converts a MIME media type string to a Go-style PascalCase name.
func mediaGoName(mangler mangling.NameMangler) func(string) string {
	return func(media string) string {
		return pascalize(mangler)(strings.ReplaceAll(media, "*", "Star"))
	}
}
