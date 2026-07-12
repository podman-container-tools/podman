// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"errors"
	"log"

	"github.com/go-swagger/go-swagger/generator"
)

type modelOptions struct {
	ModelPackage               string   `default:"models"                                                                                                    description:"the package to save the models" long:"model-package"   short:"m"`
	Models                     []string `description:"specify a model to include in generation, repeat for multiple (defaults to all)"                       long:"model"                                 short:"M"`
	ExistingModels             string   `description:"use pre-generated models e.g. github.com/foobar/model"                                                 long:"existing-models"`
	StrictAdditionalProperties bool     `description:"disallow extra properties when additionalProperties is set to false"                                   long:"strict-additional-properties"`
	KeepSpecOrder              bool     `description:"keep schema properties order identical to spec file"                                                   long:"keep-spec-order"`
	AllDefinitions             bool     `description:"generate all model definitions regardless of usage in operations"                                      hidden:"deprecated"                          long:"all-definitions"`
	StructTags                 []string `description:"the struct tags to generate, repeat for multiple (defaults to json)"                                   long:"struct-tags"`
	RootedErrorPath            bool     `description:"extends validation errors with the type name instead of an empty path, in the case of arrays and maps" long:"rooted-error-path"`
}

func (mo modelOptions) apply(opts *generator.GenOpts) {
	opts.ModelPackage = mo.ModelPackage
	opts.Models = mo.Models
	opts.ExistingModels = mo.ExistingModels
	opts.StrictAdditionalProperties = mo.StrictAdditionalProperties
	opts.PropertiesSpecOrder = mo.KeepSpecOrder
	opts.IgnoreOperations = mo.AllDefinitions
	opts.StructTags = mo.StructTags
	opts.WantsRootedErrorPath = mo.RootedErrorPath
}

// WithModels adds the model options group.
//
// This group is available to all commands that need some model generation.
type WithModels struct {
	Models modelOptions `group:"Options for model generation"`
}

// Model the generate model file command.
//
// Define the options that are specific to the "swagger generate model" command.
type Model struct {
	WithShared
	WithModels

	NoStruct              bool     `description:"when present will not generate the model struct"                                hidden:"deprecated"            long:"skip-struct"`
	Name                  []string `description:"the model to generate, repeat for multiple (defaults to all). Same as --models" long:"name"                    short:"n"`
	AcceptDefinitionsOnly bool     `description:"accepts a partial swagger spec with only the definitions key"                   long:"accept-definitions-only"`
}

// Execute generates a model file.
func (m *Model) Execute(_ []string) error {
	if m.Shared.DumpData && len(append(m.Name, m.Models.Models...)) > 1 {
		return errors.New("only 1 model at a time is supported for dumping data")
	}

	if m.Models.ExistingModels != "" {
		log.Println("warning: Ignoring existing-models flag when generating models.")
	}
	return createSwagger(m)
}

// apply options.
func (m Model) apply(opts *generator.GenOpts) {
	m.Shared.apply(opts)
	m.Models.apply(opts)

	opts.IncludeModel = !m.NoStruct
	opts.IncludeValidator = !m.NoStruct
	opts.AcceptDefinitionsOnly = m.AcceptDefinitionsOnly
}

func (m Model) log(_ string) {
	log.Println(`Generation completed!

For this generation to compile you need to have some packages in your go.mod:

	* github.com/go-openapi/validate
	* github.com/go-openapi/strfmt

You can get these now with: go mod tidy`)
}

func (m *Model) generate(opts *generator.GenOpts) error {
	return generator.GenerateModels(append(m.Name, m.Models.Models...), opts)
}
