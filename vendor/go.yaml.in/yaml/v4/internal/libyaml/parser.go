// Copyright 2006-2010 Kirill Simonov
// Copyright 2011-2019 Canonical Ltd
// Copyright 2025 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0 AND MIT

// Parser stage: Transforms token stream into event stream.
// Implements a recursive-descent parser (LL(1)) following the YAML grammar
// specification.
//
// The parser implements the following grammar:
//
// stream               ::= STREAM-START implicit_document? explicit_document* STREAM-END
// implicit_document    ::= block_node DOCUMENT-END*
// explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
// block_node_or_indentless_sequence    ::=
//                          ALIAS
//                          | properties (block_content | indentless_block_sequence)?
//                          | block_content
//                          | indentless_block_sequence
// block_node           ::= ALIAS
//                          | properties block_content?
//                          | block_content
// flow_node            ::= ALIAS
//                          | properties flow_content?
//                          | flow_content
// properties           ::= TAG ANCHOR? | ANCHOR TAG?
// block_content        ::= block_collection | flow_collection | SCALAR
// flow_content         ::= flow_collection | SCALAR
// block_collection     ::= block_sequence | block_mapping
// flow_collection      ::= flow_sequence | flow_mapping
// block_sequence       ::= BLOCK-SEQUENCE-START (BLOCK-ENTRY block_node?)* BLOCK-END
// indentless_sequence  ::= (BLOCK-ENTRY block_node?)+
// block_mapping        ::= BLOCK-MAPPING_START
//                          ((KEY block_node_or_indentless_sequence?)?
//                          (VALUE block_node_or_indentless_sequence?)?)*
//                          BLOCK-END
// flow_sequence        ::= FLOW-SEQUENCE-START
//                          (flow_sequence_entry FLOW-ENTRY)*
//                          flow_sequence_entry?
//                          FLOW-SEQUENCE-END
// flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
// flow_mapping         ::= FLOW-MAPPING-START
//                          (flow_mapping_entry FLOW-ENTRY)*
//                          flow_mapping_entry?
//                          FLOW-MAPPING-END
// flow_mapping_entry   ::= flow_node | KEY flow_node? (VALUE flow_node?)?

package libyaml

import (
	"bytes"
	"io"
	"strings"
)

// ReadHandler is called by the [Parser] when it needs to read more bytes
// from the input source.  The handler should fill the provided buffer with
// up to len(buffer) bytes from the input source.
//
// The arguments are as follows:
//
// [in]       parser      The parser object.
// [out]      buffer      The buffer for reading.
// [out]      size_read   The actual number of bytes read from the source.
//
// On success, the handler should return 1.  If the handler failed,
// the returned value should be 0. On EOF, the handler should set the
// size_read to 0 and return 1.
type ReadHandler func(parser *Parser, buffer []byte) (n int, err error)

// SimpleKey holds information about a potential simple key.
type SimpleKey struct {
	flow_level   int  // What flow level is the key at?
	required     bool // Is a simple key required?
	token_number int  // The number of the token.
	mark         Mark // The position mark.
}

// ParserState represents the state of the parser.
type ParserState int

// Possible [ParserState] constants are the different states the parser can be in.
const (
	// PARSE_STREAM_START_STATE expects STREAM-START.
	PARSE_STREAM_START_STATE ParserState = iota

	// PARSE_IMPLICIT_DOCUMENT_START_STATE expects the beginning of an implicit document.
	PARSE_IMPLICIT_DOCUMENT_START_STATE
	// PARSE_DOCUMENT_START_STATE expects DOCUMENT-START.
	PARSE_DOCUMENT_START_STATE
	// PARSE_DOCUMENT_CONTENT_STATE expects the content of a document.
	PARSE_DOCUMENT_CONTENT_STATE
	// PARSE_DOCUMENT_END_STATE expects DOCUMENT-END.
	PARSE_DOCUMENT_END_STATE
	// PARSE_BLOCK_NODE_STATE expects a block node.
	PARSE_BLOCK_NODE_STATE
	// PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE expects the first entry of a block sequence.
	PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE
	// PARSE_BLOCK_SEQUENCE_ENTRY_STATE expects an entry of a block sequence.
	PARSE_BLOCK_SEQUENCE_ENTRY_STATE
	// PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE expects an entry of an indentless sequence.
	PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE
	// PARSE_BLOCK_MAPPING_FIRST_KEY_STATE expects the first key of a block mapping.
	PARSE_BLOCK_MAPPING_FIRST_KEY_STATE
	// PARSE_BLOCK_MAPPING_KEY_STATE expects a block mapping key.
	PARSE_BLOCK_MAPPING_KEY_STATE
	// PARSE_BLOCK_MAPPING_VALUE_STATE expects a block mapping value.
	PARSE_BLOCK_MAPPING_VALUE_STATE
	// PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE expects the first entry of a flow sequence.
	PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE
	// PARSE_FLOW_SEQUENCE_ENTRY_STATE expects an entry of a flow sequence.
	PARSE_FLOW_SEQUENCE_ENTRY_STATE
	// PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE expects a key of an ordered mapping.
	PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE
	// PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE expects a value of an ordered mapping.
	PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE
	// PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE expects the and of an ordered mapping entry.
	PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE
	// PARSE_FLOW_MAPPING_FIRST_KEY_STATE expects the first key of a flow mapping.
	PARSE_FLOW_MAPPING_FIRST_KEY_STATE
	// PARSE_FLOW_MAPPING_KEY_STATE expects a key of a flow mapping.
	PARSE_FLOW_MAPPING_KEY_STATE
	// PARSE_FLOW_MAPPING_VALUE_STATE expects a value of a flow mapping.
	PARSE_FLOW_MAPPING_VALUE_STATE
	// PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE expects an empty value of a flow mapping.
	PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE
	// PARSE_END_STATE expects nothing.
	PARSE_END_STATE
)

// String returns a string representation of the parser state.
func (ps ParserState) String() string {
	switch ps {
	case PARSE_STREAM_START_STATE:
		return "PARSE_STREAM_START_STATE"
	case PARSE_IMPLICIT_DOCUMENT_START_STATE:
		return "PARSE_IMPLICIT_DOCUMENT_START_STATE"
	case PARSE_DOCUMENT_START_STATE:
		return "PARSE_DOCUMENT_START_STATE"
	case PARSE_DOCUMENT_CONTENT_STATE:
		return "PARSE_DOCUMENT_CONTENT_STATE"
	case PARSE_DOCUMENT_END_STATE:
		return "PARSE_DOCUMENT_END_STATE"
	case PARSE_BLOCK_NODE_STATE:
		return "PARSE_BLOCK_NODE_STATE"
	case PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE:
		return "PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE"
	case PARSE_BLOCK_SEQUENCE_ENTRY_STATE:
		return "PARSE_BLOCK_SEQUENCE_ENTRY_STATE"
	case PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE:
		return "PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE"
	case PARSE_BLOCK_MAPPING_FIRST_KEY_STATE:
		return "PARSE_BLOCK_MAPPING_FIRST_KEY_STATE"
	case PARSE_BLOCK_MAPPING_KEY_STATE:
		return "PARSE_BLOCK_MAPPING_KEY_STATE"
	case PARSE_BLOCK_MAPPING_VALUE_STATE:
		return "PARSE_BLOCK_MAPPING_VALUE_STATE"
	case PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE:
		return "PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE"
	case PARSE_FLOW_SEQUENCE_ENTRY_STATE:
		return "PARSE_FLOW_SEQUENCE_ENTRY_STATE"
	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE:
		return "PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE"
	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE:
		return "PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE"
	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE:
		return "PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE"
	case PARSE_FLOW_MAPPING_FIRST_KEY_STATE:
		return "PARSE_FLOW_MAPPING_FIRST_KEY_STATE"
	case PARSE_FLOW_MAPPING_KEY_STATE:
		return "PARSE_FLOW_MAPPING_KEY_STATE"
	case PARSE_FLOW_MAPPING_VALUE_STATE:
		return "PARSE_FLOW_MAPPING_VALUE_STATE"
	case PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE:
		return "PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE"
	case PARSE_END_STATE:
		return "PARSE_END_STATE"
	}
	return "<unknown parser state>"
}

// AliasData holds information about aliases.
type AliasData struct {
	anchor []byte // The anchor.
	index  int    // The node id.
	mark   Mark   // The anchor mark.
}

// Comment holds information about a comment in the YAML stream.
type Comment struct {
	// ScanMark is the position where scanning for comments started
	ScanMark Mark
	// TokenMark is the position after which tokens will be associated with this comment
	TokenMark Mark
	// StartMark is the position of the '#' comment mark
	StartMark Mark
	// EndMark is the position where the comment ended
	EndMark Mark

	Head []byte
	Line []byte
	Foot []byte
}

const indentStyleUnknown byte = 0

// Parser structure holds all information about the current
// state of the parser.
type Parser struct {
	lastError error

	// Reader stuff
	read_handler ReadHandler // Read handler.

	input_reader io.Reader // File input data.
	input        []byte    // String input data.
	input_pos    int

	eof bool // EOF flag

	buffer     []byte // The working buffer.
	buffer_pos int    // The current position of the buffer.

	unread int // The number of unread characters in the buffer.

	newlines int // The number of line breaks since last non-break/non-blank character

	raw_buffer     []byte // The raw buffer.
	raw_buffer_pos int    // The current position of the buffer.

	encoding Encoding // The input encoding.

	offset int  // The offset of the current position (in bytes).
	mark   Mark // The mark of the current position.

	// Comments

	// HeadComment is the current head comments.
	HeadComment []byte
	// LineComment is the current line comments.
	LineComment []byte
	// FootComment is the current foot comments.
	FootComment  []byte
	tail_comment []byte // Foot comment that happens at the end of a block.
	stem_comment []byte // Comment in item preceding a nested structure (list inside list item, etc)

	comments      []Comment // The folded comments for all parsed tokens
	comments_head int

	skip_comments bool // Skip comment scanning for performance

	// Scanner stuff

	stream_start_produced bool // Have we started to scan the input stream?
	stream_end_produced   bool // Have we reached the end of the input stream?

	flow_level int // The number of unclosed '[' and '{' indicators.

	tokens          []Token // The tokens queue.
	tokens_head     int     // The head of the tokens queue.
	tokens_parsed   int     // The number of tokens fetched from the queue.
	token_available bool    // Does the tokens queue contain a token ready for dequeueing.

	indent  int   // The current indentation level.
	indents []int // The indentation levels stack.

	block_indent_tab Mark // First tab before a possible block collection.

	simple_key_allowed  bool        // May a simple key occur at the current position?
	simple_key_possible bool        // Is the current simple key possible?
	simple_key          SimpleKey   // The current simple key.
	simple_key_stack    []SimpleKey // The stack of simple keys.

	depthCheck  func(int, *DepthContext) error // Depth limit check function
	formatError func(*LoadError) string        // Optional error formatter

	indentConfig *IndentConfig
	// indentStyle is parser-local auto-detection state.
	// Keeping it here leaves reusable indentation configuration immutable.
	indentStyle byte

	// Parser stuff

	state          ParserState    // The current parser state.
	states         []ParserState  // The parser states stack.
	marks          []Mark         // The stack of marks.
	tag_directives []TagDirective // The list of TAG directives.

	// Representer stuff

	aliases []AliasData // The alias data.

	// Optional pre-tokenized and pre-parsed input used by stage-oriented
	// tools. These bypass the reader/scanner or parser respectively.
	token_input bool
	event_input bool
	events      []Event
	events_head int
}

// SetIndentConfig configures tab-aware indentation scanning.
func (parser *Parser) SetIndentConfig(config *IndentConfig) {
	parser.indentConfig = config
}

// NewParser creates a new parser object.
func NewParser() Parser {
	return Parser{
		raw_buffer: make([]byte, 0, input_raw_buffer_size),
		buffer:     make([]byte, 0, input_buffer_size),
		mark:       Mark{Line: 1, Column: 1},
		depthCheck: DefaultDepthCheck,
	}
}

// Parse gets the next event.
func (parser *Parser) Parse(event *Event) error {
	// Erase the event object.
	*event = Event{}

	if parser.event_input {
		if parser.events_head >= len(parser.events) {
			return io.EOF
		}
		*event = parser.events[parser.events_head]
		parser.events_head++
		return nil
	}

	if parser.lastError != nil {
		return parser.lastError
	}

	// No events after the end of the stream or error.
	if parser.stream_end_produced || parser.state == PARSE_END_STATE {
		return io.EOF
	}

	// Generate the next event.
	if err := parser.stateMachine(event); err != nil {
		parser.lastError = err
		return err
	}

	return nil
}

// Delete a parser object.
func (parser *Parser) Delete() {
	*parser = Parser{}
}

// String read handler.
func yamlStringReadHandler(parser *Parser, buffer []byte) (n int, err error) {
	if parser.input_pos == len(parser.input) {
		return 0, io.EOF
	}
	n = copy(buffer, parser.input[parser.input_pos:])
	parser.input_pos += n
	return n, nil
}

// Reader read handler.
func yamlReaderReadHandler(parser *Parser, buffer []byte) (n int, err error) {
	return parser.input_reader.Read(buffer)
}

// SetInputString sets a string input.
func (parser *Parser) SetInputString(input []byte) {
	if parser.read_handler != nil {
		panic("must set the input source only once")
	}
	parser.read_handler = yamlStringReadHandler
	parser.input = input
	parser.input_pos = 0
}

// SetInputReader sets a file input.
func (parser *Parser) SetInputReader(r io.Reader) {
	if parser.read_handler != nil {
		panic("must set the input source only once")
	}
	parser.read_handler = yamlReaderReadHandler
	parser.input_reader = r
}

// SetInputTokens sets a pre-tokenized input stream. It is intended for
// internal pipeline tooling that needs to resume processing at the parser
// stage.
func (parser *Parser) SetInputTokens(tokens []Token, comments []Comment) {
	if parser.read_handler != nil || parser.token_input || parser.event_input {
		panic("must set the input source only once")
	}
	parser.token_input = true
	parser.tokens = append([]Token(nil), tokens...)
	parser.comments = append([]Comment(nil), comments...)
}

// SetInputEvents sets a pre-parsed event stream. It is intended for internal
// pipeline tooling that needs to resume processing at the composer stage.
func (parser *Parser) SetInputEvents(events []Event) {
	if parser.read_handler != nil || parser.token_input || parser.event_input {
		panic("must set the input source only once")
	}
	parser.event_input = true
	parser.events = append([]Event(nil), events...)
}

// SetEncoding sets the source encoding.
func (parser *Parser) SetEncoding(encoding Encoding) {
	if parser.encoding != ANY_ENCODING {
		panic("must set the encoding only once")
	}
	parser.encoding = encoding
}

// GetPendingComments returns the parser's comment queue for CLI access.
func (parser *Parser) GetPendingComments() []Comment {
	return parser.comments
}

// GetCommentsHead returns the current position in the comment queue.
func (parser *Parser) GetCommentsHead() int {
	return parser.comments_head
}

// SetSkipComments enables or disables comment scanning.
// When enabled, the scanner skips comment tokens for better performance.
func (parser *Parser) SetSkipComments(skip bool) {
	parser.skip_comments = skip
}

// default_tag_directives defines the standard tag directives (! and !!)
// that are implicitly available in all YAML documents.
var default_tag_directives = []TagDirective{
	{[]byte("!"), []byte("!")},
	{[]byte("!!"), []byte("tag:yaml.org,2002:")},
}

// State dispatcher.
func (parser *Parser) stateMachine(event *Event) error {
	// trace("yaml_parser_state_machine", "state:", parser.state.String())

	switch parser.state {
	case PARSE_STREAM_START_STATE:
		return parser.parseStreamStart(event)

	case PARSE_IMPLICIT_DOCUMENT_START_STATE:
		return parser.parseDocumentStart(event, true)

	case PARSE_DOCUMENT_START_STATE:
		return parser.parseDocumentStart(event, false)

	case PARSE_DOCUMENT_CONTENT_STATE:
		return parser.parseDocumentContent(event)

	case PARSE_DOCUMENT_END_STATE:
		return parser.parseDocumentEnd(event)

	case PARSE_BLOCK_NODE_STATE:
		return parser.parseNode(event, true, false)

	case PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE:
		return parser.parseBlockSequenceEntry(event, true)

	case PARSE_BLOCK_SEQUENCE_ENTRY_STATE:
		return parser.parseBlockSequenceEntry(event, false)

	case PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE:
		return parser.parseIndentlessSequenceEntry(event)

	case PARSE_BLOCK_MAPPING_FIRST_KEY_STATE:
		return parser.parseBlockMappingKey(event, true)

	case PARSE_BLOCK_MAPPING_KEY_STATE:
		return parser.parseBlockMappingKey(event, false)

	case PARSE_BLOCK_MAPPING_VALUE_STATE:
		return parser.parseBlockMappingValue(event)

	case PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE:
		return parser.parseFlowSequenceEntry(event, true)

	case PARSE_FLOW_SEQUENCE_ENTRY_STATE:
		return parser.parseFlowSequenceEntry(event, false)

	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE:
		return parser.parseFlowSequenceEntryMappingKey(event)

	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE:
		return parser.parseFlowSequenceEntryMappingValue(event)

	case PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE:
		return parser.parseFlowSequenceEntryMappingEnd(event)

	case PARSE_FLOW_MAPPING_FIRST_KEY_STATE:
		return parser.parseFlowMappingKey(event, true)

	case PARSE_FLOW_MAPPING_KEY_STATE:
		return parser.parseFlowMappingKey(event, false)

	case PARSE_FLOW_MAPPING_VALUE_STATE:
		return parser.parseFlowMappingValue(event, false)

	case PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE:
		return parser.parseFlowMappingValue(event, true)

	default:
		panic("invalid parser state")
	}
}

// Parse the production:
//
//	stream   ::= STREAM-START implicit_document? explicit_document* STREAM-END
//	             ************
func (parser *Parser) parseStreamStart(event *Event) error {
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}
	if token.Type != STREAM_START_TOKEN {
		return parser.formatParserError("did not find expected <stream-start>", token.StartMark)
	}
	parser.state = PARSE_IMPLICIT_DOCUMENT_START_STATE
	*event = Event{
		Type:      STREAM_START_EVENT,
		StartMark: token.StartMark,
		EndMark:   token.EndMark,
		encoding:  token.encoding,
	}
	parser.skipToken()
	return nil
}

// Parse the productions:
//
//	implicit_document    ::= block_node DOCUMENT-END*
//	                         *
//	explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
//	                         *************************
func (parser *Parser) parseDocumentStart(event *Event, implicit bool) error {
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}

	// Parse extra document end indicators.
	for token.Type == DOCUMENT_END_TOKEN {
		parser.skipToken()
		if err := parser.peekToken(&token); err != nil {
			return err
		}
	}

	if implicit && token.Type != VERSION_DIRECTIVE_TOKEN &&
		token.Type != TAG_DIRECTIVE_TOKEN &&
		token.Type != DOCUMENT_START_TOKEN &&
		token.Type != STREAM_END_TOKEN {
		// Parse an implicit document.
		if err := parser.processDirectives(nil, nil); err != nil {
			return err
		}
		parser.states = append(parser.states, PARSE_DOCUMENT_END_STATE)
		parser.state = PARSE_BLOCK_NODE_STATE

		var head_comment []byte
		if len(parser.HeadComment) > 0 {
			// [Go] Scan the header comment backwards, and if an
			// empty line is found, break the header so the part
			// before the last empty line goes into the document
			// header, while the bottom of it goes into a follow up
			// event.
			for i := len(parser.HeadComment) - 1; i > 0; i-- {
				if parser.HeadComment[i] == '\n' {
					if i == len(parser.HeadComment)-1 {
						head_comment = parser.HeadComment[:i]
						parser.HeadComment = parser.HeadComment[i+1:]
						break
					} else if parser.HeadComment[i-1] == '\n' {
						head_comment = parser.HeadComment[:i-1]
						parser.HeadComment = parser.HeadComment[i+1:]
						break
					}
				}
			}
		}

		*event = Event{
			Type:      DOCUMENT_START_EVENT,
			StartMark: token.StartMark,
			EndMark:   token.EndMark,
			Implicit:  true,

			HeadComment: head_comment,
		}

	} else if token.Type != STREAM_END_TOKEN {
		// Parse an explicit document.
		var version_directive *VersionDirective
		var tag_directives []TagDirective
		start_mark := token.StartMark
		if err := parser.processDirectives(&version_directive, &tag_directives); err != nil {
			return err
		}
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		if token.Type != DOCUMENT_START_TOKEN {
			return parser.formatParserError(
				"did not find expected <document start>", token.StartMark)
		}
		parser.states = append(parser.states, PARSE_DOCUMENT_END_STATE)
		parser.state = PARSE_DOCUMENT_CONTENT_STATE
		end_mark := token.EndMark

		*event = Event{
			Type:             DOCUMENT_START_EVENT,
			StartMark:        start_mark,
			EndMark:          end_mark,
			versionDirective: version_directive,
			tagDirectives:    tag_directives,
			Implicit:         false,
		}
		parser.skipToken()

	} else {
		// Parse the stream end.
		parser.state = PARSE_END_STATE
		*event = Event{
			Type:      STREAM_END_EVENT,
			StartMark: token.StartMark,
			EndMark:   token.EndMark,
		}
		parser.setEventComments(event)
		parser.skipToken()
	}

	return nil
}

// Parse the productions:
//
//	explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
//	                                                   ***********
func (parser *Parser) parseDocumentContent(event *Event) error {
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}

	if token.Type == VERSION_DIRECTIVE_TOKEN ||
		token.Type == TAG_DIRECTIVE_TOKEN ||
		token.Type == DOCUMENT_START_TOKEN ||
		token.Type == DOCUMENT_END_TOKEN ||
		token.Type == STREAM_END_TOKEN {
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]
		return parser.processEmptyScalar(event,
			token.StartMark)
	}
	return parser.parseNode(event, true, false)
}

// Parse the productions:
//
//	implicit_document    ::= block_node DOCUMENT-END*
//	                                    *************
//	explicit_document    ::= DIRECTIVE* DOCUMENT-START block_node? DOCUMENT-END*
func (parser *Parser) parseDocumentEnd(event *Event) error {
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}

	start_mark := token.StartMark
	end_mark := token.StartMark

	implicit := true
	switch token.Type {
	case DOCUMENT_END_TOKEN:
		end_mark = token.EndMark
		parser.skipToken()
		implicit = false
	case VERSION_DIRECTIVE_TOKEN, TAG_DIRECTIVE_TOKEN:
		return parser.formatParserError(
			"did not find expected <document end> before directive", token.StartMark)
	case DOCUMENT_START_TOKEN, STREAM_END_TOKEN:
		// Document ended properly.
		// https://github.com/yaml/go-yaml/pull/410
	default:
		return parser.formatParserError(
			"did not find expected <document start>", token.StartMark)
	}

	parser.tag_directives = parser.tag_directives[:0]

	parser.state = PARSE_DOCUMENT_START_STATE
	if !implicit {
		parser.state = PARSE_IMPLICIT_DOCUMENT_START_STATE
	}
	*event = Event{
		Type:      DOCUMENT_END_EVENT,
		StartMark: start_mark,
		EndMark:   end_mark,
		Implicit:  implicit,
	}
	parser.setEventComments(event)
	if len(event.HeadComment) > 0 && len(event.FootComment) == 0 {
		event.FootComment = event.HeadComment
		event.HeadComment = nil
	}
	return nil
}

// Parse directives.
func (parser *Parser) processDirectives(version_directive_ref **VersionDirective, tag_directives_ref *[]TagDirective) error {
	var version_directive *VersionDirective
	var tag_directives []TagDirective

	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}

	for token.Type == VERSION_DIRECTIVE_TOKEN || token.Type == TAG_DIRECTIVE_TOKEN {
		switch token.Type {
		case VERSION_DIRECTIVE_TOKEN:
			if version_directive != nil {
				return parser.formatParserError(
					"found duplicate %YAML directive", token.StartMark)
			}
			if token.major != 1 || token.minor != 1 {
				return parser.formatParserError(
					"found incompatible YAML document", token.StartMark)
			}
			version_directive = &VersionDirective{
				major: token.major,
				minor: token.minor,
			}
		case TAG_DIRECTIVE_TOKEN:
			value := TagDirective{
				handle: token.Value,
				prefix: token.prefix,
			}
			if err := parser.appendTagDirective(value, false, token.StartMark); err != nil {
				return err
			}
			tag_directives = append(tag_directives, value)
		}

		parser.skipToken()
		if err := parser.peekToken(&token); err != nil {
			return err
		}
	}

	for i := range default_tag_directives {
		if err := parser.appendTagDirective(default_tag_directives[i], true, token.StartMark); err != nil {
			return err
		}
	}

	if version_directive_ref != nil {
		*version_directive_ref = version_directive
	}
	if tag_directives_ref != nil {
		*tag_directives_ref = tag_directives
	}
	return nil
}

// Append a tag directive to the directives stack.
func (parser *Parser) appendTagDirective(value TagDirective, allow_duplicates bool, mark Mark) error {
	for i := range parser.tag_directives {
		if bytes.Equal(value.handle, parser.tag_directives[i].handle) {
			if allow_duplicates {
				return nil
			}
			return parser.formatParserError("found duplicate %TAG directive", mark)
		}
	}

	// [Go] I suspect the copy is unnecessary. This was likely done
	// because there was no way to track ownership of the data.
	value_copy := TagDirective{
		handle: make([]byte, len(value.handle)),
		prefix: make([]byte, len(value.prefix)),
	}
	copy(value_copy.handle, value.handle)
	copy(value_copy.prefix, value.prefix)
	parser.tag_directives = append(parser.tag_directives, value_copy)
	return nil
}

// Parse the productions:
//
//	block_node_or_indentless_sequence    ::=
//	                         ALIAS
//	                         *****
//	                         | properties (block_content | indentless_block_sequence)?
//	                           **********  *
//	                         | block_content | indentless_block_sequence
//	                           *
//	block_node           ::= ALIAS
//	                         *****
//	                         | properties block_content?
//	                           ********** *
//	                         | block_content
//	                           *
//	flow_node            ::= ALIAS
//	                         *****
//	                         | properties flow_content?
//	                           ********** *
//	                         | flow_content
//	                           *
//	properties           ::= TAG ANCHOR? | ANCHOR TAG?
//	                         *************************
//	block_content        ::= block_collection | flow_collection | SCALAR
//	                                                              ******
//	flow_content         ::= flow_collection | SCALAR
//	                                           ******
func (parser *Parser) parseNode(event *Event, block, indentless_sequence bool) error {
	// defer trace("yaml_parser_parse_node", "block:", block, "indentless_sequence:", indentless_sequence)()

	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}

	if token.Type == ALIAS_TOKEN {
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]
		*event = Event{
			Type:      ALIAS_EVENT,
			StartMark: token.StartMark,
			EndMark:   token.EndMark,
			Anchor:    token.Value,
		}
		parser.setEventComments(event)
		parser.skipToken()
		return nil
	}

	start_mark := token.StartMark
	end_mark := token.StartMark

	var tag_token bool
	var tag_handle, tag_suffix, anchor []byte
	var tag_mark Mark
	switch token.Type {
	case ANCHOR_TOKEN:
		anchor = token.Value
		start_mark = token.StartMark
		end_mark = token.EndMark
		parser.skipToken()
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		if token.Type == TAG_TOKEN {
			tag_token = true
			tag_handle = token.Value
			tag_suffix = token.suffix
			tag_mark = token.StartMark
			end_mark = token.EndMark
			parser.skipToken()
			if err := parser.peekToken(&token); err != nil {
				return err
			}
		}
	case TAG_TOKEN:
		tag_token = true
		tag_handle = token.Value
		tag_suffix = token.suffix
		start_mark = token.StartMark
		tag_mark = token.StartMark
		end_mark = token.EndMark
		parser.skipToken()
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		if token.Type == ANCHOR_TOKEN {
			anchor = token.Value
			end_mark = token.EndMark
			parser.skipToken()
			if err := parser.peekToken(&token); err != nil {
				return err
			}
		}
	}

	var tag []byte
	if tag_token {
		if len(tag_handle) == 0 {
			tag = tag_suffix
		} else {
			for i := range parser.tag_directives {
				if bytes.Equal(parser.tag_directives[i].handle, tag_handle) {
					tag = append([]byte(nil), parser.tag_directives[i].prefix...)
					tag = append(tag, tag_suffix...)
					break
				}
			}
			if len(tag) == 0 {
				return parser.formatParserErrorContext(
					"while parsing a node", start_mark,
					"found undefined tag handle", tag_mark)
			}
		}
	}

	implicit := len(tag) == 0
	if indentless_sequence && token.Type == BLOCK_ENTRY_TOKEN {
		end_mark = token.EndMark
		parser.state = PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE
		*event = Event{
			Type:      SEQUENCE_START_EVENT,
			StartMark: start_mark,
			EndMark:   end_mark,
			Anchor:    anchor,
			Tag:       tag,
			Implicit:  implicit,
			Style:     Style(BLOCK_SEQUENCE_STYLE),
		}
		return nil
	}
	if token.Type == SCALAR_TOKEN {
		var plain_implicit, quoted_implicit bool
		end_mark = token.EndMark
		if (len(tag) == 0 && token.Style == PLAIN_SCALAR_STYLE) || (len(tag) == 1 && tag[0] == '!') {
			plain_implicit = true
		} else if len(tag) == 0 {
			quoted_implicit = true
		}
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]

		*event = Event{
			Type:            SCALAR_EVENT,
			StartMark:       start_mark,
			EndMark:         end_mark,
			Anchor:          anchor,
			Tag:             tag,
			Value:           token.Value,
			Implicit:        plain_implicit,
			quoted_implicit: quoted_implicit,
			Style:           Style(token.Style),
		}
		parser.setEventComments(event)
		parser.skipToken()
		return nil
	}
	if token.Type == FLOW_SEQUENCE_START_TOKEN {
		// [Go] Some of the events below can be merged as they differ only on style.
		end_mark = token.EndMark
		parser.state = PARSE_FLOW_SEQUENCE_FIRST_ENTRY_STATE
		*event = Event{
			Type:      SEQUENCE_START_EVENT,
			StartMark: start_mark,
			EndMark:   end_mark,
			Anchor:    anchor,
			Tag:       tag,
			Implicit:  implicit,
			Style:     Style(FLOW_SEQUENCE_STYLE),
		}
		parser.setEventComments(event)
		return nil
	}
	if token.Type == FLOW_MAPPING_START_TOKEN {
		end_mark = token.EndMark
		parser.state = PARSE_FLOW_MAPPING_FIRST_KEY_STATE
		*event = Event{
			Type:      MAPPING_START_EVENT,
			StartMark: start_mark,
			EndMark:   end_mark,
			Anchor:    anchor,
			Tag:       tag,
			Implicit:  implicit,
			Style:     Style(FLOW_MAPPING_STYLE),
		}
		parser.setEventComments(event)
		return nil
	}
	if block && token.Type == BLOCK_SEQUENCE_START_TOKEN {
		end_mark = token.EndMark
		parser.state = PARSE_BLOCK_SEQUENCE_FIRST_ENTRY_STATE
		*event = Event{
			Type:      SEQUENCE_START_EVENT,
			StartMark: start_mark,
			EndMark:   end_mark,
			Anchor:    anchor,
			Tag:       tag,
			Implicit:  implicit,
			Style:     Style(BLOCK_SEQUENCE_STYLE),
		}
		if parser.stem_comment != nil {
			event.HeadComment = parser.stem_comment
			parser.stem_comment = nil
		}
		return nil
	}
	if block && token.Type == BLOCK_MAPPING_START_TOKEN {
		end_mark = token.EndMark
		parser.state = PARSE_BLOCK_MAPPING_FIRST_KEY_STATE
		*event = Event{
			Type:      MAPPING_START_EVENT,
			StartMark: start_mark,
			EndMark:   end_mark,
			Anchor:    anchor,
			Tag:       tag,
			Implicit:  implicit,
			Style:     Style(BLOCK_MAPPING_STYLE),
		}
		if parser.stem_comment != nil {
			event.HeadComment = parser.stem_comment
			parser.stem_comment = nil
		}
		return nil
	}
	if len(anchor) > 0 || len(tag) > 0 {
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]

		*event = Event{
			Type:            SCALAR_EVENT,
			StartMark:       start_mark,
			EndMark:         end_mark,
			Anchor:          anchor,
			Tag:             tag,
			Implicit:        implicit,
			quoted_implicit: false,
			Style:           Style(PLAIN_SCALAR_STYLE),
		}
		return nil
	}

	context := "while parsing a flow node"
	if block {
		context = "while parsing a block node"
	}
	return parser.formatParserErrorContext(context, start_mark,
		"did not find expected node content", token.StartMark)
}

// Parse the productions:
//
//	block_sequence ::= BLOCK-SEQUENCE-START (BLOCK-ENTRY block_node?)* BLOCK-END
//	                   ********************  *********** *             *********
func (parser *Parser) parseBlockSequenceEntry(event *Event, first bool) error {
	if first {
		var token *Token
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		parser.marks = append(parser.marks, token.StartMark)
		parser.skipToken()
	}

	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}

	if token.Type == BLOCK_ENTRY_TOKEN {
		mark := token.EndMark
		prior_head_len := len(parser.HeadComment)
		parser.skipToken()
		if err := parser.splitStemComment(prior_head_len); err != nil {
			return err
		}
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		if token.Type != BLOCK_ENTRY_TOKEN && token.Type != BLOCK_END_TOKEN {
			parser.states = append(parser.states, PARSE_BLOCK_SEQUENCE_ENTRY_STATE)
			return parser.parseNode(event, true, false)
		} else {
			parser.state = PARSE_BLOCK_SEQUENCE_ENTRY_STATE
			return parser.processEmptyScalar(event, mark)
		}
	}
	if token.Type == BLOCK_END_TOKEN {
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]
		parser.marks = parser.marks[:len(parser.marks)-1]

		*event = Event{
			Type:      SEQUENCE_END_EVENT,
			StartMark: token.StartMark,
			EndMark:   token.EndMark,
		}

		parser.skipToken()
		return nil
	}

	context_mark := parser.marks[len(parser.marks)-1]
	parser.marks = parser.marks[:len(parser.marks)-1]
	return parser.formatParserErrorContext(
		"while parsing a block collection", context_mark,
		"did not find expected '-' indicator", token.StartMark)
}

// Parse the productions:
//
//	indentless_sequence  ::= (BLOCK-ENTRY block_node?)+
//	                          *********** *
func (parser *Parser) parseIndentlessSequenceEntry(event *Event) error {
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}

	if token.Type == BLOCK_ENTRY_TOKEN {
		mark := token.EndMark
		prior_head_len := len(parser.HeadComment)
		parser.skipToken()
		if err := parser.splitStemComment(prior_head_len); err != nil {
			return err
		}
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		if token.Type != BLOCK_ENTRY_TOKEN &&
			token.Type != KEY_TOKEN &&
			token.Type != VALUE_TOKEN &&
			token.Type != BLOCK_END_TOKEN {
			parser.states = append(parser.states, PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE)
			return parser.parseNode(event, true, false)
		}
		parser.state = PARSE_INDENTLESS_SEQUENCE_ENTRY_STATE
		return parser.processEmptyScalar(event, mark)
	}
	parser.state = parser.states[len(parser.states)-1]
	parser.states = parser.states[:len(parser.states)-1]

	*event = Event{
		Type:      SEQUENCE_END_EVENT,
		StartMark: token.StartMark,
		EndMark:   token.StartMark, // [Go] Shouldn't this be token.end_mark?
	}
	return nil
}

// Split stem comment from head comment.
//
// When a sequence or map is found under a sequence entry, the former head
// comment is assigned to the underlying sequence or map as a whole, not the
// individual sequence or map entry as would be expected otherwise.
// To handle this case the previous head comment is moved aside as the stem
// comment.
func (parser *Parser) splitStemComment(stem_len int) error {
	if stem_len == 0 {
		return nil
	}

	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}
	if token.Type != BLOCK_SEQUENCE_START_TOKEN && token.Type != BLOCK_MAPPING_START_TOKEN {
		return nil
	}

	parser.stem_comment = parser.HeadComment[:stem_len]
	if len(parser.HeadComment) == stem_len {
		parser.HeadComment = nil
	} else {
		// Copy suffix to prevent very strange bugs if someone ever appends
		// further bytes to the prefix in the stem_comment slice above.
		parser.HeadComment = append([]byte(nil), parser.HeadComment[stem_len+1:]...)
	}
	return nil
}

// Parse the productions:
//
//	block_mapping        ::= BLOCK-MAPPING_START
//	                         *******************
//	                         ((KEY block_node_or_indentless_sequence?)?
//	                           *** *
//	                         (VALUE block_node_or_indentless_sequence?)?)*
//
//	                         BLOCK-END
//	                         *********
func (parser *Parser) parseBlockMappingKey(event *Event, first bool) error {
	if first {
		var token *Token
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		parser.marks = append(parser.marks, token.StartMark)
		parser.skipToken()
	}

	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}

	// [Go] A tail comment was left from the prior mapping value processed.
	// Emit an event as it needs to be processed with that value and not
	// the following key.
	if len(parser.tail_comment) > 0 {
		*event = Event{
			Type:        TAIL_COMMENT_EVENT,
			StartMark:   token.StartMark,
			EndMark:     token.EndMark,
			FootComment: parser.tail_comment,
		}
		parser.tail_comment = nil
		return nil
	}

	switch token.Type {
	case KEY_TOKEN:
		mark := token.EndMark
		parser.skipToken()
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		if token.Type != KEY_TOKEN &&
			token.Type != VALUE_TOKEN &&
			token.Type != BLOCK_END_TOKEN {
			parser.states = append(parser.states, PARSE_BLOCK_MAPPING_VALUE_STATE)
			return parser.parseNode(event, true, true)
		} else {
			parser.state = PARSE_BLOCK_MAPPING_VALUE_STATE
			return parser.processEmptyScalar(event, mark)
		}
	case VALUE_TOKEN:
		parser.state = PARSE_BLOCK_MAPPING_VALUE_STATE
		return parser.processEmptyScalar(event, token.StartMark)
	case BLOCK_END_TOKEN:
		parser.state = parser.states[len(parser.states)-1]
		parser.states = parser.states[:len(parser.states)-1]
		parser.marks = parser.marks[:len(parser.marks)-1]
		*event = Event{
			Type:      MAPPING_END_EVENT,
			StartMark: token.StartMark,
			EndMark:   token.EndMark,
		}
		parser.setEventComments(event)
		parser.skipToken()
		return nil
	}

	context_mark := parser.marks[len(parser.marks)-1]
	parser.marks = parser.marks[:len(parser.marks)-1]
	return parser.formatParserErrorContext(
		"while parsing a block mapping", context_mark,
		"did not find expected key", token.StartMark)
}

// Parse the productions:
//
//	block_mapping        ::= BLOCK-MAPPING_START
//
//	                          ((KEY block_node_or_indentless_sequence?)?
//
//	                          (VALUE block_node_or_indentless_sequence?)?)*
//	                           ***** *
//	                          BLOCK-END
func (parser *Parser) parseBlockMappingValue(event *Event) error {
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}
	if token.Type == VALUE_TOKEN {
		mark := token.EndMark
		parser.skipToken()
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		if token.Type != KEY_TOKEN &&
			token.Type != VALUE_TOKEN &&
			token.Type != BLOCK_END_TOKEN {
			parser.states = append(parser.states, PARSE_BLOCK_MAPPING_KEY_STATE)
			return parser.parseNode(event, true, true)
		}
		parser.state = PARSE_BLOCK_MAPPING_KEY_STATE
		return parser.processEmptyScalar(event, mark)
	}
	parser.state = PARSE_BLOCK_MAPPING_KEY_STATE
	return parser.processEmptyScalar(event, token.StartMark)
}

// Parse the productions:
//
//	flow_sequence        ::= FLOW-SEQUENCE-START
//	                         *******************
//	                         (flow_sequence_entry FLOW-ENTRY)*
//	                          *                   **********
//	                         flow_sequence_entry?
//	                         *
//	                         FLOW-SEQUENCE-END
//	                         *****************
//	flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//	                         *
func (parser *Parser) parseFlowSequenceEntry(event *Event, first bool) error {
	if first {
		var token *Token
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		parser.marks = append(parser.marks, token.StartMark)
		parser.skipToken()
	}
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}
	if token.Type != FLOW_SEQUENCE_END_TOKEN {
		if !first {
			if token.Type == FLOW_ENTRY_TOKEN {
				parser.skipToken()
				if err := parser.peekToken(&token); err != nil {
					return err
				}
			} else {
				context_mark := parser.marks[len(parser.marks)-1]
				parser.marks = parser.marks[:len(parser.marks)-1]
				return parser.formatParserErrorContext(
					"while parsing a flow sequence", context_mark,
					"did not find expected ',' or ']'", token.StartMark)
			}
		}

		if token.Type == KEY_TOKEN || token.Type == VALUE_TOKEN {
			parser.state = PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_KEY_STATE
			*event = Event{
				Type:      MAPPING_START_EVENT,
				StartMark: token.StartMark,
				EndMark:   token.EndMark,
				Implicit:  true,
				Style:     Style(FLOW_MAPPING_STYLE),
			}
			if token.Type == KEY_TOKEN {
				parser.skipToken()
			}
			return nil
		} else if token.Type != FLOW_SEQUENCE_END_TOKEN {
			parser.states = append(parser.states, PARSE_FLOW_SEQUENCE_ENTRY_STATE)
			return parser.parseNode(event, false, false)
		}
	}

	parser.state = parser.states[len(parser.states)-1]
	parser.states = parser.states[:len(parser.states)-1]
	parser.marks = parser.marks[:len(parser.marks)-1]

	*event = Event{
		Type:      SEQUENCE_END_EVENT,
		StartMark: token.StartMark,
		EndMark:   token.EndMark,
	}
	parser.setEventComments(event)

	parser.skipToken()
	return nil
}

// Parse the productions:
//
//	flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//	                                     *** *
func (parser *Parser) parseFlowSequenceEntryMappingKey(event *Event) error {
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}
	if token.Type != VALUE_TOKEN &&
		token.Type != FLOW_ENTRY_TOKEN &&
		token.Type != FLOW_SEQUENCE_END_TOKEN {
		parser.states = append(parser.states, PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE)
		return parser.parseNode(event, false, false)
	}
	parser.state = PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_VALUE_STATE
	return parser.processEmptyScalar(event, token.StartMark)
}

// Parse the productions:
//
//	flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//	                                                     ***** *
func (parser *Parser) parseFlowSequenceEntryMappingValue(event *Event) error {
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}
	if token.Type == VALUE_TOKEN {
		parser.skipToken()
		var token *Token
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		if token.Type != FLOW_ENTRY_TOKEN && token.Type != FLOW_SEQUENCE_END_TOKEN {
			parser.states = append(parser.states, PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE)
			return parser.parseNode(event, false, false)
		}
	}
	parser.state = PARSE_FLOW_SEQUENCE_ENTRY_MAPPING_END_STATE
	return parser.processEmptyScalar(event, token.StartMark)
}

// Parse the productions:
//
//	flow_sequence_entry  ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//	                                                                     *
func (parser *Parser) parseFlowSequenceEntryMappingEnd(event *Event) error {
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}
	parser.state = PARSE_FLOW_SEQUENCE_ENTRY_STATE
	*event = Event{
		Type:      MAPPING_END_EVENT,
		StartMark: token.StartMark,
		EndMark:   token.StartMark, // [Go] Shouldn't this be end_mark?
	}
	return nil
}

// Parse the productions:
//
//	flow_mapping         ::= FLOW-MAPPING-START
//	                         ******************
//	                         (flow_mapping_entry FLOW-ENTRY)*
//	                          *                  **********
//	                         flow_mapping_entry?
//	                         ******************
//	                         FLOW-MAPPING-END
//	                         ****************
//	flow_mapping_entry   ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//	                         *           *** *
func (parser *Parser) parseFlowMappingKey(event *Event, first bool) error {
	if first {
		var token *Token
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		parser.marks = append(parser.marks, token.StartMark)
		parser.skipToken()
	}

	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}

	if token.Type != FLOW_MAPPING_END_TOKEN {
		if !first {
			if token.Type == FLOW_ENTRY_TOKEN {
				parser.skipToken()
				if err := parser.peekToken(&token); err != nil {
					return err
				}
			} else {
				context_mark := parser.marks[len(parser.marks)-1]
				parser.marks = parser.marks[:len(parser.marks)-1]
				return parser.formatParserErrorContext(
					"while parsing a flow mapping", context_mark,
					"did not find expected ',' or '}'", token.StartMark)
			}
		}

		if token.Type == VALUE_TOKEN {
			parser.state = PARSE_FLOW_MAPPING_VALUE_STATE
			return parser.processEmptyScalar(event, token.StartMark)
		}
		if token.Type == KEY_TOKEN {
			parser.skipToken()
			if err := parser.peekToken(&token); err != nil {
				return err
			}
			if token.Type != VALUE_TOKEN &&
				token.Type != FLOW_ENTRY_TOKEN &&
				token.Type != FLOW_MAPPING_END_TOKEN {
				parser.states = append(parser.states, PARSE_FLOW_MAPPING_VALUE_STATE)
				return parser.parseNode(event, false, false)
			} else {
				parser.state = PARSE_FLOW_MAPPING_VALUE_STATE
				return parser.processEmptyScalar(event, token.StartMark)
			}
		} else if token.Type != FLOW_MAPPING_END_TOKEN {
			parser.states = append(parser.states, PARSE_FLOW_MAPPING_EMPTY_VALUE_STATE)
			return parser.parseNode(event, false, false)
		}
	}

	parser.state = parser.states[len(parser.states)-1]
	parser.states = parser.states[:len(parser.states)-1]
	parser.marks = parser.marks[:len(parser.marks)-1]
	*event = Event{
		Type:      MAPPING_END_EVENT,
		StartMark: token.StartMark,
		EndMark:   token.EndMark,
	}
	parser.setEventComments(event)
	parser.skipToken()
	return nil
}

// Parse the productions:
//
//	flow_mapping_entry   ::= flow_node | KEY flow_node? (VALUE flow_node?)?
//	                                  *                  ***** *
func (parser *Parser) parseFlowMappingValue(event *Event, empty bool) error {
	var token *Token
	if err := parser.peekToken(&token); err != nil {
		return err
	}
	// A key without a KEY token may still have a value when the ':' is on
	// a later line, or the key itself spans multiple lines.
	if empty && token.Type != VALUE_TOKEN {
		parser.state = PARSE_FLOW_MAPPING_KEY_STATE
		return parser.processEmptyScalar(event, token.StartMark)
	}
	if token.Type == VALUE_TOKEN {
		parser.skipToken()
		if err := parser.peekToken(&token); err != nil {
			return err
		}
		if token.Type != FLOW_ENTRY_TOKEN && token.Type != FLOW_MAPPING_END_TOKEN {
			parser.states = append(parser.states, PARSE_FLOW_MAPPING_KEY_STATE)
			return parser.parseNode(event, false, false)
		}
	}
	parser.state = PARSE_FLOW_MAPPING_KEY_STATE
	return parser.processEmptyScalar(event, token.StartMark)
}

// Peek the next token in the token queue.
func (parser *Parser) peekToken(out **Token) error {
	if !parser.token_available {
		if err := parser.fetchMoreTokens(); err != nil {
			return err
		}
	}

	token := &parser.tokens[parser.tokens_head]
	parser.UnfoldComments(token)
	*out = token
	return nil
}

// UnfoldComments walks through the comments queue and joins all
// comments behind the position of the provided token into the respective
// top-level comment slices in the parser.
func (parser *Parser) UnfoldComments(token *Token) {
	for parser.comments_head < len(parser.comments) && token.StartMark.Index >= parser.comments[parser.comments_head].TokenMark.Index {
		comment := &parser.comments[parser.comments_head]
		if len(comment.Head) > 0 {
			if token.Type == BLOCK_END_TOKEN {
				// No heads on ends, so keep comment.Head for a follow up token.
				break
			}
			if len(parser.HeadComment) > 0 {
				parser.HeadComment = append(parser.HeadComment, '\n')
			}
			parser.HeadComment = append(parser.HeadComment, comment.Head...)
		}
		if len(comment.Foot) > 0 {
			if len(parser.FootComment) > 0 {
				parser.FootComment = append(parser.FootComment, '\n')
			}
			parser.FootComment = append(parser.FootComment, comment.Foot...)
		}
		if len(comment.Line) > 0 {
			if len(parser.LineComment) > 0 {
				parser.LineComment = append(parser.LineComment, '\n')
			}
			parser.LineComment = append(parser.LineComment, comment.Line...)
		}
		*comment = Comment{}
		parser.comments_head++
	}
}

// Remove the next token from the queue (must be called after peek_token).
func (parser *Parser) skipToken() {
	parser.token_available = false
	parser.tokens_parsed++
	parser.stream_end_produced = parser.tokens[parser.tokens_head].Type == STREAM_END_TOKEN
	parser.tokens_head++
}

// formatParserError creates a LoadError with the given problem message
// and mark position.
func (parser *Parser) formatParserError(problem string, problemMark Mark) *LoadError {
	return &LoadError{
		Stage:     ParserStage,
		Mark:      problemMark,
		Message:   problem,
		formatter: parser.formatError,
	}
}

// formatParserErrorContext creates a LoadError with both context and
// problem information, each with their own mark positions.
func (parser *Parser) formatParserErrorContext(context string, contextMark Mark, problem string, problemMark Mark) *LoadError {
	return &LoadError{
		Stage:       ParserStage,
		ContextMark: contextMark,
		ContextMsg:  context,
		Mark:        problemMark,
		Message:     problem,
		formatter:   parser.formatError,
	}
}

// setEventComments transfers accumulated comments from the parser to the
// event and clears the parser's comment state.
func (parser *Parser) setEventComments(event *Event) {
	event.HeadComment = parser.HeadComment
	event.LineComment = parser.LineComment
	event.FootComment = parser.FootComment
	parser.HeadComment = nil
	parser.LineComment = nil
	parser.FootComment = nil
	parser.tail_comment = nil
	parser.stem_comment = nil
}

// Generate an empty scalar event.
func (parser *Parser) processEmptyScalar(event *Event, mark Mark) error {
	*event = Event{
		Type:      SCALAR_EVENT,
		StartMark: mark,
		EndMark:   mark,
		Value:     nil, // Empty
		Implicit:  true,
		Style:     Style(PLAIN_SCALAR_STYLE),
	}
	return nil
}

// ParserGetEvents parses the YAML input and returns the generated event stream.
func ParserGetEvents(in []byte) (string, error) {
	p := NewComposer(in, nil)
	defer p.Destroy()
	var events strings.Builder
	var event Event
	for {
		if err := p.Parser.Parse(&event); err != nil {
			return "", err
		}
		formatted := formatEvent(&event)
		events.WriteString(formatted)
		if event.Type == STREAM_END_EVENT {
			event.Delete()
			break
		}
		event.Delete()
		events.WriteByte('\n')
	}
	return events.String(), nil
}

func escapeScalarValue(b []byte) string {
	builder := strings.Builder{}
	builder.Grow(len(b))
	for _, c := range string(b) {
		switch c {
		case '\\':
			builder.WriteString(`\\`)
		case 0:
			builder.WriteString(`\0`)
		case '\b':
			builder.WriteString(`\b`)
		case '\n':
			builder.WriteString(`\n`)
		case '\r':
			builder.WriteString(`\r`)
		case '\t':
			builder.WriteString(`\t`)
		default:
			builder.WriteRune(c)
		}
	}
	return builder.String()
}

// formatEvent formats an event as a human-readable string for debugging
// and testing purposes.
func formatEvent(e *Event) string {
	var b strings.Builder
	switch e.Type {
	case STREAM_START_EVENT:
		b.WriteString("+STR")
	case STREAM_END_EVENT:
		b.WriteString("-STR")
	case DOCUMENT_START_EVENT:
		b.WriteString("+DOC")
		if !e.Implicit {
			b.WriteString(" ---")
		}
	case DOCUMENT_END_EVENT:
		b.WriteString("-DOC")
		if !e.Implicit {
			b.WriteString(" ...")
		}
	case ALIAS_EVENT:
		b.WriteString("=ALI *")
		b.Write(e.Anchor)
	case SCALAR_EVENT:
		b.WriteString("=VAL")
		if len(e.Anchor) > 0 {
			b.WriteString(" &")
			b.Write(e.Anchor)
		}
		if len(e.Tag) > 0 {
			b.WriteString(" <")
			b.Write(e.Tag)
			b.WriteString(">")
		}
		switch e.ScalarStyle() {
		case PLAIN_SCALAR_STYLE:
			b.WriteString(" :")
		case LITERAL_SCALAR_STYLE:
			b.WriteString(" |")
		case FOLDED_SCALAR_STYLE:
			b.WriteString(" >")
		case SINGLE_QUOTED_SCALAR_STYLE:
			b.WriteString(" '")
		case DOUBLE_QUOTED_SCALAR_STYLE:
			b.WriteString(` "`)
		}
		b.WriteString(escapeScalarValue(e.Value))

	case SEQUENCE_START_EVENT:
		b.WriteString("+SEQ")
		if e.SequenceStyle() == FLOW_SEQUENCE_STYLE {
			b.WriteString(" []")
		}
		if len(e.Anchor) > 0 {
			b.WriteString(" &")
			b.Write(e.Anchor)
		}
		if len(e.Tag) > 0 {
			b.WriteString(" <")
			b.Write(e.Tag)
			b.WriteString(">")
		}
	case SEQUENCE_END_EVENT:
		b.WriteString("-SEQ")
	case MAPPING_START_EVENT:
		b.WriteString("+MAP")
		if e.MappingStyle() == FLOW_MAPPING_STYLE {
			b.WriteString(" {}")
		}
		if len(e.Anchor) > 0 {
			b.WriteString(" &")
			b.Write(e.Anchor)
		}
		if len(e.Tag) > 0 {
			b.WriteString(" <")
			b.Write(e.Tag)
			b.WriteString(">")
		}
	case MAPPING_END_EVENT:
		b.WriteString("-MAP")
	}
	return b.String()
}
