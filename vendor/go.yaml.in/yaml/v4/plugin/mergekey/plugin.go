// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Package mergekey supplies the standard shallow Merge-Key algorithm.
package mergekey

import (
	"errors"
	"fmt"

	"go.yaml.in/yaml/v4/internal/libyaml"
)

// Config controls merge recognition, repeated directives, and conflicts.
type Config = libyaml.MergeKeyConfig

// Plugin implements the standard Merge-Key algorithm.
type Plugin struct {
	libyaml.DefaultMergeKey
}

// Option configures a plugin.
type Option func(*Plugin)

// New creates a plugin. Invalid options are rejected by yaml.WithPlugin.
func New(opts ...Option) *Plugin {
	p := &Plugin{}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// WithDisable disables all merging when true.
func WithDisable(disable bool) Option {
	return func(p *Plugin) { p.Config.Disable = disable }
}

// WithMultiple selects error or combine for repeated merge directives.
func WithMultiple(policy string) Option {
	return func(p *Plugin) { p.Config.Multiple = policy }
}

// WithConflicts selects first, last, or error across target and sources.
func WithConflicts(policy string) Option {
	return func(p *Plugin) { p.Config.Conflicts = policy }
}

// NewFromYAML creates a plugin from a named implementation's configuration.
func NewFromYAML(cfg map[string]any) (*Plugin, error) {
	p := New()
	for key, value := range cfg {
		switch key {
		case "disable":
			if shorthand, ok := value.(map[string]any); ok {
				if len(shorthand) != 0 {
					return nil, errors.New("merge-key: disable shorthand must be empty")
				}
				p.Config.Disable = true
				continue
			}
			disabled, ok := value.(bool)
			if !ok {
				return nil, errors.New("merge-key: disable must be a boolean")
			}
			p.Config.Disable = disabled
		case "multiple", "conflicts":
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("merge-key: %s must be a string", key)
			}
			if key == "multiple" {
				p.Config.Multiple = text
			} else {
				p.Config.Conflicts = text
			}
		default:
			return nil, fmt.Errorf("merge-key: unknown key %q", key)
		}
	}
	return p, p.Config.Validate()
}
