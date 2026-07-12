// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-openapi/analysis"
	swaggererrors "github.com/go-openapi/errors"
	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"

	yamlv2 "gopkg.in/yaml.v2"
)

func (g *GenOpts) validateAndFlattenSpec() (*loads.Document, error) {
	// Load spec document
	specDoc, err := loads.Spec(g.Spec)
	if err != nil {
		return nil, err
	}

	// If accepts definitions only, add dummy swagger header to pass validation
	if g.AcceptDefinitionsOnly {
		specDoc, err = applyDefaultSwagger(specDoc)
		if err != nil {
			return nil, err
		}
	}

	// Validate if needed
	if g.ValidateSpec {
		log.Printf("validating spec %v", g.Spec)
		validationErrors := validate.Spec(specDoc, strfmt.Default)
		if validationErrors != nil {
			var b strings.Builder

			fmt.Fprintf(&b,
				"The swagger spec at %q is invalid against the swagger specification %s. See errors :\n",
				g.Spec, specDoc.Version(),
			)

			var cerr *swaggererrors.CompositeError
			if errors.As(validationErrors, &cerr) {
				for _, desc := range cerr.Errors {
					fmt.Fprintf(&b, "- %s\n", desc)
				}
			}

			return nil, errors.New(b.String())
		}

		// TODO(fredbi): due to uncontrolled $ref state in spec, we need to reload the spec atm, or flatten won't
		// work properly (validate expansion alters the $ref cache in go-openapi/spec)
		specDoc, _ = loads.Spec(g.Spec)
	}

	// Flatten spec
	//
	// Some preprocessing is required before codegen
	//
	// This ensures at least that $ref's in the spec document are canonical,
	// i.e all $ref are local to this file and point to some uniquely named definition.
	//
	// Default option is to ensure minimal flattening of $ref, bundling remote $refs and relocating arbitrary JSON
	// pointers as definitions.
	// This preprocessing may introduce duplicate names (e.g. remote $ref with same name). In this case, a definition
	// suffixed with "OAIGen" is produced.
	//
	// Full flattening option farther transforms the spec by moving every complex object (e.g. with some properties)
	// as a standalone definition.
	//
	// Eventually, an "expand spec" option is available. It is essentially useful for testing purposes.
	//
	// NOTE(fredbi): spec expansion may produce some unsupported constructs and is not yet protected against the
	// following cases:
	//  - polymorphic types generation may fail with expansion (expand destructs the reuse intent of the $ref in allOf)
	//  - name duplicates may occur and result in compilation failures
	//
	// The right place to fix these shortcomings is go-openapi/analysis.

	g.FlattenOpts.BasePath = specDoc.SpecFilePath()
	g.FlattenOpts.Spec = analysis.New(specDoc.Spec())

	g.printFlattenOpts()

	if err = analysis.Flatten(*g.FlattenOpts); err != nil {
		return nil, err
	}

	if g.FlattenOpts.Expand {
		// for a similar reason as the one mentioned above for validate,
		// schema expansion alters the internal doc cache in the spec.
		// This nasty bug (in spec expander) affects circular references.
		// So we need to reload the spec from a clone.
		// Notice that since the spec inside the document has been modified, we should
		// ensure that Pristine refreshes its row root document.
		specDoc = specDoc.Pristine()
	}

	// yields the preprocessed spec document
	return specDoc, nil
}

func (g *GenOpts) analyzeSpec() (*loads.Document, *analysis.Spec, error) {
	// load, validate and flatten
	specDoc, err := g.validateAndFlattenSpec()
	if err != nil {
		return nil, nil, err
	}

	// spec preprocessing option
	if g.PropertiesSpecOrder {
		g.Spec = WithAutoXOrder(g.Spec)
		specDoc, err = loads.Spec(g.Spec)
		if err != nil {
			return nil, nil, err
		}
	}

	// analyze the spec
	analyzed := analysis.New(specDoc.Spec())

	return specDoc, analyzed, nil
}

func (g *GenOpts) printFlattenOpts() {
	var preprocessingOption string
	switch {
	case g.FlattenOpts.Expand:
		preprocessingOption = "expand"
	case g.FlattenOpts.Minimal:
		preprocessingOption = "minimal flattening"
	default:
		preprocessingOption = "full flattening"
	}
	log.Printf("preprocessing spec with option:  %s", preprocessingOption)
}

// findSwaggerSpec fetches a default swagger spec if none is provided.
func findSwaggerSpec(nm string) (string, error) {
	specs := []string{"swagger.json", "swagger.yml", "swagger.yaml"}
	if nm != "" {
		specs = []string{nm}
	}
	var name string
	for _, nn := range specs {
		f, err := os.Stat(nn)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if f.IsDir() {
			return "", fmt.Errorf("%s is a directory", nn)
		}
		name = nn
		break
	}
	if name == "" {
		return "", errors.New("couldn't find a swagger spec")
	}
	return name, nil
}

// WithAutoXOrder amends the spec to specify property order as they appear
// in the spec (supports yaml documents only).
//
//nolint:gocognit // TODO(fredbi): refactor
func WithAutoXOrder(specPath string) string {
	lookFor := func(ele any, key string) (yamlv2.MapSlice, bool) {
		if slice, ok := ele.(yamlv2.MapSlice); ok {
			for _, v := range slice {
				if v.Key == key {
					if slice, ok := v.Value.(yamlv2.MapSlice); ok {
						return slice, ok
					}
				}
			}
		}
		return nil, false
	}

	var addXOrder func(any)
	addXOrder = func(element any) {
		if props, ok := lookFor(element, "properties"); ok {
			for i, prop := range props {
				if pSlice, ok := prop.Value.(yamlv2.MapSlice); ok {
					isObject := false
					xOrderIndex := -1 // find if x-order already exists

					for i, v := range pSlice {
						if v.Key == "type" && v.Value == object {
							isObject = true
						}
						if v.Key == xOrder {
							xOrderIndex = i
							break
						}
					}

					if xOrderIndex > -1 { // override existing x-order
						pSlice[xOrderIndex] = yamlv2.MapItem{Key: xOrder, Value: i}
					} else { // append new x-order
						pSlice = append(pSlice, yamlv2.MapItem{Key: xOrder, Value: i})
					}
					prop.Value = pSlice
					props[i] = prop

					if isObject {
						addXOrder(pSlice)
					}
				}
			}
		}
	}

	data, err := swag.LoadFromFileOrHTTP(specPath)
	if err != nil {
		panic(err)
	}

	yamlDoc, err := BytesToYAMLv2Doc(data)
	if err != nil {
		panic(err)
	}

	if defs, ok := lookFor(yamlDoc, "definitions"); ok {
		for _, def := range defs {
			addXOrder(def.Value)
		}
	}

	addXOrder(yamlDoc)

	out, err := yamlv2.Marshal(yamlDoc)
	if err != nil {
		panic(err)
	}

	tmpDir, err := os.MkdirTemp("", "go-swagger-")
	if err != nil {
		panic(err)
	}

	tmpFile := filepath.Join(tmpDir, filepath.Base(specPath))
	if err := os.WriteFile(tmpFile, out, readableFile); err != nil {
		panic(err)
	}
	return tmpFile
}

// BytesToYAMLv2Doc converts a byte slice into a YAML document.
func BytesToYAMLv2Doc(data []byte) (any, error) {
	var canary map[any]any // validate this is an object and not a different type
	if err := yamlv2.Unmarshal(data, &canary); err != nil {
		return nil, err
	}

	var document yamlv2.MapSlice // preserve order that is present in the document
	if err := yamlv2.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	return document, nil
}

func applyDefaultSwagger(doc *loads.Document) (*loads.Document, error) {
	// bake a minimal swagger spec to pass validation
	swspec := doc.Spec()
	if swspec.Swagger == "" {
		swspec.Swagger = "2.0"
	}
	if swspec.Info == nil {
		info := new(spec.Info)
		info.Version = "0.0.0"
		info.Title = "minimal"
		swspec.Info = info
	}
	if swspec.Paths == nil {
		swspec.Paths = &spec.Paths{}
	}
	// rewrite the document with the new addition
	jazon, err := json.Marshal(swspec)
	if err != nil {
		return nil, err
	}
	return loads.Analyzed(jazon, swspec.Swagger)
}
