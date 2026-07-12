// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"errors"
	"os"
	"path"

	"github.com/go-openapi/swag"
)

// GenerateClient generates a client library for a swagger spec document.
func GenerateClient(name string, modelNames, operationIDs []string, opts *GenOpts) error {
	if err := opts.CheckOpts(); err != nil {
		return err
	}

	if err := opts.setTemplates(); err != nil {
		return err
	}

	specDoc, analyzed, err := opts.analyzeSpec()
	if err != nil {
		return err
	}

	models, err := gatherModels(specDoc, modelNames)
	if err != nil {
		return err
	}

	operations := gatherOperations(analyzed, operationIDs)
	if len(operations) == 0 {
		return errors.New("no operations were selected")
	}

	generator := appGenerator{
		Name:              appNameOrDefault(specDoc, name, defaultClientName),
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
		Principal:         opts.PrincipalAlias(),
		DefaultScheme:     opts.DefaultScheme,
		DefaultProduces:   opts.DefaultProduces,
		DefaultConsumes:   opts.DefaultConsumes,
		GenOpts:           opts,
	}
	generator.Receiver = "o"
	return (&clientGenerator{generator}).Generate()
}

type clientGenerator struct {
	appGenerator
}

func (c *clientGenerator) Generate() error {
	app, err := c.makeCodegenApp()
	if err != nil {
		return err
	}
	app.DefaultImports["cli"] = path.Join(
		c.GenOpts.LanguageOpts.baseImport(c.Target),
		"cli",
	)
	app.DefaultImports["client"] = path.Join(
		c.GenOpts.LanguageOpts.baseImport(c.Target),
		"client",
	)
	app.DefaultImports["operations"] = path.Join(
		c.GenOpts.LanguageOpts.baseImport(c.Target),
		"client",
		"operations",
	)

	for i := range app.Models {
		di := app.Models[i].DefaultImports
		di["models"] = path.Join(
			c.GenOpts.LanguageOpts.baseImport(c.Target),
			"models",
		)
		di["client"] = path.Join(
			c.GenOpts.LanguageOpts.baseImport(c.Target),
			"client",
		)
	}

	if c.DumpData {
		return dumpData(os.Stdout, swag.ToDynamicJSON(app))
	}

	if c.GenOpts.IncludeModel {
		for _, m := range app.Models {
			if m.IsStream {
				continue
			}
			mod := m
			if err := c.GenOpts.renderDefinition(&mod); err != nil {
				return err
			}
		}
	}

	if c.GenOpts.IncludeHandler {
		for _, g := range app.OperationGroups {
			opg := g
			for _, o := range opg.Operations {
				op := o
				if err := c.GenOpts.renderOperation(&op); err != nil {
					return err
				}
			}
			if err := c.GenOpts.renderOperationGroup(&opg); err != nil {
				return err
			}
		}
	}

	if c.GenOpts.IncludeSupport {
		if err := c.GenOpts.renderApplication(&app); err != nil {
			return err
		}
	}

	return nil
}
