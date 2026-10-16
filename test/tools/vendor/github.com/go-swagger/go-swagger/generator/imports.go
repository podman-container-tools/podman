// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"path"
)

// importsBuilder produces the default import maps for generated models and
// operations. It embeds *GenOpts to reach the package layout, target and
// language options.
type importsBuilder struct {
	*GenOpts
}

func newImportsBuilder(g *GenOpts) *importsBuilder {
	return &importsBuilder{GenOpts: g}
}

// defaultImports produces a default map for imports with models.
func (g *importsBuilder) defaultImports() (map[string]string, error) {
	baseImport, err := g.LanguageOpts.BaseImport(g.Target)
	if err != nil {
		return nil, errTarget(g.Target, err)
	}

	defaultImports := make(map[string]string, sensibleDefaultMapAlloc)

	var modelsAlias, importPath string
	if g.ExistingModels == "" {
		// generated models
		importPath = path.Join(
			baseImport,
			g.LanguageOpts.ManglePackagePath(g.ModelPackage, defaultModelsTarget))
		modelsAlias = g.LanguageOpts.ManglePackageName(g.ModelPackage, defaultModelsTarget)
	} else {
		// external models
		importPath = g.LanguageOpts.ManglePackagePath(g.ExistingModels, "")
		modelsAlias = path.Base(defaultModelsTarget)
	}
	defaultImports[modelsAlias] = importPath

	// resolve model representing an authenticated principal
	alias, _, target := resolvePrincipal(g.Principal)
	if alias == "" || target == g.ModelPackage || path.Base(target) == modelsAlias {
		// if principal is specified with the models generation package, do not import any extra package
		return defaultImports, nil
	}

	if pth, _ := path.Split(target); pth != "" {
		// if principal is specified with a path, assume this is a fully qualified package and generate this import
		defaultImports[alias] = target
	} else {
		// if principal is specified with a relative path (no "/", e.g. internal.Principal), assume it is located in generated target
		defaultImports[alias] = path.Join(baseImport, target)
	}

	return defaultImports, nil
}

// initImports produces a default map for import with the specified root for operations.
func (g *importsBuilder) initImports(operationsPackage string) (map[string]string, error) {
	baseImport, err := g.LanguageOpts.BaseImport(g.Target)
	if err != nil {
		return nil, errTarget(g.Target, err)
	}

	imports := make(map[string]string, sensibleDefaultMapAlloc)
	imports[g.LanguageOpts.ManglePackageName(operationsPackage, defaultOperationsTarget)] = path.Join(
		baseImport,
		g.LanguageOpts.ManglePackagePath(operationsPackage, defaultOperationsTarget))

	return imports, nil
}

func errTarget(target string, err error) error {
	return fmt.Errorf("could not determine import path for target %q. All generated source must reside within a single target tree under GOPATH or a module: %w", target, err)
}
