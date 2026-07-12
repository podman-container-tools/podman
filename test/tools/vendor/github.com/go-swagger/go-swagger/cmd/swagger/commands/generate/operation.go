// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"errors"
	"log"

	"github.com/go-swagger/go-swagger/generator"
)

type operationOptions struct {
	Operations []string `description:"specify an operation to include, repeat for multiple (defaults to all)" long:"operation"                                 short:"O"`
	Tags       []string `description:"the tags to include, if not specified defaults to all"                  group:"operations"                               long:"tags"`
	APIPackage string   `default:"operations"                                                                 description:"the package to save the operations" long:"api-package" short:"a"`
	WithEnumCI bool     `description:"allow case-insensitive enumerations"                                    long:"with-enum-ci"`

	// tags handling
	SkipTagPackages bool `description:"skips the generation of tag-based operation packages, resulting in a flat generation" long:"skip-tag-packages"`
}

func (oo operationOptions) apply(opts *generator.GenOpts) {
	opts.Operations = oo.Operations
	opts.Tags = oo.Tags
	opts.APIPackage = oo.APIPackage
	opts.AllowEnumCI = oo.WithEnumCI
	opts.SkipTagPackages = oo.SkipTagPackages
}

// WithOperations adds the operations options group.
type WithOperations struct {
	Operations operationOptions `group:"Options for operation generation"`
}

// Operation the generate operation files command.
type Operation struct {
	WithShared
	WithOperations

	clientOptions
	serverOptions
	schemeOptions
	mediaOptions

	ModelPackage string `default:"models" description:"the package to save the models" long:"model-package" short:"m"`

	NoHandler    bool `description:"when present will not generate an operation handler"       long:"skip-handler"`
	NoStruct     bool `description:"when present will not generate the parameter model struct" long:"skip-parameters"`
	NoResponses  bool `description:"when present will not generate the response model struct"  long:"skip-responses"`
	NoURLBuilder bool `description:"when present will not generate a URL builder"              long:"skip-url-builder"`

	Name []string `description:"the operations to generate, repeat for multiple (defaults to all). Same as --operations" long:"name" short:"n"`
}

// Execute generates a model file.
func (o *Operation) Execute(_ []string) error {
	if o.Shared.DumpData && len(append(o.Name, o.Operations.Operations...)) > 1 {
		return errors.New("only 1 operation at a time is supported for dumping data")
	}

	return createSwagger(o)
}

// apply options.
func (o Operation) apply(opts *generator.GenOpts) {
	o.Shared.apply(opts)
	o.Operations.apply(opts)
	o.clientOptions.apply(opts)
	o.serverOptions.apply(opts)
	o.schemeOptions.apply(opts)
	o.mediaOptions.apply(opts)

	opts.ModelPackage = o.ModelPackage
	opts.IncludeHandler = !o.NoHandler
	opts.IncludeResponses = !o.NoResponses
	opts.IncludeParameters = !o.NoStruct
	opts.IncludeURLBuilder = !o.NoURLBuilder
}

func (o *Operation) generate(opts *generator.GenOpts) error {
	return generator.GenerateServerOperation(append(o.Name, o.Operations.Operations...), opts)
}

func (o Operation) log(_ string) {
	log.Println(`Generation completed!

For this generation to compile you need to have some packages in your go.mod:

	* github.com/go-openapi/runtime

You can get these now with: go mod tidy`)
}
