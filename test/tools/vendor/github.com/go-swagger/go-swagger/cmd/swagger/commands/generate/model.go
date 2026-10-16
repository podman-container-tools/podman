// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"errors"
	"log"

	"github.com/go-swagger/go-swagger/generator"
)

// modelOptionsCommon selects the models to render and the order they are rendered in.
//
// These options bear on any rendering of the models, go code or not.
type modelOptionsCommon struct {
	Models         []string `description:"specify a model to include in generation, repeat for multiple (defaults to all)" long:"model"           short:"M"`
	KeepSpecOrder  bool     `description:"keep schema properties order identical to spec file"                             long:"keep-spec-order"`
	AllDefinitions bool     `description:"generate all model definitions regardless of usage in operations"                hidden:"deprecated"    long:"all-definitions"`
}

func (mo modelOptionsCommon) apply(opts *generator.GenOpts) {
	opts.Models = mo.Models
	opts.PropertiesSpecOrder = mo.KeepSpecOrder
	opts.IgnoreOperations = mo.AllDefinitions
}

// modelCodegenOptions only bear on the go code generated for models, starting with the
// package this code is written to.
type modelCodegenOptions struct {
	ModelPackage               string   `default:"models"                                                                                                           description:"the package to save the models" long:"model-package" short:"m"`
	ExistingModels             string   `description:"use pre-generated models e.g. github.com/foobar/model"                                                        long:"existing-models"`
	StrictAdditionalProperties bool     `description:"disallow extra properties when additionalProperties is set to false"                                          long:"strict-additional-properties"`
	StructTags                 []string `description:"the struct tags to generate, repeat for multiple (defaults to json)"                                          long:"struct-tags"`
	RootedErrorPath            bool     `description:"extends validation errors with the type name instead of an empty path, in the case of arrays and maps"        long:"rooted-error-path"`
	WithStringer               bool     `description:"generate a fmt.Stringer String() method on models, rendering field values as JSON (see issue #872)"           long:"with-stringer"`
	GenerateGetters            bool     `description:"generate a Get<Field> method for each field on models"                                                        long:"generate-getters"`
	NoDefaultOmitEmpty         bool     `description:"do not default to omitempty struct tags unless x-omitempty is explicitly set on a property (see issue #2386)" long:"no-default-omit-empty"`
}

func (mo modelCodegenOptions) apply(opts *generator.GenOpts) {
	opts.ModelPackage = mo.ModelPackage
	opts.ExistingModels = mo.ExistingModels
	opts.StrictAdditionalProperties = mo.StrictAdditionalProperties
	opts.StructTags = mo.StructTags
	opts.WantsRootedErrorPath = mo.RootedErrorPath
	opts.WantsStringer = mo.WithStringer
	opts.WantsGetters = mo.GenerateGetters
	opts.NoDefaultOmitEmpty = mo.NoDefaultOmitEmpty
}

type modelOptions struct {
	modelOptionsCommon
	modelCodegenOptions
}

func (mo modelOptions) apply(opts *generator.GenOpts) {
	mo.modelOptionsCommon.apply(opts)
	mo.modelCodegenOptions.apply(opts)
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

// Usage documents the spec argument in the help message.
func (m Model) Usage() string {
	return usageWithSpec("model")
}

// Execute generates a model file.
func (m *Model) Execute(args []string) error {
	if m.Shared.DumpData && len(append(m.Name, m.Models.Models...)) > 1 {
		return errors.New("only 1 model at a time is supported for dumping data")
	}

	if m.Models.ExistingModels != "" {
		log.Println("warning: Ignoring existing-models flag when generating models.")
	}
	return createSwagger(m, args)
}

// apply options.
func (m Model) apply(opts *generator.GenOpts) {
	m.Shared.apply(opts)
	m.Models.apply(opts)

	opts.IncludeModel = !m.NoStruct
	opts.IncludeValidator = !m.NoStruct
	opts.AcceptDefinitionsOnly = m.AcceptDefinitionsOnly
}

func (m *Model) generate(opts *generator.GenOpts) error {
	return generator.GenerateModels(append(m.Name, m.Models.Models...), opts)
}

func (m Model) log(_ string) {
	noticeImports()
}
