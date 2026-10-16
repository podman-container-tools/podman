// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package generate

import (
	"github.com/jessevdk/go-flags"

	"github.com/go-swagger/go-swagger/generator"
)

// pluginOptions exposes the template plugin, which relies on go plugins and is therefore
// not available on windows.
type pluginOptions struct {
	TemplatePlugin flags.Filename `description:"the template plugin to use" group:"shared" long:"template-plugin" short:"p"`
}

func (p pluginOptions) apply(opts *generator.GenOpts) {
	opts.TemplatePlugin = string(p.TemplatePlugin)
}
