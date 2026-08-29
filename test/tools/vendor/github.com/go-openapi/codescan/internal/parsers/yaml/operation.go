// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	"fmt"
	"strings"

	"github.com/go-openapi/swag/yamlutils"
)

// UnmarshalBody runs a raw godoc-comment YAML body through the standard godoc → YAML → JSON
// pipeline expected by every Swagger target that consumes JSON-shape input:
//
//  1. RemoveIndent — strip the common indent godoc adds to every
//     line and turn leading tabs into two-space sequences (YAML
//     refuses tab indentation).
//  2. yaml.Unmarshal into a generic map[any]any.
//  3. yamlutils.YAMLToJSON — coerce the map[any]any soup into
//     JSON-shaped values with concrete leaf types.
//  4. Hand the resulting JSON bytes to the caller's callback,
//     typically a *spec.<Target>.UnmarshalJSON or a json.Unmarshal
//     into a caller-provided struct.
//
// Empty body returns nil — the caller's target is left untouched.
//
// Used by the operations bridge (swagger:operation YAML body), the meta bridge
// (securityDefinitions, externalDocs), and any future mapping target that needs the same shape.
// Sequence-shaped bodies (e.g. meta `Tags:`) use [UnmarshalListBody].
func UnmarshalBody(body string, unmarshal func([]byte) error) error {
	if body == "" {
		return nil
	}

	yamlValue := make(map[any]any)
	if err := decodeYAMLBody([]byte(normaliseBody(body)), &yamlValue); err != nil {
		return fmt.Errorf("yaml body: %w", err)
	}

	jsonValue, err := yamlutils.YAMLToJSON(yamlValue)
	if err != nil {
		return fmt.Errorf("yaml→json: %w", err)
	}

	return unmarshal(jsonValue)
}

// UnmarshalListBody is the sequence-shaped counterpart to
// [UnmarshalBody]: it runs the same godoc → YAML → JSON pipeline but
// decodes the body as a YAML list ([]any) rather than a mapping. Used
// by the meta bridge for the top-level `Tags:` block (a list of tag
// objects). Empty body returns nil.
func UnmarshalListBody(body string, unmarshal func([]byte) error) error {
	if body == "" {
		return nil
	}

	var seq []any
	if err := decodeYAMLBody([]byte(normaliseBody(body)), &seq); err != nil {
		return fmt.Errorf("yaml body: %w", err)
	}

	jsonValue, err := yamlutils.YAMLToJSON(seq)
	if err != nil {
		return fmt.Errorf("yaml→json: %w", err)
	}

	return unmarshal(jsonValue)
}

// normaliseBody applies the godoc-comment dedent shared by the body unmarshal helpers: strip the
// common leading indent and expand residual leading tabs to spaces (YAML refuses tab indentation).
func normaliseBody(body string) string {
	lines := strings.Split(body, "\n")
	lines = RemoveIndent(lines)
	return strings.Join(lines, "\n")
}
