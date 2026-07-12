// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generate

import (
	"log"
	"strings"

	"github.com/go-swagger/go-swagger/generator"
)

type serverOptions struct {
	ServerPackage         string `default:"restapi" description:"the package to save the server specific code"                                               long:"server-package"         short:"s"`
	MainTarget            string `default:""        description:"the location of the generated main. Defaults to cmd/{name}-server"                          long:"main-package"           short:""`
	ImplementationPackage string `default:""        description:"the location of the backend implementation of the server, which will be autowired with api" long:"implementation-package" short:""`
}

func (cs serverOptions) apply(opts *generator.GenOpts) {
	opts.ServerPackage = cs.ServerPackage
}

// Server the command to generate an entire server application.
type Server struct {
	WithShared
	WithModels
	WithOperations

	serverOptions
	schemeOptions
	mediaOptions

	SkipModels             bool   `description:"no models will be generated when this flag is specified"           long:"skip-models"`
	SkipOperations         bool   `description:"no operations will be generated when this flag is specified"       long:"skip-operations"`
	SkipSupport            bool   `description:"no supporting files will be generated when this flag is specified" long:"skip-support"`
	ExcludeMain            bool   `description:"exclude main function, so just generate the library"               long:"exclude-main"`
	ExcludeSpec            bool   `description:"don't embed the swagger specification"                             long:"exclude-spec"`
	FlagStrategy           string `choice:"go-flags"                                                               choice:"pflag"                 choice:"flag"    default:"go-flags"                                      description:"the strategy to provide flags for the server" long:"flag-strategy"`
	CompatibilityMode      string `choice:"modern"                                                                 choice:"intermediate"          default:"modern" description:"the compatibility mode for the tls server" long:"compatibility-mode"`
	RegenerateConfigureAPI bool   `description:"Force regeneration of configureapi.go"                             long:"regenerate-configureapi"`

	Name string `description:"the name of the application, defaults to a mangled value of info.title" long:"name" short:"A"`
	// TODO(fredbi): CmdName string `long:"cmd-name" short:"A" description:"the name of the server command, when main is generated (defaults to {name}-server)"`

	// deprecated flags
	WithContext bool `description:"handlers get a context as first arg (deprecated)" long:"with-context"`
}

// Execute runs this command.
func (s *Server) Execute(_ []string) error {
	return createSwagger(s)
}

// apply options.
func (s *Server) apply(opts *generator.GenOpts) {
	if s.WithContext {
		log.Printf("warning: deprecated option --with-context is ignored")
		s.WithContext = false
	}

	s.Shared.apply(opts)
	s.Models.apply(opts)
	s.Operations.apply(opts)
	s.serverOptions.apply(opts)
	s.schemeOptions.apply(opts)
	s.mediaOptions.apply(opts)

	opts.IncludeModel = !s.SkipModels
	opts.IncludeValidator = !s.SkipModels
	opts.IncludeHandler = !s.SkipOperations
	opts.IncludeParameters = !s.SkipOperations
	opts.IncludeResponses = !s.SkipOperations
	opts.IncludeURLBuilder = !s.SkipOperations
	opts.IncludeSupport = !s.SkipSupport
	opts.IncludeMain = !s.ExcludeMain
	opts.ExcludeSpec = s.ExcludeSpec
	opts.FlagStrategy = s.FlagStrategy
	opts.CompatibilityMode = s.CompatibilityMode
	opts.RegenerateConfigureAPI = s.RegenerateConfigureAPI

	opts.Name = s.Name
	opts.MainPackage = s.MainTarget

	opts.ImplementationPackage = s.ImplementationPackage
}

func (s *Server) generate(opts *generator.GenOpts) error {
	return generator.GenerateServer(s.Name, s.Models.Models, s.Operations.Operations, opts)
}

func (s Server) log(_ string) {
	var flagsPackage string
	switch {
	case strings.HasPrefix(s.FlagStrategy, "pflag"):
		flagsPackage = "github.com/spf13/pflag"
	case strings.HasPrefix(s.FlagStrategy, "flag"):
		flagsPackage = "flag"
	default:
		flagsPackage = "github.com/jessevdk/go-flags"
	}

	log.Println(`Generation completed!

For this generation to compile you need to have some packages in your go.mod:

	* github.com/go-openapi/runtime
	* ` + flagsPackage + `

You can get these now with: go mod tidy`)
}
