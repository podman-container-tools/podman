// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

package libyaml

import (
	"errors"
	"fmt"
	"io"
)

// ParserPlugin supplies a complete event stream for a load operation.
// Parse returns a complete stream, including stream and document boundaries.
// Implementations must support concurrent calls with independent input.
type ParserPlugin interface {
	Parse(input []byte) ([]PluginEvent, error)
}

// IndentMode supplies loading and dumping defaults.
type IndentMode string

const (
	IndentModeAuto IndentMode = "auto"
	IndentModeTabs IndentMode = "tabs"
)

// IndentStyle identifies the characters used for structural indentation.
type IndentStyle string

const (
	IndentStyleAuto   IndentStyle = "auto"
	IndentStyleSpaces IndentStyle = "spaces"
	IndentStyleTabs   IndentStyle = "tabs"
)

// IndentScope controls how long an auto-detected style remains active.
type IndentScope string

const (
	IndentScopeDocument IndentScope = "document"
	IndentScopeStream   IndentScope = "stream"
)

// IndentConfig configures tab-indentation loading and dumping.
type IndentConfig struct {
	Mode      IndentMode
	LoadStyle IndentStyle
	DumpStyle IndentStyle
	Scope     IndentScope
}

// Normalize fills defaults and validates a tab-indentation configuration.
func (c IndentConfig) Normalize() (IndentConfig, error) {
	switch c.Mode {
	case "":
		c.Mode = IndentModeAuto
	case IndentModeAuto, IndentModeTabs:
	default:
		return c, errors.New("yaml: tab-indent mode must be auto or tabs")
	}
	if c.LoadStyle == "" {
		if c.Mode == IndentModeTabs {
			c.LoadStyle = IndentStyleTabs
		} else {
			c.LoadStyle = IndentStyleAuto
		}
	}
	if c.DumpStyle == "" {
		c.DumpStyle = IndentStyleTabs
	}
	if c.Scope == "" {
		c.Scope = IndentScopeDocument
	}
	switch c.LoadStyle {
	case IndentStyleAuto, IndentStyleSpaces, IndentStyleTabs:
	default:
		return c, errors.New("yaml: tab-indent load must be auto, spaces, or tabs")
	}
	switch c.DumpStyle {
	case IndentStyleSpaces, IndentStyleTabs:
	default:
		return c, errors.New("yaml: tab-indent dump must be spaces or tabs")
	}
	switch c.Scope {
	case IndentScopeDocument, IndentScopeStream:
	default:
		return c, errors.New("yaml: tab-indent auto must be document or stream")
	}
	return c, nil
}

// TabIndentPlugin enables tab-aware structural indentation.
type TabIndentPlugin interface {
	TabIndentConfig() IndentConfig
}

// hasInputPlugins reports whether input must pass through EventReader.
// Parser plugins replace native parsing.
func hasInputPlugins(opts *Options) bool {
	return opts != nil && opts.Parser != nil
}

// PluginEvent is a portable YAML event supplied by a plugin.
// Type is stream_start, stream_end, document_start, document_end,
// mapping_start, mapping_end, sequence_start, sequence_end, scalar, or alias.
// Style is empty (plain), single, double, literal, or folded for scalars.
// Alias events use Anchor for the referenced name.
// Tags are resolved URIs or local tags; an empty tag requests resolution.
// Positions are one-based, with zero meaning unknown.
type PluginEvent struct {
	Type               string
	Value, Anchor, Tag string
	Style              string
	Flow, Explicit     bool
	StartMark, EndMark Mark
	Version            *StreamVersionDirective
	TagDirectives      []StreamTagDirective
	HeadComment        string
	LineComment        string
	FootComment        string
}

// EventReader reads native or plugin events using the same loading options.
// Plugin input is buffered on the first Parse call.
type EventReader struct {
	parser       Parser
	reader       io.Reader
	opts         *Options
	events       []PluginEvent
	index        int
	initialized  bool
	pluginEvents bool
	err          error
}

// NewEventReader creates an event reader for the configured input pipeline.
func NewEventReader(r io.Reader, opts *Options) *EventReader {
	return &EventReader{reader: r, opts: opts}
}

// Delete releases buffered input and native parser resources.
func (e *EventReader) Delete() {
	e.parser.Delete()
	e.reader = nil
	e.events = nil
}

// Parse returns the next event, or [io.EOF] after the stream end.
func (e *EventReader) Parse(event *Event) error {
	if !e.initialized {
		e.initialize()
	}
	if e.err != nil {
		return e.err
	}
	if !e.pluginEvents {
		return e.parser.Parse(event)
	}
	if e.index == len(e.events) {
		return io.EOF
	}
	*event = pluginEvent(e.events[e.index])
	e.index++
	return nil
}

// initialize selects the native or plugin parsing pipeline on first use.
func (e *EventReader) initialize() {
	e.initialized = true
	if !hasInputPlugins(e.opts) {
		e.parser = NewParser()
		e.parser.SetInputReader(e.reader)
		e.reader = nil
		if e.opts != nil {
			e.parser.depthCheck = e.opts.DepthCheck
			e.parser.formatError = e.opts.FormatLoadError
			e.parser.indentConfig = e.opts.IndentConfig
		}
		return
	}

	input, err := io.ReadAll(e.reader)
	e.reader = nil
	if err != nil {
		e.err = formatLoadError(NewLoadError(ReaderStage, err.Error(), Mark{}, err), e.opts.FormatLoadError)
		return
	}
	e.pluginEvents = true
	e.events, err = e.opts.Parser.Parse(input)
	if err == nil {
		err = validatePluginEvents(e.events, e.opts.DepthCheck)
	}
	if err != nil {
		e.err = formatLoadError(NewLoadError(ParserStage, err.Error(), Mark{}, err), e.opts.FormatLoadError)
	}
}

// validatePluginEvents checks structure before the recursive composer sees it.
func validatePluginEvents(events []PluginEvent, depthCheck func(int, *DepthContext) error) error {
	type frame struct {
		kind     string
		children int
	}
	var stack []frame
	started, ended := false, false
	for i, e := range events {
		bad := func() error {
			return fmt.Errorf("invalid parser event %d (%s)", i, e.Type)
		}
		if ended || (!started && e.Type != "stream_start") {
			return bad()
		}
		switch e.Type {
		case "stream_start":
			if started {
				return bad()
			}
			started = true
		case "stream_end":
			if len(stack) != 0 {
				return bad()
			}
			ended = true
		case "document_start":
			if len(stack) != 0 {
				return bad()
			}
			stack = append(stack, frame{kind: e.Type})
		case "document_end", "mapping_end", "sequence_end":
			if len(stack) == 0 {
				return bad()
			}
			f := stack[len(stack)-1]
			if (e.Type == "document_end" && (f.kind != "document_start" || f.children != 1)) ||
				(e.Type == "mapping_end" && (f.kind != "mapping_start" || f.children%2 != 0)) ||
				(e.Type == "sequence_end" && f.kind != "sequence_start") {
				return bad()
			}
			stack = stack[:len(stack)-1]
		case "scalar", "alias", "mapping_start", "sequence_start":
			if len(stack) == 0 {
				return bad()
			}
			f := &stack[len(stack)-1]
			f.children++
			if f.kind == "document_start" && f.children > 1 {
				return bad()
			}
			if e.Type == "alias" && e.Anchor == "" {
				return bad()
			}
			if e.Type == "scalar" {
				switch e.Style {
				case "", "single", "double", "literal", "folded":
				default:
					return bad()
				}
			}
			if e.Type == "mapping_start" || e.Type == "sequence_start" {
				check := depthCheck
				if check == nil {
					check = DefaultDepthCheck
				}
				kind := DepthKindBlock
				if e.Flow {
					kind = DepthKindFlow
				}
				if err := check(len(stack), &DepthContext{Kind: kind}); err != nil {
					return err
				}
				stack = append(stack, frame{kind: e.Type})
			}
		default:
			return bad()
		}
		if e.Version != nil && (e.Type != "document_start" ||
			e.Version.Major != 1 || (e.Version.Minor != 1 && e.Version.Minor != 2)) {
			return fmt.Errorf("unsupported parser version directive at event %d", i)
		}
	}
	if !ended {
		return errors.New("parser stream is missing stream_end")
	}
	return nil
}

func pluginEvent(p PluginEvent) Event {
	e := Event{
		StartMark: p.StartMark, EndMark: p.EndMark,
		Value: []byte(p.Value), Tag: []byte(p.Tag),
		HeadComment: []byte(p.HeadComment), LineComment: []byte(p.LineComment),
		FootComment: []byte(p.FootComment), Implicit: !p.Explicit,
	}
	if p.Anchor != "" {
		e.Anchor = []byte(p.Anchor)
	}
	switch p.Type {
	case "stream_start":
		e.Type, e.encoding = STREAM_START_EVENT, UTF8_ENCODING
	case "stream_end":
		e.Type = STREAM_END_EVENT
	case "document_start":
		e.Type = DOCUMENT_START_EVENT
		if p.Version != nil {
			e.versionDirective = &VersionDirective{major: int8(p.Version.Major), minor: int8(p.Version.Minor)}
		}
		for _, td := range p.TagDirectives {
			e.tagDirectives = append(e.tagDirectives, TagDirective{handle: []byte(td.Handle), prefix: []byte(td.Prefix)})
		}
	case "document_end":
		e.Type = DOCUMENT_END_EVENT
	case "mapping_start":
		e.Type, e.Style = MAPPING_START_EVENT, Style(BLOCK_MAPPING_STYLE)
		if p.Flow {
			e.Style = Style(FLOW_MAPPING_STYLE)
		}
	case "mapping_end":
		e.Type = MAPPING_END_EVENT
	case "sequence_start":
		e.Type, e.Style = SEQUENCE_START_EVENT, Style(BLOCK_SEQUENCE_STYLE)
		if p.Flow {
			e.Style = Style(FLOW_SEQUENCE_STYLE)
		}
	case "sequence_end":
		e.Type = SEQUENCE_END_EVENT
	case "alias":
		e.Type = ALIAS_EVENT
	case "scalar":
		e.Type = SCALAR_EVENT
		e.Implicit = (p.Tag == "" && p.Style == "") || p.Tag == "!"
		e.quoted_implicit = p.Tag == "" && p.Style != ""
		switch p.Style {
		case "":
			e.Style = Style(PLAIN_SCALAR_STYLE)
		case "single":
			e.Style = Style(SINGLE_QUOTED_SCALAR_STYLE)
		case "double":
			e.Style = Style(DOUBLE_QUOTED_SCALAR_STYLE)
		case "literal":
			e.Style = Style(LITERAL_SCALAR_STYLE)
		case "folded":
			e.Style = Style(FOLDED_SCALAR_STYLE)
		}
	}
	return e
}
