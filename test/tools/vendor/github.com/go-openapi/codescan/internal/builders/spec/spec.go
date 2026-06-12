// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"go/ast"

	"github.com/go-openapi/codescan/internal/builders/operations"
	"github.com/go-openapi/codescan/internal/builders/parameters"
	"github.com/go-openapi/codescan/internal/builders/responses"
	"github.com/go-openapi/codescan/internal/builders/routes"
	"github.com/go-openapi/codescan/internal/builders/schema"
	"github.com/go-openapi/codescan/internal/parsers"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

type Builder struct {
	scanModels  bool
	input       *oaispec.Swagger
	ctx         *scanner.ScanCtx
	discovered  []*scanner.EntityDecl
	definitions map[string]oaispec.Schema
	responses   map[string]oaispec.Response
	operations  map[string]*oaispec.Operation
}

func NewBuilder(input *oaispec.Swagger, sc *scanner.ScanCtx, scanModels bool) *Builder {
	if input == nil {
		input = new(oaispec.Swagger)
		input.Swagger = "2.0"
	}

	if input.Paths == nil {
		input.Paths = new(oaispec.Paths)
	}
	if input.Definitions == nil {
		input.Definitions = make(map[string]oaispec.Schema)
	}
	if input.Responses == nil {
		input.Responses = make(map[string]oaispec.Response)
	}
	if input.Extensions == nil {
		input.Extensions = make(oaispec.Extensions)
	}

	return &Builder{
		ctx:         sc,
		input:       input,
		scanModels:  scanModels,
		operations:  collectOperationsFromInput(input),
		definitions: input.Definitions,
		responses:   input.Responses,
	}
}

func (s *Builder) Build() (*oaispec.Swagger, error) {
	// this initial scan step is skipped if !scanModels.
	// Discovered dependencies should however be resolved.
	if err := s.buildModels(); err != nil {
		return nil, err
	}

	if err := s.buildParameters(); err != nil {
		return nil, err
	}

	if err := s.buildResponses(); err != nil {
		return nil, err
	}

	// build definitions dictionary
	if err := s.buildDiscovered(); err != nil {
		return nil, err
	}

	if err := s.buildRoutes(); err != nil {
		return nil, err
	}

	if err := s.buildOperations(); err != nil {
		return nil, err
	}

	if err := s.buildMeta(); err != nil {
		return nil, err
	}

	if s.input.Swagger == "" {
		s.input.Swagger = "2.0"
	}

	return s.input, nil
}

func (s *Builder) buildDiscovered() error {
	// loop over discovered until all the items are in definitions
	keepGoing := len(s.discovered) > 0
	for keepGoing {
		var queue []*scanner.EntityDecl
		for _, d := range s.discovered {
			nm, _ := d.Names()
			if _, ok := s.definitions[nm]; !ok {
				queue = append(queue, d)
			}
		}
		s.discovered = nil
		for _, sd := range queue {
			if err := s.buildDiscoveredSchema(sd); err != nil {
				return err
			}
		}
		keepGoing = len(s.discovered) > 0
	}

	return nil
}

func (s *Builder) buildDiscoveredSchema(decl *scanner.EntityDecl) error {
	sb := schema.NewBuilder(s.ctx, decl)
	sb.SetDiscovered(s.discovered)
	if err := sb.Build(s.definitions); err != nil {
		return err
	}

	s.discovered = append(s.discovered, sb.PostDeclarations()...)

	return nil
}

func (s *Builder) buildMeta() error {
	// build swagger object
	for decl := range s.ctx.Meta() {
		if err := parsers.NewMetaParser(s.input).Parse(decl.Comments); err != nil {
			return err
		}
	}

	return nil
}

func (s *Builder) buildOperations() error {
	for pp := range s.ctx.Operations() {
		ob := operations.NewBuilder(s.ctx, pp, s.operations)
		if err := ob.Build(s.input.Paths); err != nil {
			return err
		}
	}

	return nil
}

func (s *Builder) buildRoutes() error {
	// build paths dictionary
	for pp := range s.ctx.Routes() {
		rb := routes.NewBuilder(
			s.ctx,
			pp,
			routes.Inputs{
				Responses:   s.responses,
				Operations:  s.operations,
				Definitions: s.definitions,
			},
		)
		if err := rb.Build(s.input.Paths); err != nil {
			return err
		}
	}

	return nil
}

func (s *Builder) buildResponses() error {
	// build responses dictionary
	for decl := range s.ctx.Responses() {
		rb := responses.NewBuilder(s.ctx, decl)
		if err := rb.Build(s.responses); err != nil {
			return err
		}
		s.discovered = append(s.discovered, rb.PostDeclarations()...)
	}

	return nil
}

func (s *Builder) buildParameters() error {
	// build parameters dictionary
	for decl := range s.ctx.Parameters() {
		pb := parameters.NewBuilder(s.ctx, decl)
		if err := pb.Build(s.operations); err != nil {
			return err
		}
		s.discovered = append(s.discovered, pb.PostDeclarations()...)
	}

	return nil
}

func (s *Builder) buildModels() error {
	// build models dictionary
	if !s.scanModels {
		return nil
	}

	for _, decl := range s.ctx.Models() {
		if err := s.buildDiscoveredSchema(decl); err != nil {
			return err
		}
	}

	return s.joinExtraModels()
}

func (s *Builder) joinExtraModels() error {
	l := s.ctx.NumExtraModels()
	if l == 0 {
		return nil
	}

	tmp := make(map[*ast.Ident]*scanner.EntityDecl, l)
	for k, v := range s.ctx.ExtraModels() {
		tmp[k] = v
		s.ctx.MoveExtraToModel(k)
	}

	// process extra models and see if there is any reference to a new extra one
	for _, decl := range tmp {
		if err := s.buildDiscoveredSchema(decl); err != nil {
			return err
		}
	}

	if s.ctx.NumExtraModels() > 0 {
		return s.joinExtraModels()
	}

	return nil
}

func collectOperationsFromInput(input *oaispec.Swagger) map[string]*oaispec.Operation {
	operations := make(map[string]*oaispec.Operation)
	if input == nil || input.Paths == nil {
		return operations
	}

	for _, pth := range input.Paths.Paths {
		if pth.Get != nil {
			operations[pth.Get.ID] = pth.Get
		}
		if pth.Post != nil {
			operations[pth.Post.ID] = pth.Post
		}
		if pth.Put != nil {
			operations[pth.Put.ID] = pth.Put
		}
		if pth.Patch != nil {
			operations[pth.Patch.ID] = pth.Patch
		}
		if pth.Delete != nil {
			operations[pth.Delete.ID] = pth.Delete
		}
		if pth.Head != nil {
			operations[pth.Head.ID] = pth.Head
		}
		if pth.Options != nil {
			operations[pth.Options.ID] = pth.Options
		}
	}

	return operations
}
