// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"text/template"

	"github.com/spf13/viper"

	"github.com/go-openapi/analysis"

	"github.com/go-swagger/go-swagger/generator/internal/language"
	templatesrepo "github.com/go-swagger/go-swagger/generator/internal/templates-repo"
)

// GenOpts encapsulates the generator options.
//
// TemplatePlugin names an optional Go plugin that injects extra template
// functions. Go plugins are only supported on non-Windows platforms; on
// Windows the option is accepted but ignored (see [repo.Repository.LoadPlugin]).
type GenOpts struct {
	IncludeModel               bool
	IncludeValidator           bool
	IncludeHandler             bool
	IncludeParameters          bool
	IncludeResponses           bool
	IncludeURLBuilder          bool
	IncludeMain                bool
	IncludeSupport             bool
	IncludeCLi                 bool
	ExcludeSpec                bool
	DumpData                   bool
	ValidateSpec               bool
	FlattenOpts                *analysis.FlattenOpts
	IsClient                   bool
	machineryBuilt             bool // guards buildMachinery (language opts, func map, templates repo)
	sectionsResolved           bool // guards resolveSections (default render plan)
	prepared                   bool // guards Prepare
	PropertiesSpecOrder        bool
	StrictAdditionalProperties bool
	AllowTemplateOverride      bool

	Spec                   string
	APIPackage             string
	ModelPackage           string
	ServerPackage          string
	ClientPackage          string
	CliPackage             string
	CliAppName             string // name of cli app. For example "dockerctl"
	ImplementationPackage  string
	Principal              string
	PrincipalCustomIface   bool   // user-provided interface for Principal (non-nullable)
	Target                 string // dir location where generated code is written to
	Sections               SectionOpts
	LanguageOpts           *language.Options
	TypeMapping            map[string]string
	Imports                map[string]string
	DefaultScheme          string
	DefaultProduces        string
	DefaultConsumes        string
	WithXML                bool
	TemplateDir            string
	Template               string
	TemplatePlugin         string
	RegenerateConfigureAPI bool
	Operations             []string
	Models                 []string
	Tags                   []string
	StructTags             []string
	Name                   string
	FlagStrategy           string
	CompatibilityMode      string
	ExistingModels         string
	Copyright              string
	SkipTagPackages        bool
	MainPackage            string
	IgnoreOperations       bool
	AllowEnumCI            bool
	StrictResponders       bool
	AcceptDefinitionsOnly  bool
	WantsRootedErrorPath   bool
	WantsStringer          bool
	ReturnErrors           bool
	WithCustomFormatter    bool
	WithExtraInitialisms   []string
	Restricted             bool
	Rooted                 string

	// Viper carries an optional configuration (typically a `.swagger.{yml,json}`
	// file). Its `layout:` sections are applied as overrides on top of the
	// default render plan during Prepare.
	Viper *viper.Viper

	templates *templatesrepo.Repository
	funcMap   template.FuncMap
}

// loadTemplates loads the optional template plugin, the selected contrib
// templates and the custom template directory configured on the options.
func (g *GenOpts) loadTemplates() error {
	if g.TemplatePlugin != "" {
		if err := g.templates.LoadPlugin(g.TemplatePlugin); err != nil {
			return err
		}
	}

	if g.Template != "" {
		// set contrib templates
		if err := g.templates.LoadContrib(g.Template, embeddedAssets{}); err != nil {
			return err
		}
	}

	g.templates.SetAllowOverride(g.AllowTemplateOverride)

	if g.TemplateDir != "" {
		// set custom templates
		if err := g.templates.LoadDir(g.TemplateDir); err != nil {
			return err
		}
	}

	return nil
}
