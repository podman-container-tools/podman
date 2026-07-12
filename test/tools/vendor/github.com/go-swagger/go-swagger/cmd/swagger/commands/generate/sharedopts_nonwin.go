// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

//go:build !windows
// +build !windows

package generate

import (
	"github.com/jessevdk/go-flags"

	"github.com/go-swagger/go-swagger/generator"
)

type sharedOptions struct {
	sharedOptionsCommon

	// TemplatePlugin option is not available on windows, since it relies on go plugins.
	TemplatePlugin flags.Filename `description:"the template plugin to use" group:"shared" long:"template-plugin" short:"p"`
}

func (s sharedOptions) apply(opts *generator.GenOpts) {
	opts.TemplatePlugin = string(s.TemplatePlugin)
	s.sharedOptionsCommon.apply(opts)
}
