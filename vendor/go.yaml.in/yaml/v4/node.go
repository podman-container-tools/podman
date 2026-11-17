package yaml

import "go.yaml.in/yaml/v4/internal/libyaml"

// -----------------------------------------------------------------------------
// Node-related type aliases and constants
// -----------------------------------------------------------------------------
type (
	// Node represents an element in the YAML document hierarchy.
	// While documents are typically encoded and decoded into higher level
	// types, such as structs and maps, Node is an intermediate representation
	// that allows detailed control over the content being decoded or encoded.
	//
	// It's worth noting that although Node offers access into details such as
	// line numbers, columns, and comments, the content when re-encoded will
	// not have its original textual representation preserved.
	// An effort is made to render the data pleasantly, and to preserve
	// comments near the data they describe, though.
	//
	// Values that make use of the Node type interact with the yaml package in
	// the same way any other type would do, by encoding and decoding yaml data
	// directly or indirectly into them.
	//
	// For example:
	//
	//	var person struct {
	//	        Name    string
	//	        Address yaml.Node
	//	}
	//	err := yaml.Unmarshal(data, &person)
	//
	// Or by itself:
	//
	//	var person Node
	//	err := yaml.Unmarshal(data, &person)
	Node = libyaml.Node

	// Kind represents the type of YAML node.
	Kind = libyaml.Kind

	// Style represents the formatting style of a YAML node.
	Style = libyaml.Style

	// Marshaler interface may be implemented by types to customize their
	// behavior when being marshaled into a YAML document.
	Marshaler = libyaml.Marshaler

	// Unmarshaler is the interface implemented by types that can unmarshal
	// a YAML description of themselves.
	Unmarshaler = libyaml.Unmarshaler

	// IsZeroer is used to check whether an object is zero to determine whether
	// it should be omitted when marshaling with the ,omitempty flag.
	// One notable implementation is [time.Time].
	IsZeroer = libyaml.IsZeroer
)

// [Kind] constants define the different types of YAML [Node].
const (
	// DocumentNode represents the root of a YAML document.
	DocumentNode = libyaml.DocumentNode

	// SequenceNode represents a YAML sequence (list).
	SequenceNode = libyaml.SequenceNode

	// MappingNode represents a YAML mapping (dictionary).
	MappingNode = libyaml.MappingNode

	// ScalarNode represents a YAML scalar value.
	ScalarNode = libyaml.ScalarNode

	// AliasNode represents a reference to an anchored node.
	AliasNode = libyaml.AliasNode

	// StreamNode represents a container for multiple YAML documents.
	StreamNode = libyaml.StreamNode
)

// [Style] constants define different formatting styles for YAML nodes.
const (
	// TaggedStyle explicitly shows the tag on the node.
	TaggedStyle = libyaml.TaggedStyle

	// DoubleQuotedStyle uses double quotes for scalar values.
	DoubleQuotedStyle = libyaml.DoubleQuotedStyle

	// SingleQuotedStyle uses single quotes for scalar values.
	SingleQuotedStyle = libyaml.SingleQuotedStyle

	// LiteralStyle uses literal block scalar style (|).
	LiteralStyle = libyaml.LiteralStyle

	// FoldedStyle uses folded block scalar style (>).
	FoldedStyle = libyaml.FoldedStyle

	// FlowStyle uses flow style (inline) formatting.
	FlowStyle = libyaml.FlowStyle
)
