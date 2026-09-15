// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"errors"
	"os"
	"path"

	"github.com/go-openapi/swag/jsonutils"
)

// GenerateClient generates a client library for a swagger spec document.
func GenerateClient(name string, modelNames, operationIDs []string, opts *GenOpts) error {
	if err := opts.Prepare(); err != nil {
		return err
	}

	specDoc, analyzed, err := newSpecAnalyzer(opts).analyzeSpec()
	if err != nil {
		return err
	}

	models, err := gatherModels(specDoc, modelNames)
	if err != nil {
		return err
	}

	operations := gatherOperations(opts, analyzed, operationIDs)
	if len(operations) == 0 {
		return errors.New("no operations were selected")
	}

	mangler := opts.LanguageOpts.Mangler
	funcMap := opts.funcMap
	mediaMime, ok := funcMap["mediaTypeName"].(func(string) string)
	if !ok {
		return errors.New("internal error: mediaTypeName function expected to be func(string) string")
	}

	generator := appGenerator{
		Name:              appNameOrDefault(opts.LanguageOpts, specDoc, name, defaultClientName),
		SpecDoc:           specDoc,
		Analyzed:          analyzed,
		Models:            models,
		Operations:        operations,
		Target:            opts.Target,
		DumpData:          opts.DumpData,
		Package:           opts.LanguageOpts.ManglePackageName(opts.ClientPackage, defaultClientTarget),
		APIPackage:        opts.LanguageOpts.ManglePackagePath(opts.APIPackage, defaultOperationsTarget),
		ModelsPackage:     opts.LanguageOpts.ManglePackagePath(opts.ModelPackage, defaultModelsTarget),
		ServerPackage:     opts.LanguageOpts.ManglePackagePath(opts.ServerPackage, defaultServerTarget),
		ClientPackage:     opts.LanguageOpts.ManglePackagePath(opts.ClientPackage, defaultClientTarget),
		OperationsPackage: opts.LanguageOpts.ManglePackagePath(opts.ClientPackage, defaultClientTarget),
		Principal:         principalAlias(opts.Principal),
		DefaultScheme:     opts.DefaultScheme,
		DefaultProduces:   opts.DefaultProduces,
		DefaultConsumes:   opts.DefaultConsumes,
		GenOpts:           opts,
		mangler:           mangler,
		mediaMime:         mediaMime,
	}
	generator.Receiver = "o"
	return (&clientGenerator{generator}).Generate()
}

type clientGenerator struct {
	appGenerator
}

func (c *clientGenerator) Generate() error {
	importTarget, err := c.GenOpts.LanguageOpts.BaseImport(c.Target)
	if err != nil {
		return errTarget(c.Target, err)
	}

	app, err := c.makeCodegenApp()
	if err != nil {
		return err
	}
	app.DefaultImports["cli"] = path.Join(
		importTarget,
		"cli",
	)
	app.DefaultImports["client"] = path.Join(
		importTarget,
		"client",
	)
	app.DefaultImports["operations"] = path.Join(
		importTarget,
		"client",
		"operations",
	)

	for i := range app.Models {
		di := app.Models[i].DefaultImports
		di["models"] = path.Join(
			importTarget,
			"models",
		)
		di["client"] = path.Join(
			importTarget,
			"client",
		)
	}

	if c.DumpData {
		var dynamicApp any
		if err := jsonutils.FromDynamicJSON(app, &dynamicApp); err != nil {
			return err
		}

		return dumpData(os.Stdout, dynamicApp)
	}

	if c.GenOpts.IncludeModel {
		for _, m := range app.Models {
			if m.IsStream {
				continue
			}
			mod := m
			if err := newRenderer(c.GenOpts).renderDefinition(&mod); err != nil {
				return err
			}
		}
	}

	if c.GenOpts.IncludeHandler {
		for _, g := range app.OperationGroups {
			opg := g
			for _, o := range opg.Operations {
				op := o
				if err := newRenderer(c.GenOpts).renderOperation(&op); err != nil {
					return err
				}
			}
			if err := newRenderer(c.GenOpts).renderOperationGroup(&opg); err != nil {
				return err
			}
		}
	}

	if c.GenOpts.IncludeSupport {
		if err := newRenderer(c.GenOpts).renderApplication(&app); err != nil {
			return err
		}
	}

	return nil
}
