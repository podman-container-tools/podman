// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

package libyaml

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	aliasPathName   = regexp.MustCompile(`^[A-Za-z0-9_+\-]+$`)
	aliasPathDigits = regexp.MustCompile(`^[0-9]+$`)
	aliasPathQuoted = regexp.MustCompile(`^'[0-9]+'$`)
)

func validateAliasPath(name string, parts []string) error {
	for _, part := range parts {
		if part == "" {
			return aliasPathError(name, part, "empty path segment")
		}
		if !aliasPathName.MatchString(part) && !aliasPathQuoted.MatchString(part) {
			return aliasPathError(name, part, "invalid path segment")
		}
	}
	for _, part := range parts[1:] {
		if aliasPathDigits.MatchString(part) && len(part) > 1 && part[0] == '0' {
			return aliasPathError(name, part, "sequence index must not have leading zeros")
		}
	}
	return nil
}

func aliasPathError(name, segment, reason string) error {
	return fmt.Errorf("alias-data path %q segment %q: %s", name, segment, reason)
}

type aliasPathResult struct {
	node  *Node
	found bool
}

// A fresh lookup per segment memoizes shared merge sources without confusing
// repeated nodes in a DAG with cycles on the active lookup stack.
type aliasPathLookup struct {
	name, segment, key string
	active             map[*Node]bool
	memo               map[*Node]aliasPathResult
}

func (p *aliasPathLookup) fail(reason string) error {
	return aliasPathError(p.name, p.segment, reason)
}

func (p *aliasPathLookup) dereference(node *Node) (*Node, error) {
	seen := map[*Node]bool{}
	for node != nil && node.Kind == AliasNode {
		if seen[node] {
			return nil, p.fail("cyclic alias")
		}
		seen[node] = true
		node = node.Alias
	}
	if node == nil {
		return nil, p.fail("unresolved alias or nil data node")
	}
	return node, nil
}

func (p *aliasPathLookup) mapping(node *Node) (*Node, bool, error) {
	node, err := p.dereference(node)
	if err != nil {
		return nil, false, err
	}
	if node.Kind != MappingNode {
		return nil, false, p.fail("merge source must be a mapping")
	}
	if result, found := p.memo[node]; found {
		return result.node, result.found, nil
	}
	if p.active[node] {
		return nil, false, p.fail("cyclic merge")
	}
	p.active[node] = true
	defer delete(p.active, node)

	var explicit aliasPathResult
	var merge *Node
	// The current anchor can still be under composition. Only completed
	// key/value pairs are available to lookup at this point.
	for i := 0; i+1 < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		key, err := p.dereference(keyNode)
		if err != nil {
			return nil, false, err
		}
		// Local anchors have not reached the resolver yet. Resolve a shallow
		// copy so numeric and quoted string keys retain their normal types.
		resolved := *key
		resolved.Content = nil
		NewResolver(nil).Resolve(&resolved)
		tag := resolved.ShortTag()
		if keyNode.Kind == ScalarNode && isMerge(&resolved) {
			merge = node.Content[i+1]
		} else if resolved.Kind == ScalarNode &&
			(tag == strTag || tag == mergeTag) && resolved.Value == p.key {
			explicit = aliasPathResult{node.Content[i+1], true}
		}
	}
	if explicit.found {
		p.memo[node] = explicit
		return explicit.node, true, nil
	}
	if merge != nil {
		value, found, err := p.merge(merge)
		if err != nil {
			return nil, false, err
		}
		if found {
			p.memo[node] = aliasPathResult{value, true}
			return value, true, nil
		}
	}
	p.memo[node] = aliasPathResult{}
	return nil, false, nil
}

func (p *aliasPathLookup) merge(node *Node) (*Node, bool, error) {
	node, err := p.dereference(node)
	if err != nil {
		return nil, false, err
	}
	if node.Kind != SequenceNode {
		return p.mapping(node)
	}
	for _, child := range node.Content {
		value, found, err := p.mapping(child)
		if err != nil {
			return nil, false, err
		}
		if found {
			return value, true, nil
		}
	}
	return nil, false, nil
}

func walkAliasPath(name string, node *Node, segments []string) (*Node, error) {
	for _, segment := range segments {
		lookup := aliasPathLookup{
			name: name, segment: segment, key: segment,
			active: map[*Node]bool{}, memo: map[*Node]aliasPathResult{},
		}
		if aliasPathQuoted.MatchString(segment) {
			lookup.key = segment[1 : len(segment)-1]
		}
		var err error
		node, err = lookup.dereference(node)
		if err != nil {
			return nil, err
		}
		switch node.Kind {
		case MappingNode:
			if aliasPathDigits.MatchString(segment) {
				return nil, lookup.fail("numeric mapping keys must be single-quoted")
			}
			var found bool
			node, found, err = lookup.mapping(node)
			if err != nil {
				return nil, err
			}
			if !found {
				return nil, lookup.fail("missing mapping key")
			}
		case SequenceNode:
			for _, ch := range segment {
				if ch < '0' || ch > '9' {
					return nil, lookup.fail("sequence index must contain decimal digits")
				}
			}
			index, err := strconv.Atoi(segment)
			if err != nil || index >= len(node.Content) {
				return nil, lookup.fail("sequence index out of range")
			}
			node = node.Content[index]
		default:
			return nil, lookup.fail("cannot traverse a scalar")
		}
	}
	return node, nil
}
