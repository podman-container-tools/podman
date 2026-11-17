// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"

	"github.com/go-swagger/go-swagger/generator/internal/language"
)

// renderer drives template rendering: it resolves a template's output location,
// executes it with the configured func map, formats the result and writes the
// generated file.
//
// It embeds *GenOpts to reach the generation options (func map, templates
// repository, language options, sections, target, include flags...).
type renderer struct {
	*GenOpts
}

func newRenderer(g *GenOpts) *renderer {
	return &renderer{GenOpts: g}
}

func (g *renderer) location(t *TemplateOpts, data any) (string, string, error) {
	v := reflect.Indirect(reflect.ValueOf(data))
	fld := v.FieldByName("Name")
	var name string
	if fld.IsValid() {
		log.Println("name field", fld.String())
		name = fld.String()
	}

	fldpack := v.FieldByName("Package")
	pkg := g.APIPackage
	if fldpack.IsValid() {
		log.Println("package field", fldpack.String())
		pkg = fldpack.String()
	}

	var tags []string
	tagsF := v.FieldByName("Tags")
	if tagsF.IsValid() {
		if tt, ok := tagsF.Interface().([]string); ok {
			tags = tt
		}
	}

	var useTags bool
	useTagsF := v.FieldByName("UseTags")
	if useTagsF.IsValid() {
		var ok bool
		useTags, ok = useTagsF.Interface().(bool)
		if !ok {
			return "", "", fmt.Errorf("expected UseTags to be bool, but got %T", useTagsF.Interface())
		}
	}

	pthTpl, err := template.New(t.Name + "-target").Funcs(g.funcMap).Parse(t.Target)
	if err != nil {
		return "", "", err
	}

	fNameTpl, err := template.New(t.Name + "-filename").Funcs(g.funcMap).Parse(t.FileName)
	if err != nil {
		return "", "", err
	}

	d := struct {
		Name, CliAppName,
		Package, APIPackage, ServerPackage, ClientPackage, CliPackage, ModelPackage, MainPackage,
		Target string
		Tags    []string
		UseTags bool
		Context any
	}{
		Name:          name,
		CliAppName:    g.CliAppName,
		Package:       pkg,
		APIPackage:    g.APIPackage,
		ServerPackage: g.ServerPackage,
		ClientPackage: g.ClientPackage,
		CliPackage:    g.CliPackage,
		ModelPackage:  g.ModelPackage,
		MainPackage:   g.MainPackage,
		Target:        g.Target,
		Tags:          tags,
		UseTags:       useTags,
		Context:       data,
	}

	var pthBuf bytes.Buffer
	if e := pthTpl.Execute(&pthBuf, d); e != nil {
		return "", "", e
	}

	var fNameBuf bytes.Buffer
	if e := fNameTpl.Execute(&fNameBuf, d); e != nil {
		return "", "", e
	}
	return pthBuf.String(), g.fileName(fNameBuf.String()), nil
}

func (g *renderer) render(t *TemplateOpts, data any) ([]byte, error) {
	var templ *template.Template

	if strings.HasPrefix(strings.ToLower(t.Source), "asset:") {
		tt, err := g.templates.Get(strings.TrimPrefix(t.Source, "asset:"))
		if err != nil {
			return nil, err
		}
		templ = tt
	}

	if templ == nil {
		// try to load from repository (and enable dependencies)
		name := g.LanguageOpts.Mangler.ToJSONName(strings.TrimSuffix(t.Source, ".gotmpl"))
		tt, err := g.templates.Get(name)
		if err == nil {
			templ = tt
		}
	}

	if templ == nil {
		// try to load template from disk, in TemplateDir if specified
		// (dependencies resolution is limited to preloaded assets)
		var templateFile string
		if g.TemplateDir != "" {
			templateFile = filepath.Join(g.TemplateDir, t.Source)
		} else {
			templateFile = t.Source
		}
		content, err := os.ReadFile(templateFile)
		if err != nil {
			return nil, fmt.Errorf("error while opening %s template file: %w", templateFile, err)
		}
		tt, err := template.New(t.Source).Funcs(g.funcMap).Parse(string(content))
		if err != nil {
			return nil, fmt.Errorf("template parsing failed on template %s: %w", t.Name, err)
		}
		templ = tt
	}

	if templ == nil {
		return nil, fmt.Errorf("template %q not found", t.Source)
	}

	var tBuf bytes.Buffer
	if err := templ.Execute(&tBuf, data); err != nil {
		return nil, fmt.Errorf("template execution failed for template %s: %w", t.Name, err)
	}
	log.Printf("executed template %s", t.Source)

	return tBuf.Bytes(), nil
}

// Render template and write generated source code
// generated code is reformatted ("linted"), which gives an
// additional level of checking. If this step fails, the generated
// code is still dumped, for template debugging purposes.
func (g *renderer) write(t *TemplateOpts, data any) error {
	dir, fname, err := g.location(t, data)
	if err != nil {
		return fmt.Errorf("failed to resolve template location for template %s: %w", t.Name, err)
	}

	if t.SkipExists && fileExists(dir, fname) {
		return nil
	}

	log.Printf("creating generated file %q in %q as %s", fname, dir, t.Name)
	content, err := g.render(t, data)
	if err != nil {
		return fmt.Errorf("failed rendering template data for %s: %w", t.Name, err)
	}

	if dir != "" {
		_, exists := os.Stat(dir)
		if os.IsNotExist(exists) {
			// Directory settings consistent with file privileges.
			// Environment's umask may alter this setup
			if e := os.MkdirAll(dir, readAllDir); e != nil {
				return e
			}
		}
	}

	// Conditionally format the code, unless the user wants to skip
	formatted := content
	var writeerr error

	if !t.SkipFormat {
		var baseImport string

		baseImport, err = g.LanguageOpts.BaseImport(g.Target)
		if err != nil {
			return errTarget(g.Target, err)
		}

		formatted, err = g.LanguageOpts.FormatContent(
			filepath.Join(dir, fname), content,
			language.WithFormatOnly(g.LanguageOpts.FormatOnly),
			language.WithFormatLocalPrefixes(baseImport),
		)
		if err != nil {
			log.Printf("source formatting failed on template-generated source (%q for %s). Check that your template produces valid code", filepath.Join(dir, fname), t.Name)
			writeerr = os.WriteFile(filepath.Join(dir, fname), content, readAllFile) // #nosec
			if writeerr != nil {
				return fmt.Errorf("failed to write (unformatted) file %q in %q: %w", fname, dir, writeerr)
			}
			log.Printf("unformatted generated source %q has been dumped for template debugging purposes. DO NOT build on this source!", fname)
			return fmt.Errorf("source formatting on generated source %q failed: %w", t.Name, err)
		}
	}

	writeerr = os.WriteFile(filepath.Join(dir, fname), formatted, readAllFile) // #nosec
	if writeerr != nil {
		return fmt.Errorf("failed to write file %q in %q: %w", fname, dir, writeerr)
	}
	return err
}

func (g *renderer) fileName(in string) string {
	ext := filepath.Ext(in)
	return g.LanguageOpts.Mangler.ToFileName(strings.TrimSuffix(in, ext)) + ext
}

func (g *renderer) shouldRenderApp(t *TemplateOpts, _ *GenApp) bool {
	switch g.LanguageOpts.Mangler.ToFileName(g.LanguageOpts.Mangler.ToGoName(t.Name)) {
	case "main":
		return g.IncludeMain
	case "embedded_spec":
		return !g.ExcludeSpec
	default:
		return true
	}
}

func (g *renderer) shouldRenderOperations() bool {
	return g.IncludeHandler || g.IncludeParameters || g.IncludeResponses
}

func (g *renderer) renderApplication(app *GenApp) error {
	log.Printf("rendering %d templates for application %s", len(g.Sections.Application), app.Name)
	for _, tp := range g.Sections.Application {
		templ := tp
		if !g.shouldRenderApp(&templ, app) {
			continue
		}
		if err := g.write(&templ, app); err != nil {
			return err
		}
	}

	if len(g.Sections.PostModels) > 0 {
		log.Printf("post-rendering from %d models", len(app.Models))
		for _, templateToPin := range g.Sections.PostModels {
			templateConfig := templateToPin
			for _, modelToPin := range app.Models {
				modelData := modelToPin
				if err := g.write(&templateConfig, modelData); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (g *renderer) renderOperationGroup(gg *GenOperationGroup) error {
	log.Printf("rendering %d templates for operation group %s", len(g.Sections.OperationGroups), g.Name)
	for _, tp := range g.Sections.OperationGroups {
		templ := tp
		if !g.shouldRenderOperations() {
			continue
		}

		if err := g.write(&templ, gg); err != nil {
			return err
		}
	}
	return nil
}

func (g *renderer) renderOperation(gg *GenOperation) error {
	log.Printf("rendering %d templates for operation %s", len(g.Sections.Operations), g.Name)
	for _, tp := range g.Sections.Operations {
		templ := tp
		if !g.shouldRenderOperations() {
			continue
		}

		if err := g.write(&templ, gg); err != nil {
			return err
		}
	}
	return nil
}

func (g *renderer) renderDefinition(gg *GenDefinition) error {
	log.Printf("rendering %d templates for model %s", len(g.Sections.Models), gg.Name)
	for _, tp := range g.Sections.Models {
		templ := tp
		if !g.IncludeModel {
			continue
		}

		if err := g.write(&templ, gg); err != nil {
			return err
		}
	}
	return nil
}
