// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package operations

import (
	"fmt"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/common"
	"github.com/go-openapi/codescan/internal/parsers"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// Builder constructs OAS v2 operation entries for one `swagger:operation` annotation.
//
// Embeds *common.Builder for shared state (Ctx, ParseBlocks cache, diagnostic sink).
// Decl is unused because operations build off a path annotation, not a declaration; the MakeRef /
// Decl-anchored helpers must not be called.
//
// # Details
//
// See [§builder](./README.md#builder) for the Build orchestration and the path-item-slot reuse
// semantics.
type Builder struct {
	*common.Builder

	path       parsers.ParsedPathContent
	operations map[string]*oaispec.Operation
}

func NewBuilder(ctx *scanner.ScanCtx, pth parsers.ParsedPathContent, operations map[string]*oaispec.Operation) *Builder {
	return &Builder{
		Builder:    common.New(ctx, nil),
		path:       pth,
		operations: operations,
	}
}

func (o *Builder) Build(tgt *oaispec.Paths) error {
	o.WarnStrippedPathRegex(o.path.Pos, o.path.StrippedParams)

	pthObj := tgt.Paths[o.path.Path]

	op := setPathOperation(
		o.path.Method, o.path.ID,
		&pthObj, o.operations[o.path.ID])
	op.Tags = o.path.Tags

	if err := o.applyBlockToOperation(op); err != nil {
		return fmt.Errorf("operation (%s): %w", op.ID, err)
	}

	if tgt.Paths == nil {
		tgt.Paths = make(map[string]oaispec.PathItem)
	}

	tgt.Paths[o.path.Path] = pthObj

	// Cross-ref linkage: anchor the operation node to its swagger:operation annotation;
	// parameters/responses under it resolve to this node.
	if o.Ctx.OriginEnabled() && o.path.Pos.IsValid() {
		o.Ctx.RecordOrigin(
			scanner.JSONPointer("paths", o.path.Path, strings.ToLower(o.path.Method)),
			o.Ctx.PosOf(o.path.Pos),
		)
	}

	return nil
}

func SetPathOperation(method, id string, pthObj *oaispec.PathItem, op *oaispec.Operation) *oaispec.Operation {
	return setPathOperation(method, id, pthObj, op)
}

// setPathOperation lands op on the HTTP-verb slot of pthObj.
//
// Returns the operation now occupying the slot — either the reused existing one or op itself.
// Unrecognised methods leave pthObj untouched and return op verbatim.
//
// # Details
//
// See [§path-operation-slot](./README.md#path-operation-slot) for the slot-reuse contract, the
// nil-op allocation behaviour, and the public `SetPathOperation` re-export used by sibling
// packages.
func setPathOperation(method, id string, pthObj *oaispec.PathItem, op *oaispec.Operation) *oaispec.Operation {
	if op == nil {
		op = new(oaispec.Operation)
		op.ID = id
	}

	switch strings.ToUpper(method) {
	case "GET":
		if pthObj.Get == nil || pthObj.Get.ID != id {
			pthObj.Get = op
		}
		return pthObj.Get
	case "POST":
		if pthObj.Post == nil || pthObj.Post.ID != id {
			pthObj.Post = op
		}
		return pthObj.Post
	case "PUT":
		if pthObj.Put == nil || pthObj.Put.ID != id {
			pthObj.Put = op
		}
		return pthObj.Put
	case "PATCH":
		if pthObj.Patch == nil || pthObj.Patch.ID != id {
			pthObj.Patch = op
		}
		return pthObj.Patch
	case "HEAD":
		if pthObj.Head == nil || pthObj.Head.ID != id {
			pthObj.Head = op
		}
		return pthObj.Head
	case "DELETE":
		if pthObj.Delete == nil || pthObj.Delete.ID != id {
			pthObj.Delete = op
		}
		return pthObj.Delete
	case "OPTIONS":
		if pthObj.Options == nil || pthObj.Options.ID != id {
			pthObj.Options = op
		}
		return pthObj.Options
	}

	return op
}
