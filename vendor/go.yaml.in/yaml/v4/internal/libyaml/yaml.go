// Copyright 2006-2010 Kirill Simonov
// Copyright 2011-2019 Canonical Ltd
// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0 AND MIT

// Core libyaml types and structures.
// Defines Parser, Emitter, Event, Token, and related constants for YAML
// processing.

package libyaml

import (
	"fmt"
	"strings"
)

// VersionDirective holds the YAML version directive data.
type VersionDirective struct {
	major int8 // The major version number.
	minor int8 // The minor version number.
}

// NewVersionDirective creates YAML version directive data.
func NewVersionDirective(major, minor int) *VersionDirective {
	return &VersionDirective{major: int8(major), minor: int8(minor)}
}

// Major returns the major version number.
func (v *VersionDirective) Major() int { return int(v.major) }

// Minor returns the minor version number.
func (v *VersionDirective) Minor() int { return int(v.minor) }

// TagDirective holds the YAML tag directive data.
type TagDirective struct {
	handle []byte // The tag handle.
	prefix []byte // The tag prefix.
}

// NewTagDirective creates YAML tag directive data.
func NewTagDirective(handle, prefix string) TagDirective {
	return TagDirective{handle: []byte(handle), prefix: []byte(prefix)}
}

// GetHandle returns the tag handle.
func (t *TagDirective) GetHandle() string { return string(t.handle) }

// GetPrefix returns the tag prefix.
func (t *TagDirective) GetPrefix() string { return string(t.prefix) }

// Encoding represents the character encoding of a YAML stream.
type Encoding int

// Possible [Encoding] values.
const (
	// ANY_ENCODING lets the parser choose the encoding.
	ANY_ENCODING Encoding = iota

	// UTF8_ENCODING uses UTF-8 encoding (default).
	UTF8_ENCODING
	// UTF16LE_ENCODING uses UTF-16-LE encoding with BOM.
	UTF16LE_ENCODING
	// UTF16BE_ENCODING uses UTF-16-BE encoding with BOM.
	UTF16BE_ENCODING
)

// LineBreak represents the line break style used in YAML output.
type LineBreak int

// Possible [LineBreak] values.
const (
	// ANY_BREAK lets the parser choose the break type.
	ANY_BREAK LineBreak = iota

	// CR_BREAK uses CR for line breaks (Mac style).
	CR_BREAK
	// LN_BREAK uses LN for line breaks (Unix style).
	LN_BREAK
	// CRLN_BREAK uses CR LN for line breaks (DOS style).
	CRLN_BREAK
)

// QuoteStyle represents the preferred quote style for scalar values.
type QuoteStyle int

// Quote style types for required quoting.
const (
	// QuoteSingle prefers single quotes for scalar values.
	QuoteSingle QuoteStyle = iota
	// QuoteDouble prefers double quotes for scalar values.
	QuoteDouble
	// QuoteLegacy uses double quotes in the representer and single quotes in the emitter.
	QuoteLegacy
)

// ScalarStyle returns the scalar style for this quote preference in the
// representer/serializer context.
// In this context, both QuoteDouble and QuoteLegacy use double quotes.
func (q QuoteStyle) ScalarStyle() ScalarStyle {
	if q == QuoteDouble || q == QuoteLegacy {
		return DOUBLE_QUOTED_SCALAR_STYLE
	}
	return SINGLE_QUOTED_SCALAR_STYLE
}

// ErrorType represents the category of error that occurred during processing.
type ErrorType int

// Possible [ErrorType] values. Many bad things could happen with the parser and emitter.
const (
	// No error is produced.
	NO_ERROR ErrorType = iota

	// MEMORY_ERROR when cannot allocate or reallocate a block of memory.
	MEMORY_ERROR
	// READER_ERROR when cannot read or decode the input stream.
	READER_ERROR
	// SCANNER_ERROR when cannot scan the input stream.
	SCANNER_ERROR
	// PARSER_ERROR when cannot parse the input stream.
	PARSER_ERROR
	// COMPOSER_ERROR when cannot compose a YAML document.
	COMPOSER_ERROR
	// WRITER_ERROR when cannot write to the output stream.
	WRITER_ERROR
	// EMITTER_ERROR when cannot emit a YAML stream.
	EMITTER_ERROR
)

// Mark holds the pointer position.
type Mark struct {
	Index  int // The position index.
	Line   int // The position line (1-indexed; 0 means unknown).
	Column int // The position column (1-indexed; 0 means unknown).
}

// String returns a human-readable string representation of the position mark.
func (m Mark) String() string {
	var builder strings.Builder
	if m.Line == 0 {
		return "<unknown position>"
	}

	fmt.Fprintf(&builder, "line %d", m.Line)
	if m.Column > 0 {
		fmt.Fprintf(&builder, ", column %d", m.Column)
	}

	return builder.String()
}

// ShortString is the exported form of shortString for use by plugins.
func (m Mark) ShortString() string { return m.shortString() }

// RangeString is the exported form of rangeString for use by plugins.
func (m Mark) RangeString(end Mark) string { return m.rangeString(end) }

// shortString returns a compact position string.
// Returns "<unknown position>" when Line is 0 (position not known).
// When Column is 0 (unknown), it is omitted from output ("L{line}");
// otherwise it is displayed as "L{line}.C{col}".
func (m Mark) shortString() string {
	if m.Line == 0 {
		return "<unknown position>"
	}
	if m.Column > 0 {
		return fmt.Sprintf("L%d.C%d", m.Line, m.Column)
	}
	return fmt.Sprintf("L%d", m.Line)
}

// rangeString formats a position range from start mark m to end mark.
// Both marks use shortString for their individual display.
// When marks are on the same line:
//   - Both Column==0: just "L2" (unknown columns, no range shown)
//   - Both Column>0: "L2.C6-C7" (compact column range)
//   - Mixed columns: "L1.C4-L1" (full start with line-only end)
//
// When marks are on different lines: "L1.C8-L2.C3"
func (m Mark) rangeString(end Mark) string {
	start := m.shortString()
	if m.Line == end.Line {
		if m.Column == 0 && end.Column == 0 {
			// Same line, unknown columns: just "L2"
			return start
		}
		if m.Column > 0 && end.Column > 0 {
			if m.Column == end.Column {
				// Same position: just "L2.C6"
				return start
			}
			// Same line with columns: "L2.C6-C7"
			return fmt.Sprintf("%s-C%d", start, end.Column)
		}
	}
	return fmt.Sprintf("%s-%s", start, end.shortString())
}

// Node Styles

// styleInt is the underlying type for style constants.
type styleInt int8

// ScalarStyle represents the formatting style of a scalar value.
type ScalarStyle styleInt

// Possible [ScalarStyle] values.
const (
	// ANY_SCALAR_STYLE lets the emitter choose the style.
	ANY_SCALAR_STYLE ScalarStyle = 0

	// PLAIN_SCALAR_STYLE represents the plain scalar style.
	PLAIN_SCALAR_STYLE ScalarStyle = 1 << iota
	// SINGLE_QUOTED_SCALAR_STYLE represents the single-quoted scalar style.
	SINGLE_QUOTED_SCALAR_STYLE
	// DOUBLE_QUOTED_SCALAR_STYLE represents the double-quoted scalar style.
	DOUBLE_QUOTED_SCALAR_STYLE
	// LITERAL_SCALAR_STYLE represents the literal scalar style.
	LITERAL_SCALAR_STYLE
	// FOLDED_SCALAR_STYLE represents the folded scalar style.
	FOLDED_SCALAR_STYLE
)

// String returns a string representation of a [ScalarStyle].
func (style ScalarStyle) String() string {
	switch style {
	case PLAIN_SCALAR_STYLE:
		return "Plain"
	case SINGLE_QUOTED_SCALAR_STYLE:
		return "Single"
	case DOUBLE_QUOTED_SCALAR_STYLE:
		return "Double"
	case LITERAL_SCALAR_STYLE:
		return "Literal"
	case FOLDED_SCALAR_STYLE:
		return "Folded"
	default:
		return ""
	}
}

// SequenceStyle represents the formatting style of a sequence node.
type SequenceStyle styleInt

// Possible [SequenceStyle] values.
const (
	// ANY_SEQUENCE_STYLE lets the emitter choose the style.
	ANY_SEQUENCE_STYLE SequenceStyle = iota

	// BLOCK_SEQUENCE_STYLE represents a block sequence style.
	BLOCK_SEQUENCE_STYLE
	// FLOW_SEQUENCE_STYLE represents a flow sequence style.
	FLOW_SEQUENCE_STYLE
)

// MappingStyle represents the formatting style of a mapping node.
type MappingStyle styleInt

// Possible [MappingStyle] values.
const (
	// ANY_MAPPING_STYLE lets the emitter choose the style.
	ANY_MAPPING_STYLE MappingStyle = iota

	// BLOCK_MAPPING_STYLE represents a block mapping style.
	BLOCK_MAPPING_STYLE
	// FLOW_MAPPING_STYLE represents a flow mapping style.
	FLOW_MAPPING_STYLE
)

// Tokens

// TokenType represents the [Token.Type] of a scanned [Token].
type TokenType int

// Possible [TokenType] values.
const (
	// NO_TOKEN represents an empty token.
	NO_TOKEN TokenType = iota

	// STREAM_START_TOKEN represents a STREAM-START token.
	STREAM_START_TOKEN
	// STREAM_END_TOKEN represents a STREAM-END token.
	STREAM_END_TOKEN
	// VERSION_DIRECTIVE_TOKEN represents a VERSION-DIRECTIVE token.
	VERSION_DIRECTIVE_TOKEN

	// TAG_DIRECTIVE_TOKEN represents a TAG-DIRECTIVE token.
	TAG_DIRECTIVE_TOKEN
	// DOCUMENT_START_TOKEN represents a DOCUMENT-START token.
	DOCUMENT_START_TOKEN
	// DOCUMENT_END_TOKEN represents a DOCUMENT-END token.
	DOCUMENT_END_TOKEN

	// BLOCK_SEQUENCE_START_TOKEN represents a BLOCK-SEQUENCE-START token.
	BLOCK_SEQUENCE_START_TOKEN
	// BLOCK_MAPPING_START_TOKEN represents a BLOCK-MAPPING-START token.
	BLOCK_MAPPING_START_TOKEN
	// BLOCK_END_TOKEN represents a BLOCK-END token.
	BLOCK_END_TOKEN

	// FLOW_SEQUENCE_START_TOKEN represents a FLOW-SEQUENCE-START token.
	FLOW_SEQUENCE_START_TOKEN
	// FLOW_SEQUENCE_END_TOKEN represents a FLOW-SEQUENCE-END token.
	FLOW_SEQUENCE_END_TOKEN
	// FLOW_MAPPING_START_TOKEN represents a FLOW-MAPPING-START token.
	FLOW_MAPPING_START_TOKEN
	// FLOW_MAPPING_END_TOKEN represents a FLOW-MAPPING-END token.
	FLOW_MAPPING_END_TOKEN

	// BLOCK_ENTRY_TOKEN represents a BLOCK-ENTRY token.
	BLOCK_ENTRY_TOKEN
	// FLOW_ENTRY_TOKEN represents a FLOW-ENTRY token.
	FLOW_ENTRY_TOKEN
	// KEY_TOKEN represents a KEY token.
	KEY_TOKEN
	// VALUE_TOKEN represents a VALUE token.
	VALUE_TOKEN

	// ALIAS_TOKEN represents an ALIAS token.
	ALIAS_TOKEN
	// ANCHOR_TOKEN represents an ANCHOR token.
	ANCHOR_TOKEN
	// TAG_TOKEN represents a TAG token.
	TAG_TOKEN
	// SCALAR_TOKEN represents a SCALAR token.
	SCALAR_TOKEN
	// COMMENT_TOKEN represents a COMMENT token.
	COMMENT_TOKEN
)

// String returns a string representation of the token type.
func (tt TokenType) String() string {
	switch tt {
	case NO_TOKEN:
		return "NO_TOKEN"
	case STREAM_START_TOKEN:
		return "STREAM_START_TOKEN"
	case STREAM_END_TOKEN:
		return "STREAM_END_TOKEN"
	case VERSION_DIRECTIVE_TOKEN:
		return "VERSION_DIRECTIVE_TOKEN"
	case TAG_DIRECTIVE_TOKEN:
		return "TAG_DIRECTIVE_TOKEN"
	case DOCUMENT_START_TOKEN:
		return "DOCUMENT_START_TOKEN"
	case DOCUMENT_END_TOKEN:
		return "DOCUMENT_END_TOKEN"
	case BLOCK_SEQUENCE_START_TOKEN:
		return "BLOCK_SEQUENCE_START_TOKEN"
	case BLOCK_MAPPING_START_TOKEN:
		return "BLOCK_MAPPING_START_TOKEN"
	case BLOCK_END_TOKEN:
		return "BLOCK_END_TOKEN"
	case FLOW_SEQUENCE_START_TOKEN:
		return "FLOW_SEQUENCE_START_TOKEN"
	case FLOW_SEQUENCE_END_TOKEN:
		return "FLOW_SEQUENCE_END_TOKEN"
	case FLOW_MAPPING_START_TOKEN:
		return "FLOW_MAPPING_START_TOKEN"
	case FLOW_MAPPING_END_TOKEN:
		return "FLOW_MAPPING_END_TOKEN"
	case BLOCK_ENTRY_TOKEN:
		return "BLOCK_ENTRY_TOKEN"
	case FLOW_ENTRY_TOKEN:
		return "FLOW_ENTRY_TOKEN"
	case KEY_TOKEN:
		return "KEY_TOKEN"
	case VALUE_TOKEN:
		return "VALUE_TOKEN"
	case ALIAS_TOKEN:
		return "ALIAS_TOKEN"
	case ANCHOR_TOKEN:
		return "ANCHOR_TOKEN"
	case TAG_TOKEN:
		return "TAG_TOKEN"
	case SCALAR_TOKEN:
		return "SCALAR_TOKEN"
	case COMMENT_TOKEN:
		return "COMMENT_TOKEN"
	}
	return "<unknown token>"
}

// Token holds information about a scanning token.
type Token struct {
	// The token type.
	Type TokenType

	// The start/end of the token.
	StartMark, EndMark Mark

	// The stream encoding (for STREAM_START_TOKEN).
	encoding Encoding

	// The alias/anchor/scalar Value or tag/tag directive handle
	// (for ALIAS_TOKEN, ANCHOR_TOKEN, SCALAR_TOKEN, TAG_TOKEN, TAG_DIRECTIVE_TOKEN).
	Value []byte

	// The tag suffix (for TAG_TOKEN).
	suffix []byte

	// The tag directive prefix (for TAG_DIRECTIVE_TOKEN).
	prefix []byte

	// The scalar Style (for SCALAR_TOKEN).
	Style ScalarStyle

	// The version directive major/minor (for VERSION_DIRECTIVE_TOKEN).
	major, minor int8
}

// GetEncoding returns the stream encoding carried by a STREAM-START token.
func (t *Token) GetEncoding() Encoding { return t.encoding }

// SetEncoding sets the stream encoding carried by a STREAM-START token.
func (t *Token) SetEncoding(encoding Encoding) { t.encoding = encoding }

// GetSuffix returns the suffix carried by a TAG token.
func (t *Token) GetSuffix() string { return string(t.suffix) }

// SetSuffix sets the suffix carried by a TAG token.
func (t *Token) SetSuffix(suffix string) { t.suffix = []byte(suffix) }

// GetPrefix returns the prefix carried by a TAG-DIRECTIVE token.
func (t *Token) GetPrefix() string { return string(t.prefix) }

// SetPrefix sets the prefix carried by a TAG-DIRECTIVE token.
func (t *Token) SetPrefix(prefix string) { t.prefix = []byte(prefix) }

// GetVersion returns the version carried by a VERSION-DIRECTIVE token.
func (t *Token) GetVersion() (int, int) { return int(t.major), int(t.minor) }

// SetVersion sets the version carried by a VERSION-DIRECTIVE token.
func (t *Token) SetVersion(major, minor int) {
	t.major = int8(major)
	t.minor = int8(minor)
}

// Events

// EventType represents the type of a parsing or emitting event.
type EventType int8

// Event types.
const (
	// NO_EVENT represents an empty event.
	NO_EVENT EventType = iota
	// STREAM_START_EVENT represents a STREAM-START event.
	STREAM_START_EVENT
	// STREAM_END_EVENT represents a STREAM-END event.
	STREAM_END_EVENT
	// DOCUMENT_START_EVENT represents a DOCUMENT-START event.
	DOCUMENT_START_EVENT
	// DOCUMENT_END_EVENT represents a DOCUMENT-END event.
	DOCUMENT_END_EVENT
	// ALIAS_EVENT represents an ALIAS event.
	ALIAS_EVENT
	// SCALAR_EVENT represents a SCALAR event.
	SCALAR_EVENT
	// SEQUENCE_START_EVENT represents a SEQUENCE-START event.
	SEQUENCE_START_EVENT
	// SEQUENCE_END_EVENT represents a SEQUENCE-END event.
	SEQUENCE_END_EVENT
	// MAPPING_START_EVENT represents a MAPPING-START event.
	MAPPING_START_EVENT
	// MAPPING_END_EVENT represents a MAPPING-END event.
	MAPPING_END_EVENT
	// TAIL_COMMENT_EVENT represents a TAIL-COMMENT event.
	TAIL_COMMENT_EVENT
)

// eventStrings maps EventType constants to their string representations.
var eventStrings = []string{
	NO_EVENT:             "none",
	STREAM_START_EVENT:   "stream start",
	STREAM_END_EVENT:     "stream end",
	DOCUMENT_START_EVENT: "document start",
	DOCUMENT_END_EVENT:   "document end",
	ALIAS_EVENT:          "alias",
	SCALAR_EVENT:         "scalar",
	SEQUENCE_START_EVENT: "sequence start",
	SEQUENCE_END_EVENT:   "sequence end",
	MAPPING_START_EVENT:  "mapping start",
	MAPPING_END_EVENT:    "mapping end",
	TAIL_COMMENT_EVENT:   "tail comment",
}

// String returns a string representation of the event type.
func (e EventType) String() string {
	if e < 0 || int(e) >= len(eventStrings) {
		return fmt.Sprintf("unknown event %d", e)
	}
	return eventStrings[e]
}

// Event holds information about a parsing or emitting event.
type Event struct {
	// The event type.
	Type EventType

	// The start and end of the event.
	StartMark, EndMark Mark

	// The document encoding (for STREAM_START_EVENT).
	encoding Encoding

	// The version directive (for DOCUMENT_START_EVENT).
	versionDirective *VersionDirective

	// The list of tag directives (for DOCUMENT_START_EVENT).
	tagDirectives []TagDirective

	// The comments
	HeadComment []byte
	LineComment []byte
	FootComment []byte
	TailComment []byte

	// The Anchor (for SCALAR_EVENT, SEQUENCE_START_EVENT, MAPPING_START_EVENT, ALIAS_EVENT).
	Anchor []byte

	// The Tag (for SCALAR_EVENT, SEQUENCE_START_EVENT, MAPPING_START_EVENT).
	Tag []byte

	// The scalar Value (for SCALAR_EVENT).
	Value []byte

	// Is the document start/end indicator Implicit, or the tag optional?
	// (for DOCUMENT_START_EVENT, DOCUMENT_END_EVENT, SEQUENCE_START_EVENT, MAPPING_START_EVENT, SCALAR_EVENT).
	Implicit bool

	// Is the tag optional for any non-plain style? (for SCALAR_EVENT).
	quoted_implicit bool

	// The Style (for SCALAR_EVENT, SEQUENCE_START_EVENT, MAPPING_START_EVENT).
	Style Style
}

// ScalarStyle returns the style of a scalar event.
func (e *Event) ScalarStyle() ScalarStyle { return ScalarStyle(e.Style) }

// SequenceStyle returns the style of a sequence event.
func (e *Event) SequenceStyle() SequenceStyle { return SequenceStyle(e.Style) }

// MappingStyle returns the style of a mapping event.
func (e *Event) MappingStyle() MappingStyle { return MappingStyle(e.Style) }

// GetEncoding returns the stream encoding (for STREAM_START_EVENT).
func (e *Event) GetEncoding() Encoding { return e.encoding }

// GetVersionDirective returns the version directive (for DOCUMENT_START_EVENT).
func (e *Event) GetVersionDirective() *VersionDirective { return e.versionDirective }

// GetTagDirectives returns the tag directives (for DOCUMENT_START_EVENT).
func (e *Event) GetTagDirectives() []TagDirective { return e.tagDirectives }

// GetQuotedImplicit reports whether a scalar tag may be omitted for a
// non-plain scalar style.
func (e *Event) GetQuotedImplicit() bool { return e.quoted_implicit }

// Possible values for [Node.Tag]
const (
	// NULL_TAG is the tag !!null with the only possible value: null.
	NULL_TAG = "tag:yaml.org,2002:null"
	// BOOL_TAG is the tag !!bool with the values: true and false.
	BOOL_TAG = "tag:yaml.org,2002:bool"
	// STR_TAG is the tag !!str for string values.
	STR_TAG = "tag:yaml.org,2002:str"
	// INT_TAG is the tag !!int for integer values.
	INT_TAG = "tag:yaml.org,2002:int"
	// FLOAT_TAG is the tag !!float for float values.
	FLOAT_TAG = "tag:yaml.org,2002:float"
	// TIMESTAMP_TAG is the tag !!timestamp for date and time values.
	TIMESTAMP_TAG = "tag:yaml.org,2002:timestamp"
	// SEQ_TAG is the tag !!seq for sequences.
	SEQ_TAG = "tag:yaml.org,2002:seq"
	// MAP_TAG is the tag !!map for mappings.
	MAP_TAG = "tag:yaml.org,2002:map"
)

// Additional possible values for [Node.Tag]
// These are the tags that are not in the original [libyaml] but are used in go-yaml for specific purposes.
//
// [libyaml]: https://github.com/yaml/libyaml
const (
	// BINARY_TAG is the tag !!binary for binary data.
	BINARY_TAG = "tag:yaml.org,2002:binary"
	// MERGE_TAG is the tag !!merge for merging mappings.
	MERGE_TAG = "tag:yaml.org,2002:merge"
	// DEFAULT_SCALAR_TAG is the default tag for scalars, which is !!str.
	DEFAULT_SCALAR_TAG = STR_TAG
	// DEFAULT_SEQUENCE_TAG is the default tag for sequences, which is !!seq.
	DEFAULT_SEQUENCE_TAG = SEQ_TAG
	// DEFAULT_MAPPING_TAG is the default tag for mappings, which is !!map.
	DEFAULT_MAPPING_TAG = MAP_TAG
)
