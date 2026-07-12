// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	flags "github.com/jessevdk/go-flags"
	"github.com/spf13/viper"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/swag"

	"github.com/go-swagger/go-swagger/generator"
)

// FlattenCmdOptions determines options to the flatten spec preprocessing.
type FlattenCmdOptions struct {
	WithExpand          bool     `description:"expands all $ref's in spec prior to generation (shorthand to --with-flatten=expand)" group:"shared" long:"with-expand"`
	WithFlatten         []string `choice:"minimal"                                                                                  choice:"full"  choice:"expand"              choice:"verbose" choice:"noverbose" choice:"remove-unused" choice:"keep-names" default:"minimal" default:"verbose" description:"flattens all $ref's in spec prior to generation" group:"shared" long:"with-flatten"`
	WithCustomFormatter bool     `description:"use faster custom contributed go import processing instead of the standard one"      group:"shared" long:"with-custom-formatter"`
}

// SetFlattenOptions builds flatten options from command line args.
func (f *FlattenCmdOptions) SetFlattenOptions(dflt *analysis.FlattenOpts) (res *analysis.FlattenOpts) {
	res = &analysis.FlattenOpts{}
	if dflt != nil {
		*res = *dflt
	}
	if f == nil {
		return res
	}
	verboseIsSet := false
	minimalIsSet := false
	expandIsSet := false
	if f.WithExpand {
		res.Expand = true
		expandIsSet = true
	}
	for _, opt := range f.WithFlatten {
		switch opt {
		case "verbose":
			res.Verbose = true
			verboseIsSet = true
		case "noverbose":
			if !verboseIsSet {
				// verbose flag takes precedence
				res.Verbose = false
				verboseIsSet = true
			}
		case "remove-unused":
			res.RemoveUnused = true
		case "expand":
			res.Expand = true
			expandIsSet = true
		case "full":
			if !minimalIsSet && !expandIsSet {
				// minimal flag takes precedence
				res.Minimal = false
				minimalIsSet = true
			}
		case "minimal":
			if !expandIsSet {
				// expand flag takes precedence
				res.Minimal = true
				minimalIsSet = true
			}
		case "keep-names":
			res.KeepNames = true
		}
	}
	return res
}

type sharedCommand interface {
	apply(options *generator.GenOpts)
	getConfigFile() string
	generate(options *generator.GenOpts) error
	log(command string)
}

type schemeOptions struct {
	Principal     string `description:"the model to use for the security principal" long:"principal"                              short:"P"`
	DefaultScheme string `default:"http"                                            description:"the default scheme for this API" long:"default-scheme"`

	PrincipalIface bool `description:"the security principal provided is an interface, not a struct" long:"principal-is-interface"`
}

func (so schemeOptions) apply(opts *generator.GenOpts) {
	opts.Principal = so.Principal
	opts.PrincipalCustomIface = so.PrincipalIface
	opts.DefaultScheme = so.DefaultScheme
}

type mediaOptions struct {
	DefaultProduces string `default:"application/json" description:"the default mime type that API operations produce" long:"default-produces"`
	DefaultConsumes string `default:"application/json" description:"the default mime type that API operations consume" long:"default-consumes"`
}

func (m mediaOptions) apply(opts *generator.GenOpts) {
	opts.DefaultProduces = m.DefaultProduces
	opts.DefaultConsumes = m.DefaultConsumes

	const xmlIdentifier = "xml"
	opts.WithXML = strings.Contains(opts.DefaultProduces, xmlIdentifier) || strings.Contains(opts.DefaultConsumes, xmlIdentifier)
}

// WithShared adds the shared options group.
type WithShared struct {
	Shared sharedOptions `group:"Options common to all code generation commands"`
}

func (w WithShared) getConfigFile() string {
	return string(w.Shared.ConfigFile)
}

type sharedOptionsCommon struct {
	FlattenCmdOptions

	Spec                  flags.Filename `description:"the spec file to use (default swagger.{json,yml,yaml})"                             group:"shared"                                            long:"spec"                    short:"f"`
	Target                flags.Filename `default:"./"                                                                                     description:"the base directory for generating the files" group:"shared"                 long:"target"   short:"t"`
	Template              string         `choice:"stratoscale"                                                                             description:"load contributed templates"                  group:"shared"                 long:"template"`
	TemplateDir           flags.Filename `description:"alternative template override directory"                                            group:"shared"                                            long:"template-dir"            short:"T"`
	ConfigFile            flags.Filename `description:"configuration file to use for overriding template options"                          group:"shared"                                            long:"config-file"             short:"C"`
	CopyrightFile         flags.Filename `description:"copyright file used to add copyright header"                                        group:"shared"                                            long:"copyright-file"          short:"r"`
	AdditionalInitialisms []string       `description:"consecutive capitals that should be considered intialisms"                          group:"shared"                                            long:"additional-initialism"`
	AllowTemplateOverride bool           `description:"allows overriding protected templates"                                              group:"shared"                                            long:"allow-template-override"`
	SkipValidation        bool           `description:"skips validation of spec prior to generation"                                       group:"shared"                                            long:"skip-validation"`
	DumpData              bool           `description:"when present dumps the json for the template generator instead of generating files" group:"shared"                                            long:"dump-data"`
	StrictResponders      bool           `description:"Use strict type for the handler return value"                                       long:"strict-responders"`
	ReturnErrors          bool           `description:"handlers explicitly return an error as the second value"                            group:"shared"                                            long:"return-errors"           short:"e"`
}

func (s sharedOptionsCommon) apply(opts *generator.GenOpts) {
	opts.Spec = string(s.Spec)
	opts.Target = string(s.Target)
	opts.Template = s.Template
	opts.TemplateDir = string(s.TemplateDir)
	opts.AllowTemplateOverride = s.AllowTemplateOverride
	opts.ValidateSpec = !s.SkipValidation
	opts.DumpData = s.DumpData
	opts.FlattenOpts = s.SetFlattenOptions(opts.FlattenOpts)
	opts.Copyright = string(s.CopyrightFile)
	opts.StrictResponders = s.StrictResponders
	opts.ReturnErrors = s.ReturnErrors
	opts.WithCustomFormatter = s.WithCustomFormatter

	swag.AddInitialisms(s.AdditionalInitialisms...)
}

func setCopyright(copyrightFile string) (string, error) {
	// read the Copyright from file path in opts
	if copyrightFile == "" {
		return "", nil
	}
	bytebuffer, err := os.ReadFile(copyrightFile)
	if err != nil {
		return "", err
	}
	return string(bytebuffer), nil
}

func createSwagger(s sharedCommand) error {
	var (
		cfg *viper.Viper
		err error
	)

	if configFile := s.getConfigFile(); configFile != "" {
		// process explicit config file argument
		cfg, err = readConfig(configFile)
		if err != nil {
			return err
		}

		setDebug(cfg) // viper config Debug
	}

	opts := new(generator.GenOpts)
	s.apply(opts)

	opts.Copyright, err = setCopyright(opts.Copyright)
	if err != nil {
		return fmt.Errorf("could not load copyright file: %w", err)
	}

	if opts.Template != "" {
		contribOptionsOverride(opts)
	}

	if err = opts.EnsureDefaults(); err != nil {
		return err
	}

	if err = configureOptsFromConfig(cfg, opts); err != nil {
		return err
	}

	if err = s.generate(opts); err != nil {
		return err
	}

	basepath, err := filepath.Abs(".")
	if err != nil {
		return err
	}

	targetAbs, err := filepath.Abs(opts.Target)
	if err != nil {
		return err
	}
	// TODO(fredbi): we should try and remove the need to work with relative paths,
	// as this causes unnecessary constraints on os'es that support multiple drives
	// (i.e. not single root like on unix), for example Windows.
	rp, err := filepath.Rel(basepath, targetAbs)
	if err != nil {
		return err
	}

	s.log(rp)

	return nil
}

func readConfig(filename string) (*viper.Viper, error) {
	abspath, err := filepath.Abs(filename)
	if err != nil {
		return nil, err
	}

	log.Println("reading config from", abspath)

	return generator.ReadConfig(abspath)
}

func configureOptsFromConfig(cfg *viper.Viper, opts *generator.GenOpts) error {
	if cfg == nil {
		return nil
	}

	var def generator.LanguageDefinition
	if err := cfg.Unmarshal(&def); err != nil {
		return err
	}
	return def.ConfigureOpts(opts)
}

func setDebug(cfg *viper.Viper) {
	if os.Getenv("DEBUG") == "" && os.Getenv("SWAGGER_DEBUG") == "" {
		return
	}

	// viper config debug
	cfg.Debug()
}
