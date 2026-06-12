// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"fmt"

	"github.com/go-openapi/codescan/internal/builders/operations"
	"github.com/go-openapi/codescan/internal/parsers"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

type Builder struct {
	ctx         *scanner.ScanCtx
	route       parsers.ParsedPathContent
	responses   map[string]oaispec.Response
	operations  map[string]*oaispec.Operation
	parameters  []*oaispec.Parameter
	definitions map[string]oaispec.Schema
}

type Inputs struct {
	Responses   map[string]oaispec.Response
	Operations  map[string]*oaispec.Operation
	Definitions map[string]oaispec.Schema
}

func NewBuilder(ctx *scanner.ScanCtx, route parsers.ParsedPathContent, inputs Inputs) *Builder {
	return &Builder{
		ctx:         ctx,
		route:       route,
		responses:   inputs.Responses,
		operations:  inputs.Operations,
		definitions: inputs.Definitions,
	}
}

func (r *Builder) Build(tgt *oaispec.Paths) error {
	pthObj := tgt.Paths[r.route.Path]
	op := operations.SetPathOperation(
		r.route.Method, r.route.ID,
		&pthObj, r.operations[r.route.ID],
	)
	op.Tags = r.route.Tags

	sp := parsers.NewSectionedParser(
		parsers.WithSetTitle(func(lines []string) { op.Summary = parsers.JoinDropLast(lines) }),
		parsers.WithSetDescription(func(lines []string) { op.Description = parsers.JoinDropLast(lines) }),
		parsers.WithTaggers(r.routeTaggers(op)...),
	)

	if err := sp.Parse(r.route.Remaining); err != nil {
		return fmt.Errorf("operation (%s): %w", op.ID, err)
	}

	if tgt.Paths == nil {
		tgt.Paths = make(map[string]oaispec.PathItem)
	}
	tgt.Paths[r.route.Path] = pthObj

	return nil
}
