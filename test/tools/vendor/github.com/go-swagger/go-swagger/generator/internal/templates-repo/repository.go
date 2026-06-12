// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"bytes"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"text/template/parse"

	"github.com/go-openapi/swag"
)

// AssetProvider provides access to embedded template assets.
type AssetProvider interface {
	AssetNames() []string
	MustAsset(name string) []byte
}

// Repository is the repository for the generator templates.
type Repository struct {
	files              map[string]string
	templates          map[string]*template.Template
	funcs              template.FuncMap
	protectedTemplates map[string]bool
	allowOverride      bool
	mux                sync.Mutex
}

// NewRepository creates a new template repository with the provided functions defined.
func NewRepository(funcs template.FuncMap) *Repository {
	repo := Repository{
		files:     make(map[string]string),
		templates: make(map[string]*template.Template),
		funcs:     funcs,
	}

	if repo.funcs == nil {
		repo.funcs = make(template.FuncMap)
	}

	return &repo
}

// SetProtectedTemplates sets the map of template names that cannot be overridden
// by user-provided templates.
func (t *Repository) SetProtectedTemplates(m map[string]bool) {
	t.protectedTemplates = m
}

// ShallowClone a repository.
//
// Clones the maps of files and templates, so as to be able to use
// the cloned repo concurrently.
func (t *Repository) ShallowClone() *Repository {
	clone := &Repository{
		files:              make(map[string]string, len(t.files)),
		templates:          make(map[string]*template.Template, len(t.templates)),
		funcs:              t.funcs,
		protectedTemplates: t.protectedTemplates,
		allowOverride:      t.allowOverride,
	}

	t.mux.Lock()
	defer t.mux.Unlock()

	maps.Copy(clone.files, t.files)
	maps.Copy(clone.templates, t.templates)

	return clone
}

// LoadDefaults loads templates from the given asset map.
func (t *Repository) LoadDefaults(assets map[string][]byte) error {
	for name, asset := range assets {
		if err := t.addFile(name, string(asset), true); err != nil {
			return err
		}
	}

	return nil
}

// LoadDir will walk the specified path and add each .gotmpl file it finds to the repository.
func (t *Repository) LoadDir(templatePath string) error {
	err := filepath.Walk(templatePath, func(path string, _ os.FileInfo, err error) error {
		if strings.HasSuffix(path, ".gotmpl") {
			if assetName, e := filepath.Rel(templatePath, path); e == nil {
				if data, e := os.ReadFile(path); e == nil { //nolint:gosec // pre-existing: template loading from user-specified directory
					if ee := t.AddFile(assetName, string(data)); ee != nil {
						return fmt.Errorf("could not add template: %w", ee)
					}
				}
				// Non-readable files are skipped
			}
		}

		if err != nil {
			return err
		}

		// Non-template files are skipped
		return nil
	})
	if err != nil {
		return fmt.Errorf("could not complete template processing in directory %q: %w", templatePath, err)
	}
	return nil
}

// LoadContrib loads template from contrib directory using the given asset provider.
func (t *Repository) LoadContrib(name string, provider AssetProvider) error {
	log.Printf("loading contrib %s", name)
	const pathPrefix = "templates/contrib/"
	basePath := pathPrefix + name
	filesAdded := 0
	for _, aname := range provider.AssetNames() {
		if !strings.HasSuffix(aname, ".gotmpl") {
			continue
		}
		if strings.HasPrefix(aname, basePath) {
			target := aname[len(basePath)+1:]
			err := t.addFile(target, string(provider.MustAsset(aname)), true)
			if err != nil {
				return err
			}
			log.Printf("added contributed template %s from %s", target, aname)
			filesAdded++
		}
	}
	if filesAdded == 0 {
		return fmt.Errorf("no files added from template: %s", name)
	}
	return nil
}

// MustGet a template by name, panics when fails.
func (t *Repository) MustGet(name string) *template.Template {
	tpl, err := t.Get(name)
	if err != nil {
		panic(err)
	}
	return tpl
}

// AddFile adds a file to the repository. It will create a new template based on the filename.
// It trims the .gotmpl from the end and converts the name using swag.ToJSONName. This will strip
// directory separators and Camelcase the next letter.
// e.g validation/primitive.gotmpl will become validationPrimitive
//
// If the file contains a definition for a template that is protected the whole file will not be added.
func (t *Repository) AddFile(name, data string) error {
	return t.addFile(name, data, false)
}

// SetAllowOverride allows setting allowOverride after the Repository was initialized.
func (t *Repository) SetAllowOverride(value bool) {
	t.allowOverride = value
}

// Get will return the named template from the repository, ensuring that all dependent templates are loaded.
// It will return an error if a dependent template is not defined in the repository.
func (t *Repository) Get(name string) (*template.Template, error) {
	templ, found := t.templates[name]

	if !found {
		return templ, fmt.Errorf("template doesn't exist %s", name)
	}

	return t.addDependencies(templ)
}

// DumpTemplates prints out a dump of all the defined templates, where they are defined and what their dependencies are.
func (t *Repository) DumpTemplates() {
	buf := bytes.NewBuffer(nil)
	fmt.Fprintln(buf, "\n# Templates")
	for name, templ := range t.templates {
		fmt.Fprintf(buf, "## %s\n", name)
		fmt.Fprintf(buf, "Defined in `%s`\n", t.files[name])

		if deps := findDependencies(templ.Root); len(deps) > 0 {
			fmt.Fprintf(buf, "####requires \n - %v\n\n\n", strings.Join(deps, "\n - "))
		}
		fmt.Fprintln(buf, "\n---")
	}
	log.Println(buf.String())
}

// Funcs returns the template function map, allowing callers to add or modify functions.
func (t *Repository) Funcs() template.FuncMap {
	return t.funcs
}

func (t *Repository) addFile(name, data string, allowOverride bool) error {
	fileName := name
	name = swag.ToJSONName(strings.TrimSuffix(name, ".gotmpl")) //nolint:staticcheck // tracked for migration to mangling.NameMangler

	templ, err := template.New(name).Funcs(t.funcs).Parse(data)
	if err != nil {
		return fmt.Errorf("failed to load template %s: %w", name, err)
	}

	// check if any protected templates are defined
	if !allowOverride && !t.allowOverride {
		for _, template := range templ.Templates() {
			if t.protectedTemplates[template.Name()] {
				return fmt.Errorf("cannot overwrite protected template %s", template.Name())
			}
		}
	}

	// Add each defined template into the cache
	for _, template := range templ.Templates() {
		t.files[template.Name()] = fileName
		t.templates[template.Name()] = template.Lookup(template.Name())
	}

	return nil
}

func (t *Repository) flattenDependencies(templ *template.Template, dependencies map[string]bool) map[string]bool {
	if dependencies == nil {
		dependencies = make(map[string]bool)
	}

	deps := findDependencies(templ.Root)

	for _, d := range deps {
		if _, found := dependencies[d]; !found {
			dependencies[d] = true

			if tt := t.templates[d]; tt != nil {
				dependencies = t.flattenDependencies(tt, dependencies)
			}
		}

		dependencies[d] = true
	}

	return dependencies
}

func (t *Repository) addDependencies(templ *template.Template) (*template.Template, error) {
	name := templ.Name()

	deps := t.flattenDependencies(templ, nil)

	for dep := range deps {
		if dep == "" {
			continue
		}

		tt := templ.Lookup(dep)

		// Check if we have it
		if tt == nil {
			tt = t.templates[dep]

			// Still don't have it, return an error
			if tt == nil {
				return templ, fmt.Errorf("could not find template %s", dep)
			}
			var err error

			// Add it to the parse tree
			templ, err = templ.AddParseTree(dep, tt.Tree)
			if err != nil {
				return templ, fmt.Errorf("dependency error: %w", err)
			}
		}
	}
	return templ.Lookup(name), nil
}

func findDependencies(n parse.Node) []string {
	depMap := make(map[string]bool)

	if n == nil {
		return nil
	}

	switch node := n.(type) {
	case *parse.ListNode:
		if node != nil && node.Nodes != nil {
			for _, nn := range node.Nodes {
				for _, dep := range findDependencies(nn) {
					depMap[dep] = true
				}
			}
		}
	case *parse.IfNode:
		for _, dep := range findDependencies(node.List) {
			depMap[dep] = true
		}
		for _, dep := range findDependencies(node.ElseList) {
			depMap[dep] = true
		}

	case *parse.RangeNode:
		for _, dep := range findDependencies(node.List) {
			depMap[dep] = true
		}
		for _, dep := range findDependencies(node.ElseList) {
			depMap[dep] = true
		}

	case *parse.WithNode:
		for _, dep := range findDependencies(node.List) {
			depMap[dep] = true
		}
		for _, dep := range findDependencies(node.ElseList) {
			depMap[dep] = true
		}

	case *parse.TemplateNode:
		depMap[node.Name] = true
	}

	deps := make([]string, 0, len(depMap))
	for dep := range depMap {
		deps = append(deps, dep)
	}

	return deps
}
