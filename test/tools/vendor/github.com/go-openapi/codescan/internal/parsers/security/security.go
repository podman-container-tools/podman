// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package security is the sub-parser for the `Security:` block body that appears under
// `swagger:meta`, `swagger:route` and `swagger:operation`.
//
// The body is parsed as genuine YAML — the same path `securityDefinitions` already takes — and
// normalised into the OpenAPI 2.0 shape `[]map[string][]string`: an array of Security Requirement
// Objects.
//
// Supported forms (all idiomatic YAML):
//
//	# array of requirement objects — scopes flow- or block-style
//	- petstore_auth: [write:pets, read:pets]
//	- api_key: []
//
//	# multiple keys in one item → AND (all required); items → OR
//	- petstore_auth: [read:pets]
//	  api_key: []
//
//	# bare-name shorthand → empty scopes (a go-swagger convenience)
//	- petstore_auth
//	- api_key
//
//	security: [petstore_auth, api_key]   # flow form of the shorthand
//	security: []                          # explicit opt-out (see below)
//
// An explicit empty sequence (`[]`) is distinct from an absent block: it is the OAS 2.0 idiom for
// opting OUT of security and returns a non-nil empty list, so the spec marshals `"security": []`
// (overriding any global requirement) rather than omitting the key. go-swagger#2479.
//
// Legacy form (preserved, NOT idiomatic YAML): a bare top-level mapping with one scheme per line is
// read as a list of single-scheme requirements (OR), and a scalar scope value is comma-split:
//
//	api_key:
//	oauth: read, write   # → {oauth: [read, write]}
//
// Note this is the one shape whose meaning diverges from YAML — a YAML mapping is a single object
// (AND).
// It is kept working for back-compat; new specs should use the sequence form above.
package security

import (
	"strings"

	yamlparser "github.com/go-openapi/codescan/internal/parsers/yaml"
	"go.yaml.in/yaml/v3"
)

// Requirement is one Security Requirement Object: a map from scheme name to its scope list.
//
// Multiple keys in one Requirement are AND-combined; multiple Requirements in the returned slice
// are OR-combined.
type Requirement = map[string][]string

// Parse decodes a `Security:` block body into its requirement list.
//
// An empty body returns nil (block absent → inherit); an explicit empty YAML sequence returns a
// non-nil empty list (opt-out).
// Malformed YAML returns nil rather than failing the whole scan.
func Parse(body string) []Requirement {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	// Dedent (and expand leading tabs) so the godoc-preserved indentation becomes YAML-legal, exactly
	// as the securityDefinitions / Tags bodies do.
	normalised := strings.Join(yamlparser.RemoveIndent(strings.Split(body, "\n")), "\n")

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(normalised), &doc); err != nil {
		return nil
	}
	if len(doc.Content) == 0 {
		return nil
	}

	return fromRoot(doc.Content[0])
}

// fromRoot dispatches on the top-level YAML node shape.
func fromRoot(root *yaml.Node) []Requirement {
	switch root.Kind {
	case yaml.SequenceNode:
		// Canonical OpenAPI: a list of requirement objects (plus the bare-name shorthand).
		// An empty list is the explicit opt-out, so the non-nil empty slice is intentional.
		result := make([]Requirement, 0, len(root.Content))
		for _, item := range root.Content {
			if req := requirementFromItem(item); req != nil {
				result = append(result, req)
			}
		}
		return result
	case yaml.MappingNode:
		// Legacy shorthand: one single-scheme requirement per top-level key (OR).
		return legacyMapping(root)
	case yaml.ScalarNode:
		// A single bare scheme name with no scopes.
		if name := strings.TrimSpace(root.Value); name != "" {
			return []Requirement{{name: []string{}}}
		}
		return nil
	default:
		return nil
	}
}

// requirementFromItem builds one Requirement from a sequence item: either a mapping (one or more
// AND-combined schemes) or a bare scalar scheme name.
func requirementFromItem(item *yaml.Node) Requirement {
	switch item.Kind {
	case yaml.ScalarNode:
		name := strings.TrimSpace(item.Value)
		if name == "" {
			return nil
		}
		return Requirement{name: []string{}}
	case yaml.MappingNode:
		req := Requirement{}
		for i := 0; i+1 < len(item.Content); i += 2 {
			name := strings.TrimSpace(item.Content[i].Value)
			if name == "" {
				continue
			}
			req[name] = coerceScopes(item.Content[i+1])
		}
		if len(req) == 0 {
			return nil
		}
		return req
	default:
		return nil
	}
}

// legacyMapping turns a bare top-level mapping into one single-scheme requirement per key, in
// document order (OR semantics).
func legacyMapping(node *yaml.Node) []Requirement {
	var result []Requirement
	for i := 0; i+1 < len(node.Content); i += 2 {
		name := strings.TrimSpace(node.Content[i].Value)
		if name == "" {
			continue
		}
		result = append(result, Requirement{name: coerceScopes(node.Content[i+1])})
	}
	return result
}

// coerceScopes extracts the scope list from a requirement value: a YAML sequence (flow or block)
// yields its scalar items; a scalar value is treated as the legacy comma-separated list; null/empty
// yields no scopes.
//
// The result is always non-nil so an empty scope list marshals as `[]`.
func coerceScopes(node *yaml.Node) []string {
	scopes := []string{}
	if node == nil {
		return scopes
	}
	switch node.Kind {
	case yaml.SequenceNode:
		for _, s := range node.Content {
			if v := strings.TrimSpace(s.Value); v != "" {
				scopes = append(scopes, v)
			}
		}
	case yaml.ScalarNode:
		for part := range strings.SplitSeq(node.Value, ",") {
			if v := strings.TrimSpace(part); v != "" {
				scopes = append(scopes, v)
			}
		}
	default:
		// MappingNode / AliasNode / DocumentNode are not valid scope shapes.
	}
	return scopes
}
