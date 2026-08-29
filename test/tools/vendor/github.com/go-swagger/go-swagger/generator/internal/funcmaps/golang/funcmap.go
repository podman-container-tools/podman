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
		"lineComment":        lineComment,
		"linePadComment":     linePadComment,
		"blockComment":       wrapBlockComment,
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
		// escapeDoubleQuoted escapes arg so it can be safely interpolated inside a
		// double-quoted Go string literal ("..."). strconv.Quote produces a fully
		// escaped literal; stripping its surrounding quotes yields the inner form,
		// so an embedded '"', '\' or newline in spec-derived text (e.g. an
		// operation path baked into a diagnostic message) cannot break out of the
		// literal. For text without special characters this is a no-op.
		"escapeDoubleQuoted": func(arg string) string {
			quoted := strconv.Quote(arg)

			return quoted[1 : len(quoted)-1]
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

// normalizeNewlines rewrites CRLF and lone CR to LF so comment helpers can split
// reliably on "\n".
func normalizeNewlines(str string) string {
	if !strings.ContainsRune(str, '\r') {
		return str
	}

	str = strings.ReplaceAll(str, "\r\n", "\n")

	return strings.ReplaceAll(str, "\r", "\n")
}

// lineComment renders its arguments as a complete Go line-comment block, emitting
// the "//" markers itself so call sites only pass content.
//
// Arguments are stringified and concatenated with fmt.Sprint semantics (a space is
// inserted only between two non-string operands); nil arguments are skipped, so
// optional template values drop out cleanly. This lets a composed comment be built
// inline, e.g. {{ lineComment "MinProperties: " .MinProperties }}.
//
// Besides factoring out comment construction, it harmonizes the output: every line
// is prefixed with "// " (a single space), CR/CRLF are normalized, and per-line
// trailing whitespace is trimmed. Blank lines are preserved (they matter to godoc
// and the spec re-scanner); a blank line left trailing the whole comment block is
// dropped by the Go formatter. Splitting on embedded newlines keeps multi-line
// content fully commented so it cannot break out of the comment, and the guaranteed
// space after "//" keeps content from accidentally (or maliciously) forming a
// compiler directive such as //go:embed or //line. Blank input yields no output.
func lineComment(args ...any) string {
	return renderLineComment("// ", args)
}

// linePadComment renders its arguments as a Go line-comment block like
// [lineComment], but indents every rendered line — including the continuation
// lines produced by a multi-line argument — by pad after the "//" marker. This
// preserves a fixed indentation across wrapped lines, as required by the
// indentation-significant swagger:meta package doc block.
//
// pad is the indentation that follows "//": a pad of "  " yields "//  text". A
// leading space is inserted when pad does not already start with whitespace, so
// the marker can never accidentally (or maliciously) form a compiler directive.
// Empty content yields no output.
func linePadComment(pad string, args ...any) string {
	if pad == "" || (pad[0] != ' ' && pad[0] != '\t') {
		pad = " " + pad
	}

	return renderLineComment("//"+pad, args)
}

// renderLineComment is the shared core of [lineComment] and [linePadComment].
//
// It stringifies args (fmt.Sprint semantics, nil arguments skipped), normalizes
// CR/CRLF, trims trailing whitespace from each line, then prefixes every line with
// marker (blank lines become a bare "//"). Splitting on embedded newlines keeps
// multi-line content fully commented so it cannot break out of the comment.
//
// Blank lines are preserved, including a blank line trailing the content: they
// carry meaning for godoc paragraphs and for the spec re-scanner. A blank line that
// ends up trailing the whole comment block is left to the Go formatter to drop.
// Content that is entirely blank yields no output.
// derefArg unwraps pointer arguments so they stringify by value, mirroring how
// text/template prints a pointer field with {{ .X }}. A nil interface or a nil
// pointer is reported as absent (ok=false) so the caller can skip it. This keeps
// composed comments such as {{ lineComment "MinProperties: " .MinProperties }}
// printing the int64 value rather than the *int64 address.
func derefArg(arg any) (any, bool) {
	if arg == nil {
		return nil, false
	}

	v := reflect.ValueOf(arg)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, false
		}

		v = v.Elem()
	}

	return v.Interface(), true
}

func renderLineComment(marker string, args []any) string {
	kept := make([]any, 0, len(args))
	for _, arg := range args {
		if deref, ok := derefArg(arg); ok {
			kept = append(kept, deref)
		}
	}

	str := normalizeNewlines(fmt.Sprint(kept...))
	if strings.TrimSpace(str) == "" {
		return ""
	}

	lines := strings.Split(str, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, " \t")
		if line == "" {
			lines[i] = "//"

			continue
		}

		lines[i] = marker + line
	}

	return strings.Join(lines, "\n")
}

// wrapBlockComment renders text as a complete Go block comment, emitting the
// "/*" and "*/" markers itself so call sites only pass the text.
//
// It neutralizes any inner "*/" so spec text cannot terminate the comment early,
// normalizes newlines and trims trailing whitespace. Single-line text stays
// inline ("/* text */"); multi-line text is wrapped on its own lines. Empty
// input yields no output.
func wrapBlockComment(str string) string {
	str = strings.TrimRight(normalizeNewlines(str), " \t\n")
	if str == "" {
		return ""
	}

	str = strings.ReplaceAll(str, "*/", "[*]/")

	if strings.ContainsRune(str, '\n') {
		return "/*\n" + str + "\n*/"
	}

	return "/* " + str + " */"
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
