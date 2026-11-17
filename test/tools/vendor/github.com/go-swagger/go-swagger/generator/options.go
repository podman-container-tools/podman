// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import "github.com/spf13/viper"

// Option configures a [GenOpts] when building it with [NewGenOpts].
//
// The option surface is intentionally small: presets ([ForServer], [ForClient],
// ...) plus a few ubiquitous setters. Anything else is set directly on the
// exported [GenOpts] fields before the options are finalized by [GenOpts.Prepare].
type Option func(*GenOpts)

// NewGenOpts builds a [GenOpts] and applies the given options.
//
// It performs no I/O and builds no derived state — that happens in
// [GenOpts.Prepare], which the Generate* entry points call themselves. A typical
// caller selects a preset, sets the spec and target, tweaks any exported fields
// it needs and hands the result to a Generate* function:
//
//	opts := generator.NewGenOpts(generator.ForServer(),
//		generator.WithSpec("swagger.yml"), generator.WithTarget("./gen"))
//	err := generator.GenerateServer("MyAPI", models, operations, opts)
func NewGenOpts(opts ...Option) *GenOpts {
	g := &GenOpts{}
	for _, apply := range opts {
		apply(g)
	}

	return g
}

// WithSpec sets the source spec location (a file path or an http(s) URL).
func WithSpec(spec string) Option {
	return func(g *GenOpts) { g.Spec = spec }
}

// WithTarget sets the directory where generated code is written.
func WithTarget(target string) Option {
	return func(g *GenOpts) { g.Target = target }
}

// WithViper sets an optional configuration whose `layout:` sections override the
// default render plan during [GenOpts.Prepare].
func WithViper(cfg *viper.Viper) Option {
	return func(g *GenOpts) { g.Viper = cfg }
}

// WithTemplatePlugin sets a Go plugin providing extra template functions.
//
// Go plugins are not supported on Windows, where the option is ignored.
func WithTemplatePlugin(pluginPath string) Option {
	return func(g *GenOpts) { g.TemplatePlugin = pluginPath }
}

// applyStandardLayout sets the conventional package names shared by the presets.
func applyStandardLayout(g *GenOpts) {
	g.APIPackage = defaultOperationsTarget
	g.ModelPackage = defaultModelsTarget
	g.ServerPackage = defaultServerTarget
	g.ClientPackage = defaultClientTarget
}

// ForServer configures the options to generate a server: models, validators,
// handlers, parameters, responses and the supporting files.
func ForServer() Option {
	return func(g *GenOpts) {
		applyStandardLayout(g)
		g.IsClient = false
		g.IncludeModel = true
		g.IncludeValidator = true
		g.IncludeHandler = true
		g.IncludeParameters = true
		g.IncludeResponses = true
		g.IncludeSupport = true
	}
}

// ForClient configures the options to generate a client.
func ForClient() Option {
	return func(g *GenOpts) {
		applyStandardLayout(g)
		g.IsClient = true
		g.IncludeModel = true
		g.IncludeHandler = true
		g.IncludeParameters = true
		g.IncludeResponses = true
		g.IncludeSupport = true
	}
}

// ForModel configures the options to generate models only.
func ForModel() Option {
	return func(g *GenOpts) {
		applyStandardLayout(g)
		g.IncludeModel = true
	}
}

// ForCli configures the options to generate a command-line client.
func ForCli() Option {
	return func(g *GenOpts) {
		applyStandardLayout(g)
		g.IsClient = true
		g.IncludeCLi = true
		g.CliPackage = defaultCliTarget
		g.CliAppName = defaultCliTarget
		g.IncludeModel = true
		g.IncludeHandler = true
		g.IncludeParameters = true
		g.IncludeResponses = true
		g.IncludeSupport = true
	}
}

// ForMarkdown configures the options to generate markdown documentation.
func ForMarkdown() Option {
	return func(g *GenOpts) {
		applyStandardLayout(g)
		g.IncludeModel = true
	}
}
