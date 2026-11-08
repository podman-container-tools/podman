// Copyright 2026 The go-yaml Project Contributors
// SPDX-License-Identifier: Apache-2.0

package yaml

import (
	"errors"

	"go.yaml.in/yaml/v4/internal/libyaml"
	pluginreg "go.yaml.in/yaml/v4/internal/plugin"
	"go.yaml.in/yaml/v4/plugin/aliasdata"
	"go.yaml.in/yaml/v4/plugin/loaderlimits"
	"go.yaml.in/yaml/v4/plugin/mergekey"
)

// ErrorFormatPlugin customizes how YAML load and dump errors are rendered.
//
// When registered, FormatLoadError and FormatDumpError are called to produce
// the error message string for each [LoadError] and [DumpError], allowing the
// format to be customized.
//
// Example usage:
//
//	import errorformatv3 "go.yaml.in/yaml/v4/plugin/errorformat/v3"
//	loader, err := yaml.NewLoader(reader, yaml.WithPlugin(errorformatv3.New()))
type ErrorFormatPlugin interface {
	// FormatLoadError returns the string representation of a LoadError.
	// The returned string is used as the error message.
	FormatLoadError(err *LoadError) string

	// FormatDumpError returns the string representation of a DumpError.
	// The returned string is used as the error message.
	FormatDumpError(err *DumpError) string
}

// LoaderLimitsPlugin configures safety limits for YAML loading.
//
// When registered, CheckDepth is called on each nesting depth increase,
// and CheckAlias is called on each alias expansion to detect excessive
// aliasing.
//
// Example usage:
//
//	import "go.yaml.in/yaml/v4/plugin/loaderlimits"
//	loader, err := yaml.NewLoader(reader,
//	    yaml.WithPlugin(loaderlimits.New(loaderlimits.AliasNone())))
type LoaderLimitsPlugin interface {
	// CheckDepth is called when the parser increases nesting depth.
	// depth is the current nesting level; ctx.Kind is "flow" or "block".
	// Return an error to abort parsing.
	CheckDepth(depth int, ctx *DepthContext) error

	// CheckAlias is called during alias expansion.
	// Return an error to abort construction.
	CheckAlias(aliasCount, constructCount int) error
}

// AliasDataPlugin creates per-stream anchor and alias policies.
type AliasDataPlugin = libyaml.AliasDataPlugin

// AliasDataContext owns node-based anchor storage and alias lookup.
type AliasDataContext = libyaml.AliasDataContext

// AliasDataServices supplies source snapshots and node conversions.
type AliasDataServices = libyaml.AliasDataServices

// ParserPlugin supplies a complete event stream in place of native parsing.
type ParserPlugin = libyaml.ParserPlugin

// IndentMode supplies loading and dumping defaults.
type IndentMode = libyaml.IndentMode

const (
	IndentModeAuto = libyaml.IndentModeAuto
	IndentModeTabs = libyaml.IndentModeTabs
)

// IndentStyle identifies the characters used for structural indentation.
type IndentStyle = libyaml.IndentStyle

const (
	IndentStyleAuto   = libyaml.IndentStyleAuto
	IndentStyleSpaces = libyaml.IndentStyleSpaces
	IndentStyleTabs   = libyaml.IndentStyleTabs
)

// IndentScope controls how long an auto-detected style remains active.
type IndentScope = libyaml.IndentScope

const (
	IndentScopeDocument = libyaml.IndentScopeDocument
	IndentScopeStream   = libyaml.IndentScopeStream
)

// IndentConfig configures tab-indentation loading and dumping.
type IndentConfig = libyaml.IndentConfig

// TabIndentPlugin enables tab-aware structural indentation.
type TabIndentPlugin = libyaml.TabIndentPlugin

// MergeKeyConfig controls merge recognition, repeated directives, and conflicts.
type MergeKeyConfig = libyaml.MergeKeyConfig

// MergeKeyInput supplies read-only target and source mappings to an algorithm.
type MergeKeyInput = libyaml.MergeKeyInput

// MergeKeyPlugin chooses the effective mapping before value construction.
type MergeKeyPlugin = libyaml.MergeKeyPlugin

// PluginEvent carries source-independent YAML syntax information.
type PluginEvent = libyaml.PluginEvent

// PluginFactory constructs a plugin from its plugin-specific configuration.
type PluginFactory = pluginreg.Factory

// PluginRegistration describes one named implementation of a plugin API.
// Version is empty for an unversioned implementation.
// Default selects the implementation used by boolean configuration.
type PluginRegistration = pluginreg.Registration

type nativeParserPlugin struct{}

var pluginRegistry = pluginreg.NewRegistry(
	PluginRegistration{
		API: pluginreg.MergeKeyAPI, Name: pluginreg.MergeKeyAPI, Default: true,
		Factory: func(cfg map[string]any) (any, error) {
			return mergekey.NewFromYAML(cfg)
		},
	},
	PluginRegistration{
		API: pluginreg.AliasDataAPI, Name: pluginreg.AliasDataAPI,
		Default: true,
		Factory: func(cfg map[string]any) (any, error) {
			return aliasdata.NewFromYAML(cfg)
		},
	},
	PluginRegistration{
		API: pluginreg.ErrorFormatAPI, Name: pluginreg.ErrorFormatAPI,
		Default: true,
		Factory: newErrorFormatPluginFromYAML,
	},
	PluginRegistration{
		API:     pluginreg.LoaderLimitsAPI,
		Name:    pluginreg.LoaderLimitsAPI,
		Default: true,
		Factory: func(cfg map[string]any) (any, error) {
			return loaderlimits.NewFromYAML(cfg)
		},
	},
	PluginRegistration{
		API: pluginreg.YAMLParserAPI, Name: "go-yaml", Default: true,
		Factory: func(cfg map[string]any) (any, error) {
			if len(cfg) != 0 {
				return nil, errors.New(
					"yaml: go-yaml parser configuration must be empty")
			}
			return nativeParserPlugin{}, nil
		},
	},
)

// RegisterPlugin registers a named implementation for [OptsYAML] and
// [WithNamedPlugin]. Register during application startup, before parsing
// configuration. Duplicate API and implementation name pairs are errors.
func RegisterPlugin(registration PluginRegistration) error {
	return pluginRegistry.Register(registration)
}

func namedPlugin(api string, cfg map[string]any) (any, error) {
	return pluginRegistry.Create(api, cfg)
}

// WithNamedPlugin selects the implementation whose name matches the API.
func WithNamedPlugin(api string) Option {
	return func(o *libyaml.Options) error {
		p, err := namedPlugin(api, map[string]any{})
		if err != nil {
			return err
		}
		return WithPlugin(p)(o)
	}
}
