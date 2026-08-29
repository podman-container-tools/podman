// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
	oaispec "github.com/go-openapi/spec"
)

// classifierPatternProperties applies a decl-level `swagger:patternProperties "<re>": <spec>, …`
// marker onto a top-level model schema.
//
// Each pair maps a quoted property-name regex to a typed value schema (`<spec>` uses the
// swagger:type-style grammar, type-name → $ref) — the typed counterpart of the regex-only
// `patternProperties:` field keyword, which sets an empty value schema.
//
// Like additionalProperties, it only rides on an object (lowest-priority precedence) and replaces a
// bare $ref with a clean object.
// Each regex is RE2-hygiene-checked: an invalid regex is preserved on the schema but raises a
// CodeInvalidAnnotation warning (never dropped silently).
//
// See [§pattern-properties](./README.md#pattern-properties).
func (s *Builder) classifierPatternProperties(schema *oaispec.Schema, pos token.Position) {
	arg, ok := s.findRawAnnotationArg(s.Decl.Comments, grammar.AnnPatternProperties)
	if !ok {
		return
	}

	if len(schema.Type) > 0 && !schema.Type.Contains("object") {
		s.RecordDiagnostic(grammar.Warnf(pos, grammar.CodeShapeMismatch,
			"swagger:patternProperties is only valid on an object schema; type is %q; ignored",
			schema.Type[0]))
		return
	}

	pairs, ok := parsePatternPropertyPairs(arg)
	if !ok || len(pairs) == 0 {
		s.RecordDiagnostic(grammar.Warnf(pos, grammar.CodeInvalidAnnotation,
			"swagger:patternProperties: malformed pair list %q; expected `\"<regex>\": <type>, …`", arg))
		return
	}

	if schema.Ref.String() != "" {
		schema.Ref = oaispec.Ref{}
	}
	schema.Typed("object", "")
	if schema.PatternProperties == nil {
		schema.PatternProperties = make(oaispec.SchemaProperties)
	}

	for _, pr := range pairs {
		valSchema := new(oaispec.Schema)
		// Cross-ref linkage: each value lands at <base>/patternProperties/<regex> (per-pair, not a shared
		// additionalProperties node), so anchors emitted while resolving an inlined value resolve under
		// the right pointer.
		restore := s.descend("patternProperties", pr.regex)
		resolved := s.resolveAdditionalPropertiesType(pr.spec, valSchema, pos)
		restore()
		if !resolved {
			continue // diagnostic already recorded
		}
		schema.PatternProperties[pr.regex] = *valSchema

		// RE2-hygiene check, mirroring the patternProperties: keyword wording.
		if _, err := regexp.Compile(pr.regex); err != nil {
			s.RecordDiagnostic(grammar.Warnf(pos, grammar.CodeInvalidAnnotation,
				"patternProperties: %s is not a valid Go RE2 regex (%v); "+
					"the value is preserved on the schema but downstream RE2 validators will fail",
				pr.regex, err))
		}
	}
}

// patternPropPair is one `"<regex>": <spec>` entry of a swagger:patternProperties marker.
type patternPropPair struct {
	regex string
	spec  string
}

// parsePatternPropertyPairs parses `"<re>": <spec>, "<re>": <spec>` into pairs.
//
// The regex is double-quoted so it may contain commas/colons/spaces; only `\"` is an escape inside
// it (every other backslash is preserved verbatim — `\d` stays `\d`).
// The spec runs from the colon to the next top-level comma.
// Returns ok=false on a structural error.
func parsePatternPropertyPairs(arg string) (pairs []patternPropPair, ok bool) {
	i := 0
	for i < len(arg) {
		for i < len(arg) && (arg[i] == ' ' || arg[i] == '\t' || arg[i] == ',') {
			i++
		}
		if i >= len(arg) {
			break
		}
		if arg[i] != '"' {
			return nil, false
		}
		i++ // opening quote

		var re strings.Builder
		for i < len(arg) && arg[i] != '"' {
			if arg[i] == '\\' && i+1 < len(arg) && arg[i+1] == '"' {
				re.WriteByte('"')
				i += 2
				continue
			}
			re.WriteByte(arg[i])
			i++
		}
		if i >= len(arg) {
			return nil, false // unterminated regex
		}
		i++ // closing quote

		for i < len(arg) && (arg[i] == ' ' || arg[i] == '\t') {
			i++
		}
		if i >= len(arg) || arg[i] != ':' {
			return nil, false
		}
		i++ // colon

		start := i
		for i < len(arg) && arg[i] != ',' {
			i++
		}
		spec := strings.TrimSpace(arg[start:i])
		if spec == "" {
			return nil, false
		}
		pairs = append(pairs, patternPropPair{regex: re.String(), spec: spec})
	}
	return pairs, true
}

// findRawAnnotationArg is the unfiltered sibling of findAnnotationArg: it returns the first
// argument of the first block of kind verbatim, including whitespace-bearing args (the
// swagger:patternProperties pair list).
//
// The single-word filter is intentionally not applied.
func (s *Builder) findRawAnnotationArg(cg *ast.CommentGroup, kind grammar.AnnotationKind) (string, bool) {
	for _, b := range s.ParseBlocks(cg) {
		if b.AnnotationKind() != kind {
			continue
		}
		if arg, ok := b.AnnotationArg(); ok {
			return arg, true
		}
	}
	return "", false
}
