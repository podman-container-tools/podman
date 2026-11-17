// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

// Package aliasdata supplies node-based YAML anchor and alias policies.
// New without options explicitly selects retention of stream anchors.
// Source options default to document scope; WithPath(true) enables stream scope.
// WithStream explicitly overrides either default.
// Without an explicit selection, go-yaml uses ordinary document scope.
package aliasdata

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v4/internal/libyaml"
)

// Plugin stores reusable configuration. Sources are read once per context.
// Callers must not mutate supplied data while a new context is being created.
type Plugin struct {
	data      map[string]any
	file      string
	env       any
	stream    bool
	streamSet bool
	path      bool
	disable   bool
}

// Option configures an Alias-Data plugin.
type Option func(*Plugin)

// WithData supplies named external values, converted to nodes per stream.
func WithData(data map[string]any) Option {
	return func(p *Plugin) { p.data = data }
}

// WithFile supplies a single mapping document of named external values.
func WithFile(path string) Option {
	return func(p *Plugin) { p.file = path }
}

// WithEnv selects environment values with a bool, string, or string sequence.
// Only * has wildcard meaning in a string selector.
func WithEnv(selector any) Option {
	return func(p *Plugin) { p.env = selector }
}

// WithStream controls retention of anchors across documents in one stream.
func WithStream(enabled bool) Option {
	return func(p *Plugin) {
		p.stream = enabled
		p.streamSet = true
	}
}

// WithPath controls dotted mapping and sequence lookups in alias names.
// Enabling paths enables stream mode unless WithStream overrides that default.
func WithPath(enabled bool) Option {
	return func(p *Plugin) { p.path = enabled }
}

// WithDisable rejects alias references while allowing anchor declarations.
// Disabled policies do not read or validate saved source configuration.
func WithDisable(disabled bool) Option {
	return func(p *Plugin) { p.disable = disabled }
}

var anchorName = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func validName(name string) error {
	if !anchorName.MatchString(name) {
		return fmt.Errorf("alias-data: name %q must match [A-Za-z0-9_-]+", name)
	}
	return nil
}

// New creates a reusable plugin. Empty explicit selection enables stream mode.
// Validation does not read files or environment values.
func New(options ...Option) (*Plugin, error) {
	p := &Plugin{stream: len(options) == 0}
	for _, option := range options {
		option(p)
	}
	if p.path && !p.streamSet {
		p.stream = true
	}
	if p.disable {
		return p, nil
	}
	for name := range p.data {
		if err := validName(name); err != nil {
			return nil, err
		}
	}
	if p.file != "" && strings.TrimSpace(p.file) == "" {
		return nil, errors.New("alias-data: file must be a non-empty string")
	}
	if _, err := selectEnvironment(p.env, nil, false); err != nil {
		return nil, err
	}
	return p, nil
}

// NewFromYAML validates configuration without reading any source data.
// Nonempty configurations default to document scope unless path is true.
// An explicit stream setting overrides that default, matching YAMLStar.
func NewFromYAML(config map[string]any) (*Plugin, error) {
	if disabled, ok := config["disable"].(bool); ok && disabled {
		return New(WithDisable(true))
	}
	var options []Option
	settings := 0
	for key := range config {
		if key != "disable" {
			settings++
		}
	}
	path, _ := config["path"].(bool)
	options = append(options, WithStream(settings == 0 || path))
	for key, value := range config {
		switch key {
		case "data":
			if value == nil {
				continue
			}
			data, ok := value.(map[string]any)
			if !ok {
				return nil, errors.New("alias-data: data must be a mapping of names")
			}
			options = append(options, WithData(data))
		case "file":
			path, ok := value.(string)
			if !ok || strings.TrimSpace(path) == "" {
				return nil, errors.New("alias-data: file must be a non-empty string")
			}
			options = append(options, WithFile(path))
		case "env":
			options = append(options, WithEnv(value))
		case "stream", "disable", "path":
			flag, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("alias-data: %s must be a boolean", key)
			}
			if key == "stream" {
				options = append(options, WithStream(flag))
			}
			if key == "path" {
				options = append(options, WithPath(flag))
			}
		default:
			return nil, fmt.Errorf("alias-data: unknown key %q", key)
		}
	}
	return New(options...)
}

// NewAliasDataContext snapshots configured sources for a fresh input stream.
func (p *Plugin) NewAliasDataContext(
	services libyaml.AliasDataServices,
) (libyaml.AliasDataContext, error) {
	sources := libyaml.AliasDataSources{
		Stream: p.stream, Path: p.path, Disable: p.disable,
	}
	if p.disable {
		return libyaml.NewAliasDataPolicy(sources), nil
	}
	sources.Data = map[string]*libyaml.Node{}
	for name, value := range p.data {
		if err := validName(name); err != nil {
			return nil, err
		}
		node, err := services.NewNode(value)
		if err != nil {
			return nil, fmt.Errorf("alias-data: data %q: %w", name, err)
		}
		sources.Data[name] = node
	}
	if p.file != "" {
		root, err := services.LoadFile(p.file)
		if err != nil {
			return nil, fmt.Errorf("alias-data: file %q: %w", p.file, err)
		}
		if root == nil || root.Kind != libyaml.MappingNode {
			return nil, errors.New("alias-data: file root must be a mapping")
		}
		sources.File = map[string]*libyaml.Node{}
		for i := 0; i < len(root.Content); i += 2 {
			key := root.Content[i]
			seen := map[*libyaml.Node]bool{}
			for key != nil && key.Kind == libyaml.AliasNode && !seen[key] {
				seen[key] = true
				key = key.Alias
			}
			if key == nil || key.Kind != libyaml.ScalarNode || key.ShortTag() != "!!str" {
				return nil, errors.New("alias-data: file keys must be strings")
			}
			if err := validName(key.Value); err != nil {
				return nil, err
			}
			if _, found := sources.File[key.Value]; found {
				return nil, fmt.Errorf("alias-data: duplicate file key %q", key.Value)
			}
			sources.File[key.Value] = root.Content[i+1]
		}
	}
	if p.env != nil && p.env != false {
		values, err := selectEnvironment(p.env, services.Environment(), true)
		if err != nil {
			return nil, err
		}
		sources.Env = map[string]*libyaml.Node{}
		for name, value := range values {
			node, err := services.NewNode(value)
			if err != nil {
				return nil, err
			}
			sources.Env[name] = node
		}
	}
	return libyaml.NewAliasDataPolicy(sources), nil
}

func selectEnvironment(
	selector any, values map[string]string, snapshot bool,
) (map[string]string, error) {
	selected := map[string]string{}
	selectName := func(name string) error {
		if err := validName(name); err != nil {
			return err
		}
		if snapshot {
			value, found := values[name]
			if !found {
				return fmt.Errorf("alias-data: environment variable %q is not set", name)
			}
			selected[name] = value
		}
		return nil
	}
	switch selector := selector.(type) {
	case nil:
	case bool:
		if selector && snapshot {
			selected = values
		}
	case string:
		if strings.Contains(selector, "*") {
			pattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(selector), `\*`, ".*") + "$"
			matcher := regexp.MustCompile(pattern)
			for name, value := range values {
				if matcher.MatchString(name) {
					selected[name] = value
				}
			}
		} else if err := selectName(selector); err != nil {
			return nil, err
		}
	case []string:
		for _, name := range selector {
			if err := selectName(name); err != nil {
				return nil, err
			}
		}
	case []any:
		for _, item := range selector {
			name, ok := item.(string)
			if !ok {
				return nil, errors.New("alias-data: env names must be strings")
			}
			if err := selectName(name); err != nil {
				return nil, err
			}
		}
	default:
		return nil, errors.New("alias-data: env must be a boolean, string, or sequence")
	}
	for name := range selected {
		if err := validName(name); err != nil {
			return nil, err
		}
	}
	return selected, nil
}
