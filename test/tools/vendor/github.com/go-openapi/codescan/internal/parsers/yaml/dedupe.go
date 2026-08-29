// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	"fmt"

	"go.yaml.in/yaml/v3"
)

// dropDuplicateMappingKeys walks the YAML AST rooted at node and drops earlier-occurrence duplicate
// keys from every MappingNode it encounters (last-wins semantics).
//
// It mutates Content in place; re-decoding the cleaned node through go.yaml.in/yaml/v3 then
// succeeds even under the library's strict uniqueKeys check.
//
// Two keys are considered duplicates when they share both Kind and Value — matching the library's
// own duplicate-detection rule (decode.go: mapping(), uniqueKeys branch).
// Tag differences therefore separate "1" and 1 as distinct keys.
//
// Last-wins matches the v1 gopkg.in/yaml.v2 behaviour codescan callers historically relied on.
// Diagnostic emission on duplicates is intentionally NOT done here; it is tracked as a future
// enhancement alongside the position-tracking yaml library swap (see the forthcoming-features
// roadmap).
func dropDuplicateMappingKeys(node *yaml.Node) {
	if node == nil {
		return
	}
	switch node.Kind { //nolint:exhaustive // only container kinds recurse; ScalarNode / AliasNode terminate.
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			dropDuplicateMappingKeys(child)
		}
	case yaml.MappingNode:
		node.Content = dedupePairs(node.Content)
		// Recurse into the (deduped) value side of each pair.
		for i := 1; i < len(node.Content); i += pairStride {
			dropDuplicateMappingKeys(node.Content[i])
		}
	}
}

// pairStride is the (key, value) stride of yaml.MappingNode.Content — the library packs each
// mapping as a flat slice of alternating key and value nodes.
const pairStride = 2

// dedupePairs returns content with earlier occurrences of duplicate keys removed, preserving the
// order of the surviving (last-wins) pairs.
//
// Keys are matched by (Kind, Value), matching yaml.v3's own uniqueKeys comparison.
func dedupePairs(content []*yaml.Node) []*yaml.Node {
	// At most one pair → no duplicates possible.
	if len(content) < pairStride*2 {
		return content
	}
	type key struct {
		kind  yaml.Kind
		value string
	}
	// Find the index of the latest occurrence of each key.
	latest := make(map[key]int, len(content)/pairStride)
	for i := 0; i+1 < len(content); i += pairStride {
		k := content[i]
		latest[key{k.Kind, k.Value}] = i
	}
	if len(latest)*pairStride == len(content) {
		return content // no duplicates
	}
	out := make([]*yaml.Node, 0, len(latest)*pairStride)
	for i := 0; i+1 < len(content); i += pairStride {
		k := content[i]
		if latest[key{k.Kind, k.Value}] == i {
			out = append(out, content[i], content[i+1])
		}
	}
	return out
}

// decodeYAMLBody parses body as YAML into an intermediate *yaml.Node, drops earlier-occurrence
// duplicate mapping keys (last-wins), then decodes the cleaned tree into dst. dst may be a typed
// struct, a generic map, or *any.
//
// This is the single helper the package's public surface calls instead of yaml.Unmarshal directly,
// so duplicate keys never abort the decode under strict uniqueKeys.
// Empty body is a no-op (dst untouched, nil error).
func decodeYAMLBody(body []byte, dst any) error {
	if len(body) == 0 {
		return nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal(body, &root); err != nil {
		return fmt.Errorf("yaml: %w", err)
	}
	dropDuplicateMappingKeys(&root)
	if err := root.Decode(dst); err != nil {
		return fmt.Errorf("yaml: %w", err)
	}
	return nil
}
