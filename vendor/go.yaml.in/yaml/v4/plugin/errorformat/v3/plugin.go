// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Package errorformatv3 provides the go-yaml v3-compatible error formatter.
//
// The v3 formatter renders load errors as "yaml: line N: message" and dump
// errors as "yaml: message", matching the legacy go-yaml error style.
package errorformatv3

import (
	"fmt"

	"go.yaml.in/yaml/v4/internal/libyaml"
)

// Plugin implements the v3-compatible YAML error formatter.
type Plugin struct{}

// New creates a v3-compatible error formatting plugin.
func New() *Plugin {
	return &Plugin{}
}

// FormatLoadError implements [yaml.ErrorFormatPlugin].
func (p *Plugin) FormatLoadError(err *libyaml.LoadError) string {
	line := err.Mark.Line
	if line == 0 {
		return "yaml: " + err.Message
	}
	return fmt.Sprintf("yaml: line %d: %s", line, err.Message)
}

// FormatDumpError implements [yaml.ErrorFormatPlugin].
func (p *Plugin) FormatDumpError(err *libyaml.DumpError) string {
	return "yaml: " + err.Message
}

// NewFromYAML creates a v3 error formatting plugin from YAML config.
func NewFromYAML(cfg map[string]any) (*Plugin, error) {
	for key := range cfg {
		return nil, fmt.Errorf("error-format v3: unknown key %q", key)
	}
	return New(), nil
}
