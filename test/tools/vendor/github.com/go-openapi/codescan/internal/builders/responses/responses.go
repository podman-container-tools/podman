// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package responses

import (
	"fmt"
	"go/types"

	"github.com/go-openapi/codescan/internal/builders/resolvers"
	"github.com/go-openapi/codescan/internal/builders/schema"
	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/logger"
	"github.com/go-openapi/codescan/internal/parsers"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

type ResponseBuilder struct {
	ctx       *scanner.ScanCtx
	decl      *scanner.EntityDecl
	postDecls []*scanner.EntityDecl
}

func NewBuilder(ctx *scanner.ScanCtx, decl *scanner.EntityDecl) *ResponseBuilder {
	return &ResponseBuilder{
		ctx:  ctx,
		decl: decl,
	}
}

func (r *ResponseBuilder) Build(responses map[string]oaispec.Response) error {
	// check if there is a swagger:response tag that is followed by one or more words,
	// these words are the ids of the operations this parameter struct applies to
	// once type name is found convert it to a schema, by looking up the schema in the
	// parameters dictionary that got passed into this parse method

	name, _ := r.decl.ResponseNames()
	response := responses[name]
	logger.DebugLogf(r.ctx.Debug(), "building response: %s", name)

	// analyze doc comment for the model
	sp := parsers.NewSectionedParser(
		parsers.WithSetDescription(func(lines []string) {
			response.Description = parsers.JoinDropLast(lines)
		}),
	)
	if err := sp.Parse(r.decl.Comments); err != nil {
		return err
	}

	// analyze struct body for fields etc
	// each exported struct field:
	// * gets a type mapped to a go primitive
	// * perhaps gets a format
	// * has to document the validations that apply for the type and the field
	// * when the struct field points to a model it becomes a ref: #/definitions/ModelName
	// * comments that aren't tags is used as the description
	if err := r.buildFromType(r.decl.ObjType(), &response, make(map[string]bool)); err != nil {
		return err
	}
	responses[name] = response

	return nil
}

func (r *ResponseBuilder) PostDeclarations() []*scanner.EntityDecl {
	return r.postDecls
}

func (r *ResponseBuilder) buildFromField(fld *types.Var, tpe types.Type, typable ifaces.SwaggerTypable, seen map[string]bool) error {
	logger.DebugLogf(r.ctx.Debug(), "build from field %s: %T", fld.Name(), tpe)

	switch ftpe := tpe.(type) {
	case *types.Basic:
		return resolvers.SwaggerSchemaForType(ftpe.Name(), typable)
	case *types.Struct:
		return r.buildFromFieldStruct(ftpe, typable)
	case *types.Pointer:
		return r.buildFromField(fld, ftpe.Elem(), typable, seen)
	case *types.Interface:
		return r.buildFromFieldInterface(ftpe, typable)
	case *types.Array:
		return r.buildFromField(fld, ftpe.Elem(), typable.Items(), seen)
	case *types.Slice:
		return r.buildFromField(fld, ftpe.Elem(), typable.Items(), seen)
	case *types.Map:
		return r.buildFromFieldMap(ftpe, typable)
	case *types.Named:
		return r.buildNamedField(ftpe, typable)
	case *types.Alias:
		logger.DebugLogf(r.ctx.Debug(), "alias(responses.buildFromField): got alias %v to %v", ftpe, ftpe.Rhs())
		return r.buildFieldAlias(ftpe, typable, fld, seen)
	default:
		return fmt.Errorf("unknown type for %s: %T: %w", fld.String(), fld.Type(), ErrResponses)
	}
}

func (r *ResponseBuilder) buildFromFieldStruct(ftpe *types.Struct, typable ifaces.SwaggerTypable) error {
	sb := schema.NewBuilder(r.ctx, r.decl)
	if err := sb.BuildFromType(ftpe, typable); err != nil {
		return err
	}

	r.postDecls = append(r.postDecls, sb.PostDeclarations()...)

	return nil
}

func (r *ResponseBuilder) buildFromFieldMap(ftpe *types.Map, typable ifaces.SwaggerTypable) error {
	sch := new(oaispec.Schema)
	typable.Schema().Typed("object", "").AdditionalProperties = &oaispec.SchemaOrBool{
		Schema: sch,
	}

	sb := schema.NewBuilder(r.ctx, r.decl)
	if err := sb.BuildFromType(ftpe.Elem(), schema.NewTypable(sch, typable.Level()+1, r.ctx.SkipExtensions())); err != nil {
		return err
	}

	r.postDecls = append(r.postDecls, sb.PostDeclarations()...)

	return nil
}

func (r *ResponseBuilder) buildFromFieldInterface(tpe types.Type, typable ifaces.SwaggerTypable) error {
	sb := schema.NewBuilder(r.ctx, r.decl)
	if err := sb.BuildFromType(tpe, typable); err != nil {
		return err
	}

	r.postDecls = append(r.postDecls, sb.PostDeclarations()...)

	return nil
}

func (r *ResponseBuilder) buildFromType(otpe types.Type, resp *oaispec.Response, seen map[string]bool) error {
	switch tpe := otpe.(type) {
	case *types.Pointer:
		return r.buildFromType(tpe.Elem(), resp, seen)
	case *types.Named:
		return r.buildNamedType(tpe, resp, seen)
	case *types.Alias:
		logger.DebugLogf(r.ctx.Debug(), "alias(responses.buildFromType): got alias %v to %v", tpe, tpe.Rhs())
		return r.buildAlias(tpe, resp, seen)
	default:
		return fmt.Errorf("anonymous types are currently not supported for responses: %w", ErrResponses)
	}
}

func (r *ResponseBuilder) buildNamedType(tpe *types.Named, resp *oaispec.Response, seen map[string]bool) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) || resolvers.IsStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a responses section definition: %w", o.Name(), ErrResponses)
	}
	resolvers.MustNotBeABuiltinType(o)

	switch stpe := o.Type().Underlying().(type) { // TODO(fred): this is wrong without checking for aliases?
	case *types.Struct:
		logger.DebugLogf(r.ctx.Debug(), "build from type %s: %T", o.Name(), tpe)
		if decl, found := r.ctx.DeclForType(o.Type()); found {
			return r.buildFromStruct(decl, stpe, resp, seen)
		}
		return r.buildFromStruct(r.decl, stpe, resp, seen)

	default:
		if decl, found := r.ctx.DeclForType(o.Type()); found {
			var sch oaispec.Schema
			typable := schema.NewTypable(&sch, 0, r.ctx.SkipExtensions())

			d := decl.Obj()
			if resolvers.IsStdTime(d) {
				typable.Typed("string", "date-time")
				return nil
			}
			if sfnm, isf := parsers.StrfmtName(decl.Comments); isf {
				typable.Typed("string", sfnm)
				return nil
			}
			sb := schema.NewBuilder(r.ctx, decl)
			sb.InferNames()
			if err := sb.BuildFromType(tpe.Underlying(), typable); err != nil {
				return err
			}
			resp.WithSchema(&sch)
			r.postDecls = append(r.postDecls, sb.PostDeclarations()...)
			return nil
		}
		return fmt.Errorf("responses can only be structs, did you mean for %s to be the response body?: %w", tpe.String(), ErrResponses)
	}
}

func (r *ResponseBuilder) buildAlias(tpe *types.Alias, resp *oaispec.Response, seen map[string]bool) error {
	// panic("yay")
	o := tpe.Obj()
	if resolvers.IsAny(o) || resolvers.IsStdError(o) {
		// wrong: TODO(fred): see what object exactly we want to build here - figure out with specific tests
		return fmt.Errorf("%s type not supported in the context of a responses section definition: %w", o.Name(), ErrResponses)
	}
	resolvers.MustNotBeABuiltinType(o)
	resolvers.MustHaveRightHandSide(tpe)

	rhs := tpe.Rhs()

	// If transparent aliases are enabled, use the underlying type directly without creating a definition
	if r.ctx.TransparentAliases() {
		return r.buildFromType(rhs, resp, seen)
	}

	decl, ok := r.ctx.FindModel(o.Pkg().Path(), o.Name())
	if !ok {
		return fmt.Errorf("can't find source file for aliased type: %v -> %v: %w", tpe, rhs, ErrResponses)
	}
	r.postDecls = append(r.postDecls, decl) // mark the left-hand side as discovered

	if !r.ctx.RefAliases() {
		// expand alias
		unaliased := types.Unalias(tpe)
		return r.buildFromType(unaliased.Underlying(), resp, seen)
	}

	switch rtpe := rhs.(type) {
	// load declaration for named unaliased type
	case *types.Named:
		o := rtpe.Obj()
		if o.Pkg() == nil {
			break // builtin
		}

		typable := schema.NewTypable(&oaispec.Schema{}, 0, r.ctx.SkipExtensions())
		return r.makeRef(decl, typable)
	case *types.Alias:
		o := rtpe.Obj()
		if o.Pkg() == nil {
			break // builtin
		}

		typable := schema.NewTypable(&oaispec.Schema{}, 0, r.ctx.SkipExtensions())

		return r.makeRef(decl, typable)
	}

	return r.buildFromType(rhs, resp, seen)
}

func (r *ResponseBuilder) buildNamedField(ftpe *types.Named, typable ifaces.SwaggerTypable) error {
	decl, found := r.ctx.DeclForType(ftpe.Obj().Type())
	if !found {
		return fmt.Errorf("unable to find package and source file for: %s: %w", ftpe.String(), ErrResponses)
	}

	d := decl.Obj()
	if resolvers.IsStdTime(d) {
		typable.Typed("string", "date-time")
		return nil
	}

	if sfnm, isf := parsers.StrfmtName(decl.Comments); isf {
		typable.Typed("string", sfnm)
		return nil
	}

	sb := schema.NewBuilder(r.ctx, decl)
	sb.InferNames()
	if err := sb.BuildFromType(decl.ObjType(), typable); err != nil {
		return err
	}

	r.postDecls = append(r.postDecls, sb.PostDeclarations()...)

	return nil
}

func (r *ResponseBuilder) buildFieldAlias(tpe *types.Alias, typable ifaces.SwaggerTypable, fld *types.Var, seen map[string]bool) error {
	_ = fld
	_ = seen
	o := tpe.Obj()
	if resolvers.IsAny(o) {
		// e.g. Field interface{} or Field any
		_ = typable.Schema()

		return nil // just leave an empty schema
	}

	// If transparent aliases are enabled, use the underlying type directly without creating a definition
	if r.ctx.TransparentAliases() {
		sb := schema.NewBuilder(r.ctx, r.decl)
		if err := sb.BuildFromType(tpe.Rhs(), typable); err != nil {
			return err
		}
		r.postDecls = append(r.postDecls, sb.PostDeclarations()...)
		return nil
	}

	decl, ok := r.ctx.FindModel(o.Pkg().Path(), o.Name())
	if !ok {
		return fmt.Errorf("can't find source file for aliased type: %v: %w", tpe, ErrResponses)
	}
	r.postDecls = append(r.postDecls, decl) // mark the left-hand side as discovered

	return r.makeRef(decl, typable)
}

func (r *ResponseBuilder) buildFromStruct(decl *scanner.EntityDecl, tpe *types.Struct, resp *oaispec.Response, seen map[string]bool) error {
	if tpe.NumFields() == 0 {
		return nil
	}

	for fld := range tpe.Fields() {
		if fld.Embedded() {
			if err := r.buildFromType(fld.Type(), resp, seen); err != nil {
				return err
			}
			continue
		}
		if fld.Anonymous() {
			logger.DebugLogf(r.ctx.Debug(), "skipping anonymous field")
			continue
		}

		if err := r.processResponseField(fld, decl, resp, seen); err != nil {
			return err
		}
	}

	for k := range resp.Headers {
		if !seen[k] {
			delete(resp.Headers, k)
		}
	}
	return nil
}

func (r *ResponseBuilder) processResponseField(fld *types.Var, decl *scanner.EntityDecl, resp *oaispec.Response, seen map[string]bool) error {
	if !fld.Exported() {
		return nil
	}

	afld := resolvers.FindASTField(decl.File, fld.Pos())
	if afld == nil {
		logger.DebugLogf(r.ctx.Debug(), "can't find source associated with %s", fld.String())
		return nil
	}

	if parsers.Ignored(afld.Doc) {
		logger.DebugLogf(r.ctx.Debug(), "field %v is deliberately ignored", fld)
		return nil
	}

	name, ignore, _, _, err := resolvers.ParseJSONTag(afld)
	if err != nil {
		return err
	}
	if ignore {
		return nil
	}

	// scan for param location first, this changes some behavior down the line
	in, _ := parsers.ParamLocation(afld.Doc)
	ps := resp.Headers[name]

	// support swagger:file for response
	// An API operation can return a file, such as an image or PDF. In this case,
	// define the response schema with type: file and specify the appropriate MIME types in the produces section.
	if afld.Doc != nil && parsers.FileParam(afld.Doc) {
		resp.Schema = &oaispec.Schema{}
		resp.Schema.Typed("file", "")
	} else {
		logger.DebugLogf(r.ctx.Debug(), "build response %v (%v) (not a file)", fld, fld.Type())
		if err := r.buildFromField(fld, fld.Type(), responseTypable{in, &ps, resp, r.ctx.SkipExtensions()}, seen); err != nil {
			return err
		}
	}

	if strfmtName, ok := parsers.StrfmtName(afld.Doc); ok {
		ps.Typed("string", strfmtName)
	}

	taggers, err := setupResponseHeaderTaggers(&ps, name, afld)
	if err != nil {
		return err
	}

	sp := parsers.NewSectionedParser(
		parsers.WithSetDescription(func(lines []string) { ps.Description = parsers.JoinDropLast(lines) }),
		parsers.WithTaggers(taggers...),
	)

	if err := sp.Parse(afld.Doc); err != nil {
		return err
	}

	if in != "body" {
		seen[name] = true
		if resp.Headers == nil {
			resp.Headers = make(map[string]oaispec.Header)
		}
		resp.Headers[name] = ps
	}

	return nil
}

func (r *ResponseBuilder) makeRef(decl *scanner.EntityDecl, prop ifaces.SwaggerTypable) error {
	nm, _ := decl.Names()
	ref, err := oaispec.NewRef("#/definitions/" + nm)
	if err != nil {
		return err
	}

	prop.SetRef(ref)
	r.postDecls = append(r.postDecls, decl) // mark the $ref target as discovered

	return nil
}
