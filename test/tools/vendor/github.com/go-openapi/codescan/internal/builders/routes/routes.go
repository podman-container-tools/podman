// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"fmt"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/common"
	"github.com/go-openapi/codescan/internal/builders/operations"
	"github.com/go-openapi/codescan/internal/parsers"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// Builder constructs OAS v2 path entries for one `swagger:route` annotation.
//
// It embeds *common.Builder for shared state (Ctx, ParseBlocks cache, diagnostic sink).
// The embedded Decl is left nil — routes build off a path annotation, not a declaration — so
// MakeRef and other Decl-anchored helpers must not be called.
type Builder struct {
	*common.Builder

	route      parsers.ParsedPathContent
	responses  map[string]oaispec.Response
	operations map[string]*oaispec.Operation
	// parameters  []*oaispec.Parameter // shared parameters - unsupported for now
	definitions map[string]oaispec.Schema
}

type Inputs struct {
	Responses   map[string]oaispec.Response
	Operations  map[string]*oaispec.Operation
	Definitions map[string]oaispec.Schema
}

func NewBuilder(ctx *scanner.ScanCtx, route parsers.ParsedPathContent, inputs Inputs) *Builder {
	return &Builder{
		Builder:     common.New(ctx, nil),
		route:       route,
		responses:   inputs.Responses,
		operations:  inputs.Operations,
		definitions: inputs.Definitions,
	}
}

func (r *Builder) Build(tgt *oaispec.Paths) error {
	r.WarnStrippedPathRegex(r.route.Pos, r.route.StrippedParams)

	pthObj := tgt.Paths[r.route.Path]
	op := operations.SetPathOperation(
		r.route.Method, r.route.ID,
		&pthObj, r.operations[r.route.ID],
	)
	op.Tags = r.route.Tags

	if err := r.applyBlockToRoute(op); err != nil {
		return fmt.Errorf("operation (%s): %w", op.ID, err)
	}

	if tgt.Paths == nil {
		tgt.Paths = make(map[string]oaispec.PathItem)
	}
	tgt.Paths[r.route.Path] = pthObj

	// Cross-ref linkage: anchor the operation node to its swagger:route annotation;
	// parameters/responses under it resolve to this node.
	if r.Ctx.OriginEnabled() && r.route.Pos.IsValid() {
		r.Ctx.RecordOrigin(
			scanner.JSONPointer("paths", r.route.Path, strings.ToLower(r.route.Method)),
			r.Ctx.PosOf(r.route.Pos),
		)
	}

	return nil
}
