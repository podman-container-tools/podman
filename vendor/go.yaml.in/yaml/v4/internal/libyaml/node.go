// Copyright 2011-2019 Canonical Ltd
// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Node types and constants for YAML tree representation.
// Defines Kind, Style, and Node structure for intermediate YAML representation.

package libyaml

import (
	"errors"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tag constants for YAML types
const (
	nullTag      = "!!null"
	boolTag      = "!!bool"
	strTag       = "!!str"
	intTag       = "!!int"
	floatTag     = "!!float"
	timestampTag = "!!timestamp"
	seqTag       = "!!seq"
	mapTag       = "!!map"
	binaryTag    = "!!binary"
	mergeTag     = "!!merge"
)

// longTagPrefix is the standard YAML tag prefix for core types.
const longTagPrefix = "tag:yaml.org,2002:"

// longTags maps short tags to their long form representations.
// shortTags maps long tags to their short form representations.
var (
	longTags  = make(map[string]string)
	shortTags = make(map[string]string)
)

// init initializes the tag conversion maps.
func init() {
	for _, stag := range []string{nullTag, boolTag, strTag, intTag, floatTag, timestampTag, seqTag, mapTag, binaryTag, mergeTag} {
		ltag := longTag(stag)
		longTags[stag] = ltag
		shortTags[ltag] = stag
	}
}

// shortTag converts a long-form tag to its short form (e.g., "tag:yaml.org,2002:str" to "!!str").
func shortTag(tag string) string {
	if strings.HasPrefix(tag, longTagPrefix) {
		if stag, ok := shortTags[tag]; ok {
			return stag
		}
		return "!!" + tag[len(longTagPrefix):]
	}
	return tag
}

// longTag converts a short-form tag to its long form (e.g., "!!str" to "tag:yaml.org,2002:str").
func longTag(tag string) string {
	if strings.HasPrefix(tag, "!!") {
		if ltag, ok := longTags[tag]; ok {
			return ltag
		}
		return longTagPrefix + tag[2:]
	}
	return tag
}

// Kind represents the type of YAML [Node.Kind].
type Kind uint32

// [Kind] constants define the different types of YAML [Node].
const (
	// DocumentNode represents the root of a YAML document.
	DocumentNode Kind = 1 << iota
	// SequenceNode represents a YAML sequence (list).
	SequenceNode
	// MappingNode represents a YAML mapping (dictionary).
	MappingNode
	// ScalarNode represents a YAML scalar value.
	ScalarNode
	// AliasNode represents a reference to an anchored node.
	AliasNode
	// StreamNode represents a container for multiple YAML documents.
	StreamNode
)

// Style represents the formatting style of a YAML Node (see [Node.Style])
type Style uint32

// [Style] constants define different formatting styles for YAML [Node].
const (
	// TaggedStyle explicitly shows the tag on the node.
	TaggedStyle Style = 1 << iota
	// DoubleQuotedStyle uses double quotes for scalar values.
	DoubleQuotedStyle
	// SingleQuotedStyle uses single quotes for scalar values.
	SingleQuotedStyle
	// LiteralStyle uses literal block scalar style (|).
	LiteralStyle
	// FoldedStyle uses folded block scalar style (>).
	FoldedStyle
	// FlowStyle uses flow style (inline) formatting.
	FlowStyle
)

// StreamVersionDirective represents a YAML %YAML version directive for
// [Stream.Version].
type StreamVersionDirective struct {
	Major int
	Minor int
}

// StreamTagDirective represents a YAML %TAG directive for [Stream.TagDirectives].
type StreamTagDirective struct {
	Handle string
	Prefix string
}

// Stream holds stream-level metadata for [StreamNode].
// This includes encoding, version directive, and tag directives.
type Stream struct {
	Encoding      Encoding
	Version       *StreamVersionDirective
	TagDirectives []StreamTagDirective
}

// Node represents an element in the YAML document hierarchy. While documents
// are typically encoded and decoded into higher level types, such as structs
// and maps, Node is an intermediate representation that allows detailed
// control over the content being decoded or encoded.
//
// It's worth noting that although Node offers access into details such as
// line numbers, columns, and comments, the content when re-encoded will not
// have its original textual representation preserved. An effort is made to
// render the data pleasantly, and to preserve comments near the data they
// describe, though.
//
// Values that make use of the Node type interact with the yaml package in the
// same way any other type would do, by encoding and decoding yaml data
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
type Node struct {
	// Kind defines whether the node is a document, a mapping, a sequence,
	// a scalar value, or an alias to another node. The specific data type of
	// scalar nodes may be obtained via the ShortTag and LongTag methods.
	Kind Kind

	// Style allows customizing the appearance of the node in the tree.
	Style Style

	// Tag holds the YAML tag defining the data type for the value.
	// When decoding, this field will always be set to the resolved tag,
	// even when it wasn't explicitly provided in the YAML content.
	// When encoding, if this field is unset the value type will be
	// implied from the node properties, and if it is set, it will only
	// be serialized into the representation if TaggedStyle is used or
	// the implicit tag diverges from the provided one.
	Tag string

	// Value holds the unescaped and unquoted representation of the value.
	Value string

	// Anchor holds the anchor name for this node, which allows aliases to point to it.
	Anchor string

	// Alias holds the node that this alias points to. Only valid when Kind is AliasNode.
	Alias *Node

	// Content holds contained nodes for documents, mappings, and sequences.
	Content []*Node

	// HeadComment holds any comments in the lines preceding the node and
	// not separated by an empty line.
	HeadComment string

	// LineComment holds any comments at the end of the line where the node is in.
	LineComment string

	// FootComment holds any comments following the node and before empty lines.
	FootComment string

	// Line and Column hold the node position in the decoded YAML text.
	// These fields are not respected when encoding the node.
	Line   int
	Column int

	// Stream holds stream metadata (non-nil only when Kind == StreamNode).
	Stream *Stream
}

// IsZero returns whether the node has all of its fields unset.
func (n *Node) IsZero() bool {
	return n.Kind == 0 && n.Style == 0 && n.Tag == "" && n.Value == "" && n.Anchor == "" && n.Alias == nil && n.Content == nil &&
		n.HeadComment == "" && n.LineComment == "" && n.FootComment == "" && n.Line == 0 && n.Column == 0 &&
		n.Stream == nil
}

// LongTag returns the long form of the tag that indicates the data type for
// the node. If the Tag field isn't explicitly defined, one will be computed
// based on the node properties.
func (n *Node) LongTag() string {
	return longTag(n.ShortTag())
}

// ShortTag returns the short form of the YAML tag that indicates data type for
// the node. If the Tag field isn't explicitly defined, one will be computed
// based on the node properties.
func (n *Node) ShortTag() string {
	if n.indicatedString() {
		return strTag
	}
	if n.Tag == "" || n.Tag == "!" {
		switch n.Kind {
		case MappingNode:
			return mapTag
		case SequenceNode:
			return seqTag
		case AliasNode:
			if n.Alias != nil {
				return n.Alias.ShortTag()
			}
		case ScalarNode:
			return strTag
		case 0:
			// Special case to make the zero value convenient.
			if n.IsZero() {
				return nullTag
			}
		}
		return ""
	}
	return shortTag(n.Tag)
}

// indicatedString returns true if the node's style explicitly indicates a string type.
func (n *Node) indicatedString() bool {
	return n.Kind == ScalarNode &&
		(shortTag(n.Tag) == strTag ||
			(n.Tag == "" || n.Tag == "!") && n.Style&(SingleQuotedStyle|DoubleQuotedStyle|LiteralStyle|FoldedStyle) != 0)
}

// shouldUseLiteralStyle determines if a string should use literal style.
// It returns true if the string contains newlines AND meets additional criteria:
// - is at least 2 characters long
// - contains at least one non-whitespace character
func shouldUseLiteralStyle(s string) bool {
	if !strings.Contains(s, "\n") || len(s) < 2 {
		return false
	}
	// Must contain at least one non-whitespace character
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

// SetString is a convenience function that sets the node to a string value
// and defines its style in a pleasant way depending on its content.
func (n *Node) SetString(s string) {
	n.Kind = ScalarNode
	if utf8.ValidString(s) {
		n.Value = s
		n.Tag = strTag
	} else {
		n.Value = encodeBase64(s)
		n.Tag = binaryTag
	}
	if shouldUseLiteralStyle(n.Value) {
		n.Style = LiteralStyle
	}
}

// Decode decodes the node and stores its data into the value pointed to by v.
//
// See the documentation for [yaml.Unmarshal] for details about the
// conversion of YAML into a Go value.
//
// [yaml.Unmarshal]: https://pkg.go.dev/go.yaml.in/yaml/v4#Unmarshal
func (n *Node) Decode(v any) (err error) {
	d := NewConstructor(DefaultOptions)
	defer handleErr(&err)
	out := reflect.ValueOf(v)
	if out.Kind() == reflect.Pointer && !out.IsNil() {
		out = out.Elem()
	}
	d.Construct(n, out)
	if len(d.TypeErrors) > 0 {
		return &LoadErrors{Errors: d.TypeErrors}
	}
	return nil
}

// Load decodes the node and stores its data into the value pointed to by v,
// applying the given options.
// Explicit Alias-Data policies are rejected because composition has already
// occurred and the policy cannot be applied to this node graph.
//
// Each call starts with fresh options, independently of an outer Loader.
// Pass WithKnownFields() explicitly to check fields inside a custom
// UnmarshalYAML implementation.
// Parser settings do not apply to this already composed graph.
//
// Maps and pointers (to a struct, string, int, etc) are accepted as v
// values. If an internal pointer within a struct is not initialized,
// the yaml package will initialize it if necessary. The v parameter
// must not be nil.
//
// See the documentation of the package-level Load function for details
// about YAML to Go conversion and tag options.
func (n *Node) Load(v any, opts ...Option) (err error) {
	defer handleErr(&err)
	o, err := ApplyOptions(opts...)
	if err != nil {
		return err
	}
	if o.AliasData != nil {
		return errors.New("yaml: alias-data is not supported with precomposed node input")
	}
	d := NewConstructor(o)
	out := reflect.ValueOf(v)
	if out.Kind() == reflect.Pointer && !out.IsNil() {
		out = out.Elem()
	}
	d.Construct(n, out)
	if len(d.TypeErrors) > 0 {
		return &LoadErrors{Errors: d.TypeErrors}
	}
	return nil
}

// Encode encodes value v and stores its representation in n.
//
// See the documentation for [yaml.Marshal] for details about the
// conversion of Go values into YAML.
//
// [yaml.Marshal]: https://pkg.go.dev/go.yaml.in/yaml/v4#Marshal
func (n *Node) Encode(v any) (err error) {
	defer handleErr(&err)
	// Use the 3-stage dump pipeline with round-trip to preserve styles
	r := NewRepresenter(DefaultOptions)
	node := r.Represent("", reflect.ValueOf(v))
	d := NewDesolver(DefaultOptions)
	d.Desolve(node)
	s := NewSerializer(nil, DefaultOptions)
	var out []byte
	s.Emitter.SetOutputString(&out)
	s.Serialize(node)
	s.Finish()
	// Parse back to get styles
	p := NewComposer(out, nil)
	p.Textless = true
	defer p.Destroy()
	doc := p.Compose()
	NewResolver(nil).Resolve(doc)
	*n = *doc.Content[0]
	return nil
}

// Dump encodes value v and stores its representation in n,
// applying the given options.
//
// This method is useful when you need to apply specific encoding options
// while building Node trees programmatically.
// Node stores styles, but not output indentation or document-marker settings.
// Apply those options again when serializing the node to YAML.
//
// See the documentation for [yaml.Marshal] for details about the
// conversion of Go values into YAML.
//
// [yaml.Marshal]: https://pkg.go.dev/go.yaml.in/yaml/v4#Marshal
func (n *Node) Dump(v any, opts ...Option) (err error) {
	defer handleErr(&err)
	o, err := ApplyOptions(opts...)
	if err != nil {
		return err
	}
	// Use the 3-stage dump pipeline with round-trip to preserve styles
	r := NewRepresenter(o)
	node := r.Represent("", reflect.ValueOf(v))
	d := NewDesolver(o)
	d.Desolve(node)
	s := NewSerializer(nil, o)
	var out []byte
	s.Emitter.SetOutputString(&out)
	s.Serialize(node)
	s.Finish()
	// Parse back to get styles
	p := NewComposer(out, &Options{
		IndentConfig: o.IndentConfig,
		DepthCheck:   o.DepthCheck,
	})
	p.Textless = true
	defer p.Destroy()
	doc := p.Compose()
	NewResolver(nil).Resolve(doc)
	*n = *doc.Content[0]
	return nil
}

// Marshaler interface may be implemented by types to customize their
// behavior when being marshaled into a YAML document.
type Marshaler interface {
	MarshalYAML() (any, error)
}

// Unmarshaler is the interface implemented by types that can unmarshal
// a YAML description of themselves.
type Unmarshaler interface {
	UnmarshalYAML(node *Node) error
}

// IsZeroer is used to check whether an object is zero to determine whether
// it should be omitted when marshaling with the ,omitempty flag. One notable
// implementation is [time.Time].
type IsZeroer interface {
	IsZero() bool
}

// FromYAMLNode is a reserved internal interface.
// The current loading pipeline does not invoke it.
// Use Unmarshaler to customize loading.
type FromYAMLNode interface {
	FromYAMLNode(*Node) error
}

// ToYAMLNode is a reserved internal interface.
// The current dumping pipeline does not invoke it.
// Use Marshaler to customize dumping.
type ToYAMLNode interface {
	ToYAMLNode() (*Node, error)
}

// isZero reports whether v represents the zero value for its type.
// If v implements the IsZeroer interface, IsZero() is called.
// Otherwise, zero is determined by checking type-specific conditions.
// This is used to determine omitempty behavior when marshaling.
func isZero(v reflect.Value) bool {
	kind := v.Kind()
	if z, ok := v.Interface().(IsZeroer); ok {
		if (kind == reflect.Pointer || kind == reflect.Interface) && v.IsNil() {
			return true
		}
		return z.IsZero()
	}
	switch kind {
	case reflect.String:
		return len(v.String()) == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	case reflect.Slice:
		return v.Len() == 0
	case reflect.Map:
		return v.Len() == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Struct:
		vt := v.Type()
		for i := v.NumField() - 1; i >= 0; i-- {
			if vt.Field(i).PkgPath != "" {
				continue // Private field
			}
			if !isZero(v.Field(i)) {
				return false
			}
		}
		return true
	}
	return false
}
