// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package operations

import (
	"fmt"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

type Builder struct {
	ctx        *scanner.ScanCtx
	path       parsers.ParsedPathContent
	operations map[string]*oaispec.Operation
}

func NewBuilder(ctx *scanner.ScanCtx, pth parsers.ParsedPathContent, operations map[string]*oaispec.Operation) *Builder {
	return &Builder{
		ctx:        ctx,
		path:       pth,
		operations: operations,
	}
}

func (o *Builder) Build(tgt *oaispec.Paths) error {
	pthObj := tgt.Paths[o.path.Path]

	op := setPathOperation(
		o.path.Method, o.path.ID,
		&pthObj, o.operations[o.path.ID])
	op.Tags = o.path.Tags
	sp := parsers.NewYAMLSpecScanner(
		func(lines []string) { op.Summary = parsers.JoinDropLast(lines) },     // setTitle
		func(lines []string) { op.Description = parsers.JoinDropLast(lines) }, // setDescription
	)

	if err := sp.Parse(o.path.Remaining); err != nil {
		return fmt.Errorf("operation (%s): %w", op.ID, err)
	}
	if err := sp.UnmarshalSpec(op.UnmarshalJSON); err != nil {
		return fmt.Errorf("operation (%s): %w", op.ID, err)
	}

	if tgt.Paths == nil {
		tgt.Paths = make(map[string]oaispec.PathItem)
	}

	tgt.Paths[o.path.Path] = pthObj

	return nil
}

// assignOrReuse either reuses an existing operation (if the ID matches)
// or assigns op to the slot.
//
// TODO(claude): rewrite without double indirection.
func assignOrReuse(slot **oaispec.Operation, op *oaispec.Operation, id string) *oaispec.Operation {
	if *slot != nil && id == (*slot).ID {
		return *slot
	}
	*slot = op
	return op
}

func SetPathOperation(method, id string, pthObj *oaispec.PathItem, op *oaispec.Operation) *oaispec.Operation {
	return setPathOperation(method, id, pthObj, op)
}

func setPathOperation(method, id string, pthObj *oaispec.PathItem, op *oaispec.Operation) *oaispec.Operation {
	if op == nil {
		op = new(oaispec.Operation)
		op.ID = id
	}

	switch strings.ToUpper(method) {
	case "GET":
		op = assignOrReuse(&pthObj.Get, op, id)
	case "POST":
		op = assignOrReuse(&pthObj.Post, op, id)
	case "PUT":
		op = assignOrReuse(&pthObj.Put, op, id)
	case "PATCH":
		op = assignOrReuse(&pthObj.Patch, op, id)
	case "HEAD":
		op = assignOrReuse(&pthObj.Head, op, id)
	case "DELETE":
		op = assignOrReuse(&pthObj.Delete, op, id)
	case "OPTIONS":
		op = assignOrReuse(&pthObj.Options, op, id)
	}

	return op
}
