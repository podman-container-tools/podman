// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"github.com/go-swagger/go-swagger/generator"
)

// Support generates the supporting files.
type Support struct {
	WithShared
	WithModels
	WithOperations

	clientOptions
	serverOptions
	schemeOptions
	mediaOptions

	Name string `description:"the name of the application, defaults to a mangled value of info.title" long:"name" short:"A"`
}

// Usage documents the spec argument in the help message.
func (s Support) Usage() string {
	return usageWithSpec("support")
}

// Execute generates the supporting files file.
func (s *Support) Execute(args []string) error {
	return createSwagger(s, args)
}

// apply options.
func (s *Support) apply(opts *generator.GenOpts) {
	s.Shared.apply(opts)
	s.Models.apply(opts)
	s.Operations.apply(opts)
	s.clientOptions.apply(opts)
	s.serverOptions.apply(opts)
	s.schemeOptions.apply(opts)
	s.mediaOptions.apply(opts)
}

// generate support source.
func (s *Support) generate(opts *generator.GenOpts) error {
	return generator.GenerateSupport(s.Name, s.Models.Models, s.Operations.Operations, opts)
}

// log after generation.
func (s Support) log(_ string) {
	noticeImports()
}
