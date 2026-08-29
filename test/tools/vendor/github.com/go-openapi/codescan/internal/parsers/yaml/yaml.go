// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package yaml is a thin wrapper around go.yaml.in/yaml/v3 for consuming the RawYAML bodies that
// internal/parsers/grammar/ isolates between `---` fences, plus the typed-extensions service the
// grammar lexer calls for `extensions:` raw blocks.
//
// Importers:
//
//   - The builder layer — bridge taggers that decide when to parse a
//     given RawYAML body (operations, meta, schema).
//   - internal/parsers/grammar — calls [TypedExtensions] from its
//     extensions raw-block lexer so Extension.Value ships typed.
//
// The grammar import is the one carve-out from the "grammar stays YAML-free" architecture rule.
//
// See README.md for the long-form rationale (typed-extensions pipeline, dedent strategies,
// sibling-sub-parser seam).
package yaml

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/go-openapi/swag/yamlutils"
)

// Parse unmarshals the given raw YAML body into a generic value
// (typically a map[string]any or []any). YAML library errors carry
// their own line/column numbers relative to the body, not to the Go
// source — callers that need source-relative positions wrap the error
// with their own annotation position.
//
// Returns (nil, nil) for an empty body so callers can handle
// "annotation had a fence but no content" without branching on
// error-vs-nil.
//
//nolint:nilnil // (nil, nil) is the documented "empty body" return — the caller distinguishes via len(body) if needed.
func Parse(body string) (any, error) {
	if body == "" {
		return nil, nil
	}
	var v any
	if err := decodeYAMLBody([]byte(body), &v); err != nil {
		return nil, err
	}
	return v, nil
}

// ParseInto unmarshals body into the given destination, typically a pointer to a struct the caller
// defined to match an expected YAML shape (e.g., operation-body or extension-value).
//
// Wraps the underlying error for uniform error reporting.
func ParseInto(body string, dst any) error {
	if body == "" {
		return nil
	}
	return decodeYAMLBody([]byte(body), dst)
}

// TypedExtensions parses the body of an `extensions:` raw block and returns its top-level entries
// as JSON-typed values (bool / float64 / string / []any / map[string]any).
//
// The body is dedented before parsing — the grammar lexer preserves godoc-level indentation per
// line, but YAML refuses tab indentation and treats leading whitespace as structural.
//
// The YAML → JSON normalisation enforces map[string]any with concrete leaf types via
// swag/yamlutils.YAMLToJSON; downstream consumers (vendor-extension targets, code generators,
// AddExtension surfaces) rely on that shape.
//
// No name filtering is applied here: the caller decides whether to accept only x-* keys (via
// classify.IsAllowedExtension) or to consume the full mapping.
// Empty body returns (nil, nil).
//
// See README.md §typed-extensions for the full contract.
func TypedExtensions(body string) (map[string]any, error) {
	if body == "" {
		return nil, nil //nolint:nilnil // documented "empty body" return
	}
	normalised := normaliseExtensionBody(body)
	var yamlValue any
	if err := decodeYAMLBody([]byte(normalised), &yamlValue); err != nil {
		return nil, err
	}
	jsonValue, err := yamlutils.YAMLToJSON(yamlValue)
	if err != nil {
		return nil, fmt.Errorf("yaml→json: %w", err)
	}
	var data map[string]any
	if err := json.Unmarshal(jsonValue, &data); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}
	return data, nil
}

// normaliseExtensionBody dedents an extensions-block body: strips the common leading-whitespace
// prefix shared by every non-blank line and substitutes any residual leading tabs with two spaces
// (YAML refuses tab indentation).
//
// The grammar lexer preserves each line's original whitespace so that indentation survives for
// nested YAML; the dedent therefore lives downstream of it.
//
// Tab-and-space mixes in godoc-style sources parse identically after this pass — both petstore's
// tab-indented Extensions block and the typed-nested case using two-space indentation round-trip
// uniformly.
func normaliseExtensionBody(body string) string {
	lines := strings.Split(body, "\n")

	prefix := commonLeadingWhitespace(lines)
	if prefix > 0 {
		for i, line := range lines {
			if len(line) >= prefix {
				lines[i] = line[prefix:]
			}
		}
	}

	for i, line := range lines {
		lines[i] = replaceLeadingTabs(line)
	}
	return strings.Join(lines, "\n")
}

// commonLeadingWhitespace returns the length of the longest leading whitespace run shared by every
// non-blank line.
//
// Returns 0 when any non-blank line starts at column 0 (nothing to dedent).
func commonLeadingWhitespace(lines []string) int {
	prefix := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		n := 0
		for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
			n++
		}
		if prefix < 0 || n < prefix {
			prefix = n
		}
		if prefix == 0 {
			return 0
		}
	}
	if prefix < 0 {
		return 0
	}
	return prefix
}

// replaceLeadingTabs converts any tab characters in the leading whitespace run of a line into two
// spaces, leaving the rest of the line untouched.
func replaceLeadingTabs(line string) string {
	n := 0
	for n < len(line) && (line[n] == ' ' || line[n] == '\t') {
		n++
	}
	if n == 0 {
		return line
	}
	return strings.ReplaceAll(line[:n], "\t", "  ") + line[n:]
}
