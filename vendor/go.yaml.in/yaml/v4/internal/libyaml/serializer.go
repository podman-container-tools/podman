// Copyright 2011-2019 Canonical Ltd
// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Serializer stage: Converts representation tree (Nodes) to event stream.
// Walks the node tree and produces events for the emitter.

package libyaml

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Serializer handles serialization of YAML nodes to event stream.
type Serializer struct {
	Emitter               Emitter
	out                   []byte
	lineWidth             int
	explicitStart         bool
	explicitEnd           bool
	flowSimpleCollections bool
	quotePreference       QuoteStyle
	formatDumpError       func(*DumpError) string
	doneInit              bool
}

// NewSerializer creates a new Serializer with the given options.
func NewSerializer(w io.Writer, opts *Options) *Serializer {
	emitter := NewEmitter()
	tabs := opts.IndentConfig != nil &&
		opts.IndentConfig.DumpStyle == IndentStyleTabs
	emitter.CompactSequenceIndent = opts.CompactSeqIndent
	emitter.quotePreference = opts.QuotePreference
	emitter.SetWidth(opts.LineWidth)
	emitter.SetUnicode(opts.Unicode)
	emitter.SetCanonical(opts.Canonical)
	emitter.SetLineBreak(opts.LineBreak)
	emitter.tabIndent = tabs

	// Set indentation (defaults to 2 if not specified)
	indent := opts.Indent
	if tabs || indent == 0 {
		indent = 2
	}
	emitter.BestIndent = indent

	if w != nil {
		emitter.SetOutputWriter(w)
	}

	return &Serializer{
		Emitter:               emitter,
		lineWidth:             opts.LineWidth,
		explicitStart:         opts.ExplicitStart,
		explicitEnd:           opts.ExplicitEnd,
		flowSimpleCollections: opts.FlowSimpleCollections,
		quotePreference:       opts.QuotePreference,
		formatDumpError:       opts.FormatDumpError,
	}
}

// Serialize walks a Node tree and emits events to produce YAML output.
// This is the primary method for the Serializer stage.
func (s *Serializer) Serialize(node *Node) {
	s.init()
	s.node(node, "")
}

// Sentinel values for event creation.
// These provide clarity at call sites, similar to http.NoBody.
var (
	noVersionDirective *VersionDirective = nil
	noTagDirective     []TagDirective    = nil
)

// init initializes the serializer by emitting a STREAM_START event.
func (s *Serializer) init() {
	if s.doneInit {
		return
	}
	s.emit(NewStreamStartEvent(UTF8_ENCODING))
	s.doneInit = true
}

// Finish completes serialization by emitting a STREAM_END event.
func (s *Serializer) Finish() {
	s.Emitter.OpenEnded = false
	s.emit(NewStreamEndEvent())
}

// node serializes a Node tree into YAML events.
// This is the core of the serializer stage - it walks the tree and produces
// events.
func (s *Serializer) node(node *Node, tail string) {
	// Zero nodes behave as nil.
	if node.Kind == 0 && node.IsZero() {
		s.emitScalar("null", "", "", PLAIN_SCALAR_STYLE, nil, nil, nil, nil)
		return
	}

	// Tags have been processed by Desolver:
	// - Empty tag = can be inferred or style handles it
	// - Non-empty tag = emit explicitly
	// Style has also been set by Desolver for quoting needs
	tag := node.Tag
	var forceQuoting bool
	if tag == "" && node.Kind == ScalarNode {
		// Empty tag with quoting style means the string type needs to
		// be preserved
		if node.Style&(SingleQuotedStyle|DoubleQuotedStyle|LiteralStyle|FoldedStyle) != 0 {
			forceQuoting = true
		}
	}

	switch node.Kind {
	case DocumentNode:
		event := NewDocumentStartEvent(noVersionDirective, noTagDirective, !s.explicitStart)
		event.HeadComment = []byte(node.HeadComment)
		s.emit(event)
		for _, node := range node.Content {
			s.node(node, "")
		}
		event = NewDocumentEndEvent(!s.explicitEnd)
		event.FootComment = []byte(node.FootComment)
		s.emit(event)

	case SequenceNode:
		style := BLOCK_SEQUENCE_STYLE
		// Use flow style if explicitly requested or if it's a simple
		// collection (scalar-only contents that fit within line width,
		// enabled via WithFlowSimpleCollections)
		if node.Style&FlowStyle != 0 || s.isSimpleCollection(node) {
			style = FLOW_SEQUENCE_STYLE
		}
		event := NewSequenceStartEvent([]byte(node.Anchor), []byte(longTag(tag)), tag == "", style)
		event.HeadComment = []byte(node.HeadComment)
		s.emit(event)
		for _, node := range node.Content {
			s.node(node, "")
		}
		event = NewSequenceEndEvent()
		event.LineComment = []byte(node.LineComment)
		event.FootComment = []byte(node.FootComment)
		s.emit(event)

	case MappingNode:
		style := BLOCK_MAPPING_STYLE
		// Use flow style if explicitly requested or if it's a simple
		// collection (scalar-only contents that fit within line width,
		// enabled via WithFlowSimpleCollections)
		if node.Style&FlowStyle != 0 || s.isSimpleCollection(node) {
			style = FLOW_MAPPING_STYLE
		}
		event := NewMappingStartEvent([]byte(node.Anchor), []byte(longTag(tag)), tag == "", style)
		event.TailComment = []byte(tail)
		event.HeadComment = []byte(node.HeadComment)
		s.emit(event)

		// The tail logic below moves the foot comment of prior keys to
		// the following key, since the value for each key may be a
		// nested structure and the foot needs to be processed only the
		// entirety of the value is streamed. The last tail is
		// processed with the mapping end event.
		var tail string
		for i := 0; i+1 < len(node.Content); i += 2 {
			k := node.Content[i]
			foot := k.FootComment
			if foot != "" {
				kopy := *k
				kopy.FootComment = ""
				k = &kopy
			}
			s.node(k, tail)
			tail = foot

			v := node.Content[i+1]
			s.node(v, "")
		}

		event = NewMappingEndEvent()
		event.TailComment = []byte(tail)
		event.LineComment = []byte(node.LineComment)
		event.FootComment = []byte(node.FootComment)
		s.emit(event)

	case AliasNode:
		event := NewAliasEvent([]byte(node.Value))
		event.HeadComment = []byte(node.HeadComment)
		event.LineComment = []byte(node.LineComment)
		event.FootComment = []byte(node.FootComment)
		s.emit(event)

	case ScalarNode:
		value := node.Value
		if !utf8.ValidString(value) {
			stag := shortTag(tag)
			if stag == binaryTag {
				failDumpf(
					SerializerStage,
					s.formatDumpError,
					"explicitly tagged !!binary data must be base64-encoded",
				)
			}
			if stag != "" {
				failDumpf(
					SerializerStage,
					s.formatDumpError,
					"cannot marshal invalid UTF-8 data as %s",
					stag,
				)
			}
			// It can't be represented directly as YAML so use a binary tag
			// and represent it as base64.
			tag = binaryTag
			value = encodeBase64(value)
		}

		style := PLAIN_SCALAR_STYLE
		switch {
		case node.Style&DoubleQuotedStyle != 0:
			style = DOUBLE_QUOTED_SCALAR_STYLE
		case node.Style&SingleQuotedStyle != 0:
			style = SINGLE_QUOTED_SCALAR_STYLE
		case node.Style&LiteralStyle != 0:
			style = LITERAL_SCALAR_STYLE
		case node.Style&FoldedStyle != 0:
			style = FOLDED_SCALAR_STYLE
		case strings.Contains(value, "\n"):
			style = LITERAL_SCALAR_STYLE
		case forceQuoting:
			style = s.quotePreference.ScalarStyle()
		}

		s.emitScalar(value, node.Anchor, tag, style, []byte(node.HeadComment), []byte(node.LineComment), []byte(node.FootComment), []byte(tail))
	default:
		failDumpf(
			SerializerStage,
			s.formatDumpError,
			"cannot represent node with unknown kind %d",
			node.Kind,
		)
	}
}

// emit sends an event to the underlying emitter.
func (s *Serializer) emit(event Event) {
	s.must(s.Emitter.Emit(&event))
}

// must panics if the given error is non-nil, routing to the appropriate stage.
func (s *Serializer) must(err error) {
	if err == nil {
		return
	}
	var ee EmitterError
	if errors.As(err, &ee) {
		failDumpf(EmitterStage, s.formatDumpError, "%s", ee.Message)
	}
	var we WriterError
	if errors.As(err, &we) {
		// Unwrap to get the original I/O error, stripping the
		// "write error: " prefix that WriterError adds internally.
		cause := we.Err
		if unwrapped := errors.Unwrap(we.Err); unwrapped != nil {
			cause = unwrapped
		}
		failDump(WriterStage, cause, s.formatDumpError)
	}
	msg := err.Error()
	if msg == "" {
		msg = fmt.Sprintf("unknown problem generating YAML content with %T", err)
	}
	failDumpf(SerializerStage, s.formatDumpError, "%s", msg)
}

// emitScalar emits a scalar event with the given value, anchor, tag, style,
// and associated comments.
func (s *Serializer) emitScalar(
	value, anchor, tag string, style ScalarStyle, head, line, foot, tail []byte,
) {
	implicit := tag == ""
	if !implicit {
		tag = longTag(tag)
	}
	event := NewScalarEvent([]byte(anchor), []byte(tag), []byte(value), implicit, implicit, style)
	event.HeadComment = head
	event.LineComment = line
	event.FootComment = foot
	event.TailComment = tail
	s.emit(event)
}

// isSimpleCollection checks if a node contains only scalar values and would
// fit within the line width when rendered in flow style.
func (s *Serializer) isSimpleCollection(node *Node) bool {
	if !s.flowSimpleCollections {
		return false
	}
	if node.Kind != SequenceNode && node.Kind != MappingNode {
		return false
	}
	// Check all children are scalars
	for _, child := range node.Content {
		if child.Kind != ScalarNode {
			return false
		}
	}
	// Estimate flow style length
	estimatedLen := s.estimateFlowLength(node)
	width := s.lineWidth
	if width <= 0 {
		width = 80 // Default width if not set
	}
	return estimatedLen > 0 && estimatedLen <= width
}

// estimateFlowLength estimates the character length of a node in flow style.
func (s *Serializer) estimateFlowLength(node *Node) int {
	if node.Kind == SequenceNode {
		// [item1, item2, ...] = 2 + sum(len(items)) + 2*(len-1)
		length := 2 // []
		for i, child := range node.Content {
			if i > 0 {
				length += 2 // ", "
			}
			length += len(child.Value)
		}
		return length
	}
	if node.Kind == MappingNode {
		// {key1: val1, key2: val2} = 2 + sum(key: val) + 2*(pairs-1)
		length := 2 // {}
		for i := 0; i < len(node.Content); i += 2 {
			if i > 0 {
				length += 2 // ", "
			}
			length += len(node.Content[i].Value) + 2 + len(node.Content[i+1].Value) // "key: val"
		}
		return length
	}
	return 0
}

// NewStreamStartEvent creates a new STREAM-START event.
func NewStreamStartEvent(encoding Encoding) Event {
	return Event{
		Type:     STREAM_START_EVENT,
		encoding: encoding,
	}
}

// NewStreamEndEvent creates a new STREAM-END event.
func NewStreamEndEvent() Event {
	return Event{
		Type: STREAM_END_EVENT,
	}
}

// NewDocumentStartEvent creates a new DOCUMENT-START event.
func NewDocumentStartEvent(version_directive *VersionDirective, tag_directives []TagDirective, implicit bool) Event {
	return Event{
		Type:             DOCUMENT_START_EVENT,
		versionDirective: version_directive,
		tagDirectives:    tag_directives,
		Implicit:         implicit,
	}
}

// NewDocumentEndEvent creates a new DOCUMENT-END event.
func NewDocumentEndEvent(implicit bool) Event {
	return Event{
		Type:     DOCUMENT_END_EVENT,
		Implicit: implicit,
	}
}

// NewAliasEvent creates a new ALIAS event.
func NewAliasEvent(anchor []byte) Event {
	return Event{
		Type:   ALIAS_EVENT,
		Anchor: anchor,
	}
}

// NewScalarEvent creates a new SCALAR event.
func NewScalarEvent(anchor, tag, value []byte, plain_implicit, quoted_implicit bool, style ScalarStyle) Event {
	return Event{
		Type:            SCALAR_EVENT,
		Anchor:          anchor,
		Tag:             tag,
		Value:           value,
		Implicit:        plain_implicit,
		quoted_implicit: quoted_implicit,
		Style:           Style(style),
	}
}

// NewSequenceStartEvent creates a new SEQUENCE-START event.
func NewSequenceStartEvent(anchor, tag []byte, implicit bool, style SequenceStyle) Event {
	return Event{
		Type:     SEQUENCE_START_EVENT,
		Anchor:   anchor,
		Tag:      tag,
		Implicit: implicit,
		Style:    Style(style),
	}
}

// NewSequenceEndEvent creates a new SEQUENCE-END event.
func NewSequenceEndEvent() Event {
	return Event{
		Type: SEQUENCE_END_EVENT,
	}
}

// NewMappingStartEvent creates a new MAPPING-START event.
func NewMappingStartEvent(anchor, tag []byte, implicit bool, style MappingStyle) Event {
	return Event{
		Type:     MAPPING_START_EVENT,
		Anchor:   anchor,
		Tag:      tag,
		Implicit: implicit,
		Style:    Style(style),
	}
}

// NewMappingEndEvent creates a new MAPPING-END event.
func NewMappingEndEvent() Event {
	return Event{
		Type: MAPPING_END_EVENT,
	}
}

// Delete an event object.
func (e *Event) Delete() {
	*e = Event{}
}
