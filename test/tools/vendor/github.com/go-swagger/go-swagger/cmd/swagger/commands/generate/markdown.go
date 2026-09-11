// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"github.com/jessevdk/go-flags"

	"github.com/go-swagger/go-swagger/generator"
)

// Markdown generates a markdown representation of the spec.
//
// This command documents the API as it is generated, but generates no go code: it only exposes
// the options that bear on the markdown it produces. The options that shape go source, such as
// --struct-tags or --strict-responders, are left out.
type Markdown struct {
	Shared     markdownSharedOptions  `group:"Options for reading the spec and writing the documentation"`
	Models     modelOptionsCommon     `group:"Options for selecting the documented models"`
	Operations operationOptionsCommon `group:"Options for selecting the documented operations"`

	Output flags.Filename `default:"markdown.md" description:"the file to write the generated markdown." long:"output" short:""`
}

// Usage documents the spec argument in the help message.
func (m Markdown) Usage() string {
	return usageWithSpec("markdown")
}

// Execute runs this command.
func (m *Markdown) Execute(args []string) error {
	return createSwagger(m, args)
}

func (m Markdown) getConfigFile() string {
	return string(m.Shared.ConfigFile)
}

// apply options.
func (m Markdown) apply(opts *generator.GenOpts) {
	m.Shared.apply(opts)
	m.Models.apply(opts)
	m.Operations.apply(opts)
}

func (m *Markdown) generate(opts *generator.GenOpts) error {
	return generator.GenerateMarkdown(string(m.Output), m.Models.Models, m.Operations.Operations, opts)
}

func (m Markdown) log(_ string) {
}
