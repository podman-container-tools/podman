// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Package errorformatv4 provides the go-yaml v4 error formatter.
//
// With no options, the v4 formatter renders the current structured go-yaml
// error style:
//
//	go-yaml load error in scanner at L2.C6: message
//	go-yaml dump error in representer: message
//
// It can also be configured with alternate load-error position styles.
package errorformatv4

import (
	"fmt"

	"go.yaml.in/yaml/v4/internal/libyaml"
)

const defaultLoadFormat = "go-yaml load error in %s%s at %s: %s"

const defaultDumpFormat = "go-yaml dump error in %s: %s"

// PositionStyle controls how load-error positions are rendered.
type PositionStyle int

const (
	// PositionShort renders positions as L2.C6.
	PositionShort PositionStyle = iota

	// PositionLong renders positions as line 2, column 6.
	PositionLong

	// PositionLine renders positions as line 2.
	PositionLine
)

// Plugin implements the v4 YAML error formatter.
type Plugin struct {
	positionStyle PositionStyle
}

// Option configures a [Plugin].
type Option func(*Plugin) error

// New creates a v4 error formatting plugin.
func New(opts ...Option) (*Plugin, error) {
	p := &Plugin{
		positionStyle: PositionShort,
	}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Must creates a v4 error formatting plugin and panics on invalid options.
func Must(opts ...Option) *Plugin {
	p, err := New(opts...)
	if err != nil {
		panic(err)
	}
	return p
}

// WithPositionStyle sets how load-error positions are rendered.
func WithPositionStyle(style PositionStyle) Option {
	return func(p *Plugin) error {
		switch style {
		case PositionShort, PositionLong, PositionLine:
			p.positionStyle = style
			return nil
		default:
			return fmt.Errorf("error-format v4: invalid position style %d", style)
		}
	}
}

// FormatLoadError implements [yaml.ErrorFormatPlugin].
func (p *Plugin) FormatLoadError(err *libyaml.LoadError) string {
	position := p.pos(err.Mark)
	context := ""
	if err.ContextMsg != "" {
		position = p.rangePos(err.ContextMark, err.Mark)
		context = " (" + err.ContextMsg + ")"
	}
	return fmt.Sprintf(defaultLoadFormat, err.Stage, context, position, err.Message)
}

// FormatDumpError implements [yaml.ErrorFormatPlugin].
func (p *Plugin) FormatDumpError(err *libyaml.DumpError) string {
	return fmt.Sprintf(defaultDumpFormat, err.Stage, err.Message)
}

// NewFromYAML creates a v4 error formatting plugin from YAML config.
func NewFromYAML(cfg map[string]any) (*Plugin, error) {
	var opts []Option
	for key, val := range cfg {
		switch key {
		case "position":
			s, ok := val.(string)
			if !ok {
				return nil, fmt.Errorf("error-format v4: position must be a string, got %T", val)
			}
			style, err := parsePositionStyle(s)
			if err != nil {
				return nil, err
			}
			opts = append(opts, WithPositionStyle(style))
		default:
			return nil, fmt.Errorf("error-format v4: unknown key %q", key)
		}
	}
	return New(opts...)
}

func parsePositionStyle(s string) (PositionStyle, error) {
	switch s {
	case "short":
		return PositionShort, nil
	case "long":
		return PositionLong, nil
	case "line":
		return PositionLine, nil
	default:
		return 0, fmt.Errorf("error-format v4: invalid position %q (use short, long, or line)", s)
	}
}

func (p *Plugin) pos(mark libyaml.Mark) string {
	switch p.positionStyle {
	case PositionLong:
		return longPos(mark)
	case PositionLine:
		return line(mark)
	default:
		return mark.ShortString()
	}
}

func (p *Plugin) rangePos(start, end libyaml.Mark) string {
	switch p.positionStyle {
	case PositionLong:
		return longRange(start, end)
	case PositionLine:
		if start.Line == 0 {
			return line(end)
		}
		if end.Line == 0 || start.Line == end.Line {
			return line(start)
		}
		return fmt.Sprintf("line %d-line %d", start.Line, end.Line)
	default:
		return start.RangeString(end)
	}
}

func line(mark libyaml.Mark) string {
	if mark.Line == 0 {
		return "<unknown position>"
	}
	return fmt.Sprintf("line %d", mark.Line)
}

func longPos(mark libyaml.Mark) string {
	if mark.Line == 0 {
		return "<unknown position>"
	}
	if mark.Column == 0 {
		return fmt.Sprintf("line %d", mark.Line)
	}
	return fmt.Sprintf("line %d, column %d", mark.Line, mark.Column)
}

func longRange(start, end libyaml.Mark) string {
	if start.Line == 0 {
		return longPos(end)
	}
	if end.Line == 0 || start == end {
		return longPos(start)
	}
	if start.Line == end.Line {
		if start.Column == 0 || end.Column == 0 || start.Column == end.Column {
			return fmt.Sprintf("line %d", start.Line)
		}
		return fmt.Sprintf("line %d, columns %d-%d", start.Line, start.Column, end.Column)
	}
	return fmt.Sprintf("%s-%s", longPos(start), longPos(end))
}
