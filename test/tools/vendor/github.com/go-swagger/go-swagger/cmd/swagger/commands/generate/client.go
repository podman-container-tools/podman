// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"github.com/go-swagger/go-swagger/generator"
)

type clientOptions struct {
	ClientPackage string `default:"client" description:"the package to save the client specific code" long:"client-package" short:"c"`
}

func (co clientOptions) apply(opts *generator.GenOpts) {
	opts.ClientPackage = co.ClientPackage
}

// Client the command to generate a swagger client.
type Client struct {
	WithShared
	WithModels
	WithOperations

	clientOptions
	schemeOptions
	mediaOptions

	SkipModels     bool `description:"no models will be generated when this flag is specified"     long:"skip-models"`
	SkipOperations bool `description:"no operations will be generated when this flag is specified" long:"skip-operations"`

	Name string `description:"the name of the application, defaults to a mangled value of info.title" long:"name" short:"A"`
}

// Execute runs this command.
func (c *Client) Execute(_ []string) error {
	return createSwagger(c)
}

// apply options.
func (c Client) apply(opts *generator.GenOpts) {
	c.Shared.apply(opts)
	c.Models.apply(opts)
	c.Operations.apply(opts)
	c.clientOptions.apply(opts)
	c.schemeOptions.apply(opts)
	c.mediaOptions.apply(opts)

	opts.IncludeModel = !c.SkipModels
	opts.IncludeValidator = !c.SkipModels
	opts.IncludeHandler = !c.SkipOperations
	opts.IncludeParameters = !c.SkipOperations
	opts.IncludeResponses = !c.SkipOperations
	opts.Name = c.Name

	opts.IsClient = true
	opts.IncludeSupport = true
}

func (c *Client) generate(opts *generator.GenOpts) error {
	return generator.GenerateClient(c.Name, c.Models.Models, c.Operations.Operations, opts)
}

func (c *Client) log(_ string) {
	noticeImports()
}
