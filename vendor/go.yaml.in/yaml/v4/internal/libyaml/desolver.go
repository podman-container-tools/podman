// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Desolver stage: Removes inferable tags from YAML nodes.
// This is the inverse of the Resolver - it walks a tagged node tree and
// removes tags that can be inferred during parsing, producing cleaner YAML
// output without unnecessary type annotations.

package libyaml

// Desolver handles tag removal for YAML nodes during serialization.
// It removes tags that would be automatically resolved to the same type
// during parsing, making the output cleaner and more readable.
type Desolver struct {
	opts *Options
}

// NewDesolver creates a new Desolver with the given options.
func NewDesolver(opts *Options) *Desolver {
	return &Desolver{opts: opts}
}

// Desolve walks the node tree and removes tags that can be inferred.
// This is the inverse of Resolver - it takes a fully-tagged node tree
// (from Representer) and removes unnecessary tags to produce clean output.
//
// For scalar nodes: if the value would resolve to the same tag when parsed,
// the tag is removed. For strings that would resolve differently, the tag is
// removed and quoting style is set to preserve the string type.
//
// For collection nodes (maps/sequences): default tags (!!map, !!seq) are
// removed since they're implied by the structure.
func (d *Desolver) Desolve(n *Node) {
	if n == nil {
		return
	}

	switch n.Kind {
	case ScalarNode:
		d.desolveScalar(n)
	case DocumentNode, SequenceNode, MappingNode:
		d.desolveCollection(n)
		// Recursively desolve children
		for _, child := range n.Content {
			d.Desolve(child)
		}
	case AliasNode:
		// Alias nodes don't have tags to remove
	}
}

// desolveScalar removes tags from scalar nodes when they can be inferred.
func (d *Desolver) desolveScalar(n *Node) {
	// If explicitly tagged by user (TaggedStyle), keep it
	if n.Style&TaggedStyle != 0 {
		return
	}

	// Empty tag means it's already untagged - nothing to do
	if n.Tag == "" {
		return
	}

	stag := shortTag(n.Tag)

	// Check if this is a standard scalar tag that we can potentially remove
	isStandardTag := false
	switch stag {
	case nullTag, boolTag, strTag, intTag, floatTag, timestampTag:
		isStandardTag = true
	case binaryTag:
		// Binary scalars are not implicitly resolvable - never remove.
		return
	case mergeTag:
		// Elide the implicit !!merge tag when the value is the canonical
		// merge key marker. The TaggedStyle early-return above already
		// preserves !!merge when it was explicit in the source.
		if n.Value == "<<" {
			n.Tag = ""
		}
		return
	default:
		// Custom tag - preserve it
		return
	}

	// Only process standard tags from here
	if !isStandardTag {
		return
	}

	// What tag would this value resolve to?
	var formatter func(*LoadError) string
	if d.opts != nil {
		formatter = d.opts.FormatLoadError
	}
	rtag, _ := resolve("", n.Value, formatter)

	// If resolved tag matches current tag, we can elide the tag
	if rtag == stag {
		// Tag can be inferred - remove it
		n.Tag = ""
	} else if stag == strTag {
		// This is a string type, but would resolve to something else.
		// Remove the tag and force quoting to preserve string type.
		n.Tag = ""
		// If not already quoted, set quote style based on content
		if n.Style&(SingleQuotedStyle|DoubleQuotedStyle|LiteralStyle|FoldedStyle) == 0 {
			// Determine quote style based on options or default to single quotes
			if d.opts != nil {
				// Convert ScalarStyle to Style
				switch d.opts.QuotePreference.ScalarStyle() {
				case DOUBLE_QUOTED_SCALAR_STYLE:
					n.Style |= DoubleQuotedStyle
				default:
					n.Style |= SingleQuotedStyle
				}
			} else {
				n.Style |= SingleQuotedStyle
			}
		}
	} else if stag == floatTag || stag == intTag {
		// For numeric type mismatches (like float64(1) → "1" with !!float tag):
		// Elide the tag and let YAML resolve naturally.
		// Without the tag, "1" resolves as !!int, which may change the type,
		// but that's acceptable for cleaner output (and matches old behavior).
		n.Tag = ""
	}
	// For other standard tags with mismatches, keep the tag to preserve type
}

// desolveCollection removes default tags from collection nodes.
func (d *Desolver) desolveCollection(n *Node) {
	// If explicitly tagged by user, keep it
	if n.Style&TaggedStyle != 0 {
		return
	}

	stag := shortTag(n.Tag)
	switch n.Kind {
	case MappingNode:
		// !!map is the default for mappings - remove it
		if stag == mapTag {
			n.Tag = ""
		}
	case SequenceNode:
		// !!seq is the default for sequences - remove it
		if stag == seqTag {
			n.Tag = ""
		}
	case DocumentNode:
		// Documents don't have tags in YAML output
		n.Tag = ""
	}
	// For other tags, keep them - they're explicit type information
}
