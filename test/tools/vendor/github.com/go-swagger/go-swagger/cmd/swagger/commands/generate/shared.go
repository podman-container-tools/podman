// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	flags "github.com/jessevdk/go-flags"
	"github.com/spf13/viper"

	"github.com/go-openapi/analysis"

	"github.com/go-swagger/go-swagger/generator"
)

const (
	verboseFlag   = "verbose"
	noverboseFlag = "noverbose"
	minimalFlag   = "minimal"
	fullFlag      = "full"
)

// FlattenCmdOptions determines options to the flatten spec preprocessing.
type FlattenCmdOptions struct {
	WithExpand  bool     `description:"expands all $ref's in the spec (shorthand to --with-flatten=expand)" group:"shared" long:"with-expand"`
	WithFlatten []string `choice:"minimal"                                                                  choice:"full"  choice:"expand"    choice:"verbose" choice:"noverbose" choice:"remove-unused" choice:"keep-names" default:"minimal" default:"verbose" description:"flattens all $ref's in the spec" group:"shared" long:"with-flatten"`
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
		case verboseFlag:
			res.Verbose = true
			verboseIsSet = true
		case noverboseFlag:
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
		case fullFlag:
			if !minimalIsSet && !expandIsSet {
				// minimal flag takes precedence
				res.Minimal = false
				minimalIsSet = true
			}
		case minimalFlag:
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

// usageWithSpec builds the usage line of a code generation command, which takes the spec
// either with --spec or as its single argument.
//
// It replaces the "[<command>-OPTIONS]" placeholder the flags parser renders by default.
func usageWithSpec(command string) string {
	return fmt.Sprintf("[%s-OPTIONS] [spec]", command)
}

// specOptions determine how the swagger spec is located, loaded and preprocessed.
//
// They apply to every command that generates something out of a spec, whether it is go code or not.
type specOptions struct {
	FlattenCmdOptions

	Spec           flags.Filename `description:"the spec file to use (default swagger.{json,yml,yaml})" group:"shared" long:"spec"            short:"f"`
	SkipValidation bool           `description:"skips validation of spec prior to generation"           group:"shared" long:"skip-validation"`
	Restricted     bool           `description:"Use restricted http client for remote $ref"             group:"shared" long:"restricted"`
	Rooted         string         `description:"Local $ref resolution contained relative to root FS"    group:"shared" long:"rooted"`
}

func (s specOptions) apply(opts *generator.GenOpts) {
	opts.Spec = string(s.Spec)
	opts.ValidateSpec = !s.SkipValidation
	opts.FlattenOpts = s.SetFlattenOptions(opts.FlattenOpts)
	opts.Restricted = s.Restricted
	opts.Rooted = s.Rooted
}

// outputOptions determine where the generated artifacts are written and which templates render them.
//
// They apply to every command that generates something out of a spec, whether it is go code or not.
type outputOptions struct {
	Target                flags.Filename `default:"./"                                                                                     description:"the base directory for generating the files" group:"shared"                 long:"target" short:"t"`
	TemplateDir           flags.Filename `description:"alternative template override directory"                                            group:"shared"                                            long:"template-dir"            short:"T"`
	ConfigFile            flags.Filename `description:"configuration file to use for overriding template options"                          group:"shared"                                            long:"config-file"             short:"C"`
	AdditionalInitialisms []string       `description:"consecutive capitals that should be considered intialisms"                          group:"shared"                                            long:"additional-initialism"`
	AllowTemplateOverride bool           `description:"allows overriding protected templates"                                              group:"shared"                                            long:"allow-template-override"`
	DumpData              bool           `description:"when present dumps the json for the template generator instead of generating files" group:"shared"                                            long:"dump-data"`
	EnsureTarget          bool           `description:"Create the target directory if it does not already exist"                           group:"shared"                                            long:"ensure-target"`
}

func (o outputOptions) apply(opts *generator.GenOpts) {
	opts.Target = string(o.Target)
	opts.TemplateDir = string(o.TemplateDir)
	opts.AllowTemplateOverride = o.AllowTemplateOverride
	opts.DumpData = o.DumpData
	opts.WithExtraInitialisms = o.AdditionalInitialisms
	opts.EnsureTarget = o.EnsureTarget
}

// goCodegenOptions only bear on the go code that is generated.
//
// Commands that do not produce go source, such as "generate markdown", do not expose them.
type goCodegenOptions struct {
	Template            string         `choice:"stratoscale"                                                                         description:"load contributed templates" group:"shared"               long:"template"`
	CopyrightFile       flags.Filename `description:"copyright file used to add copyright header"                                    group:"shared"                           long:"copyright-file"        short:"r"`
	StrictResponders    bool           `description:"Use strict type for the handler return value"                                   long:"strict-responders"`
	ReturnErrors        bool           `description:"handlers explicitly return an error as the second value"                        group:"shared"                           long:"return-errors"         short:"e"`
	WithCustomFormatter bool           `description:"use faster custom contributed go import processing instead of the standard one" group:"shared"                           long:"with-custom-formatter"`
}

func (g goCodegenOptions) apply(opts *generator.GenOpts) {
	opts.Template = g.Template
	opts.Copyright = string(g.CopyrightFile)
	opts.StrictResponders = g.StrictResponders
	opts.ReturnErrors = g.ReturnErrors
	opts.WithCustomFormatter = g.WithCustomFormatter
}

// sharedOptions are the options common to all code generation commands.
type sharedOptions struct {
	specOptions
	outputOptions
	goCodegenOptions
	pluginOptions
}

func (s sharedOptions) apply(opts *generator.GenOpts) {
	s.specOptions.apply(opts)
	s.outputOptions.apply(opts)
	s.goCodegenOptions.apply(opts)
	s.pluginOptions.apply(opts)
}

// markdownSharedOptions are the shared options that bear on the generated markdown.
//
// The markdown command generates no go code, so it leaves out [goCodegenOptions].
type markdownSharedOptions struct {
	specOptions
	outputOptions
	pluginOptions
}

func (s markdownSharedOptions) apply(opts *generator.GenOpts) {
	s.specOptions.apply(opts)
	s.outputOptions.apply(opts)
	s.pluginOptions.apply(opts)
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

// specFromArgs resolves the spec passed as a command line argument.
//
// Code generation commands accept the spec either with --spec (-f) or as a single
// argument, like the validate, expand, flatten, mixin and diff commands do.
func specFromArgs(opts *generator.GenOpts, args []string) error {
	switch {
	case len(args) == 0:
		return nil

	case len(args) > 1:
		return fmt.Errorf(
			"too many arguments: this command takes at most one swagger spec, but got %d: %v",
			len(args), args,
		)

	case opts.Spec != "":
		return fmt.Errorf(
			"the swagger spec is specified twice: as --spec %q and as the argument %q. Use one or the other",
			opts.Spec, args[0],
		)

	default:
		opts.Spec = args[0]

		return nil
	}
}

func createSwagger(s sharedCommand, args []string) error {
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

	// the config layout (if any) is applied as overrides when the generator
	// finalizes the options in Prepare.
	opts := generator.NewGenOpts(generator.WithViper(cfg))
	s.apply(opts)

	if err = specFromArgs(opts, args); err != nil {
		return err
	}

	opts.Copyright, err = setCopyright(opts.Copyright)
	if err != nil {
		return fmt.Errorf("could not load copyright file: %w", err)
	}

	if opts.Template != "" {
		contribOptionsOverride(opts)
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

func setDebug(cfg *viper.Viper) {
	if os.Getenv("DEBUG") == "" && os.Getenv("SWAGGER_DEBUG") == "" {
		return
	}

	// viper config debug
	cfg.Debug()
}

func printImports(extras ...string) string {
	const allImports = 11
	imports := make([]string, 0, allImports+len(extras))
	imports = append(imports,
		"github.com/go-openapi/errors",
		"github.com/go-openapi/loads",
		"github.com/go-openapi/runtime",
		"github.com/go-openapi/spec",
		"github.com/go-openapi/strfmt",
		"github.com/go-openapi/swag/cmdutils",
		"github.com/go-openapi/swag/conv",
		"github.com/go-openapi/swag/jsonutils",
		"github.com/go-openapi/swag/netutils",
		"github.com/go-openapi/swag/stringutils",
		"github.com/go-openapi/swag/typeutils",
	)
	imports = append(imports, extras...)
	sort.Strings(imports)

	var w strings.Builder
	for _, pkg := range imports {
		if pkg == "" {
			continue
		}
		fmt.Fprintf(&w, "\t* %s\n", pkg)
	}

	return w.String()
}

func noticeImports(extras ...string) {
	log.Println(
		"Generation completed!",
		"\n",
		"For this generation to compile you need to have some packages in your go.mod.",
		"\n",
		printImports(extras...),
		"\n",
		"You can get these now with: go mod tidy",
	)
}
