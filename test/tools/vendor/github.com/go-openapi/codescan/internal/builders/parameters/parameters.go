// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parameters

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

const inBody = "body"

type ParameterBuilder struct {
	ctx       *scanner.ScanCtx
	decl      *scanner.EntityDecl
	postDecls []*scanner.EntityDecl
}

func NewBuilder(ctx *scanner.ScanCtx, decl *scanner.EntityDecl) *ParameterBuilder {
	return &ParameterBuilder{
		ctx:  ctx,
		decl: decl,
	}
}

func (p *ParameterBuilder) Build(operations map[string]*oaispec.Operation) error {
	// check if there is a swagger:parameters tag that is followed by one or more words,
	// these words are the ids of the operations this parameter struct applies to
	// once type name is found convert it to a schema, by looking up the schema in the
	// parameters dictionary that got passed into this parse method
	for _, opid := range p.decl.OperationIDs() {
		operation, ok := operations[opid]
		if !ok {
			operation = new(oaispec.Operation)
			operations[opid] = operation
			operation.ID = opid
		}
		logger.DebugLogf(p.ctx.Debug(), "building parameters for: %s", opid)

		// analyze struct body for fields etc
		// each exported struct field:
		// * gets a type mapped to a go primitive
		// * perhaps gets a format
		// * has to document the validations that apply for the type and the field
		// * when the struct field points to a model it becomes a ref: #/definitions/ModelName
		// * comments that aren't tags is used as the description
		if err := p.buildFromType(p.decl.ObjType(), operation, make(map[string]oaispec.Parameter)); err != nil {
			return err
		}
	}

	return nil
}

func (p *ParameterBuilder) PostDeclarations() []*scanner.EntityDecl {
	return p.postDecls
}

func (p *ParameterBuilder) buildFromType(otpe types.Type, op *oaispec.Operation, seen map[string]oaispec.Parameter) error {
	switch tpe := otpe.(type) {
	case *types.Pointer:
		return p.buildFromType(tpe.Elem(), op, seen)
	case *types.Named:
		return p.buildNamedType(tpe, op, seen)
	case *types.Alias:
		logger.DebugLogf(p.ctx.Debug(), "alias(parameters.buildFromType): got alias %v to %v", tpe, tpe.Rhs())
		return p.buildAlias(tpe, op, seen)
	default:
		return fmt.Errorf("unhandled type (%T): %s: %w", otpe, tpe.String(), ErrParameters)
	}
}

func (p *ParameterBuilder) buildNamedType(tpe *types.Named, op *oaispec.Operation, seen map[string]oaispec.Parameter) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) || resolvers.IsStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a parameters section definition: %w", o.Name(), ErrParameters)
	}
	resolvers.MustNotBeABuiltinType(o)

	switch stpe := o.Type().Underlying().(type) {
	case *types.Struct:
		logger.DebugLogf(p.ctx.Debug(), "build from named type %s: %T", o.Name(), tpe)
		if decl, found := p.ctx.DeclForType(o.Type()); found {
			return p.buildFromStruct(decl, stpe, op, seen)
		}

		return p.buildFromStruct(p.decl, stpe, op, seen)
	default:
		return fmt.Errorf("unhandled type (%T): %s: %w", stpe, o.Type().Underlying().String(), ErrParameters)
	}
}

func (p *ParameterBuilder) buildAlias(tpe *types.Alias, op *oaispec.Operation, seen map[string]oaispec.Parameter) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) || resolvers.IsStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a parameters section definition: %w", o.Name(), ErrParameters)
	}
	resolvers.MustNotBeABuiltinType(o)
	resolvers.MustHaveRightHandSide(tpe)

	rhs := tpe.Rhs()

	// If transparent aliases are enabled, use the underlying type directly without creating a definition
	if p.ctx.TransparentAliases() {
		return p.buildFromType(rhs, op, seen)
	}

	decl, ok := p.ctx.FindModel(o.Pkg().Path(), o.Name())
	if !ok {
		return fmt.Errorf("can't find source file for aliased type: %v -> %v: %w", tpe, rhs, ErrParameters)
	}
	p.postDecls = append(p.postDecls, decl) // mark the left-hand side as discovered

	switch rtpe := rhs.(type) {
	// load declaration for named unaliased type
	case *types.Named:
		o := rtpe.Obj()
		if o.Pkg() == nil {
			break // builtin
		}
		decl, found := p.ctx.FindModel(o.Pkg().Path(), o.Name())
		if !found {
			return fmt.Errorf("can't find source file for target type of alias: %v -> %v: %w", tpe, rtpe, ErrParameters)
		}
		p.postDecls = append(p.postDecls, decl)
	case *types.Alias:
		o := rtpe.Obj()
		if o.Pkg() == nil {
			break // builtin
		}
		decl, found := p.ctx.FindModel(o.Pkg().Path(), o.Name())
		if !found {
			return fmt.Errorf("can't find source file for target type of alias: %v -> %v: %w", tpe, rtpe, ErrParameters)
		}
		p.postDecls = append(p.postDecls, decl)
	}

	return p.buildFromType(rhs, op, seen)
}

func (p *ParameterBuilder) buildFromField(fld *types.Var, tpe types.Type, typable ifaces.SwaggerTypable, seen map[string]oaispec.Parameter) error {
	logger.DebugLogf(p.ctx.Debug(), "build from field %s: %T", fld.Name(), tpe)

	switch ftpe := tpe.(type) {
	case *types.Basic:
		return resolvers.SwaggerSchemaForType(ftpe.Name(), typable)
	case *types.Struct:
		return p.buildFromFieldStruct(ftpe, typable)
	case *types.Pointer:
		return p.buildFromField(fld, ftpe.Elem(), typable, seen)
	case *types.Interface:
		return p.buildFromFieldInterface(ftpe, typable)
	case *types.Array:
		return p.buildFromField(fld, ftpe.Elem(), typable.Items(), seen)
	case *types.Slice:
		return p.buildFromField(fld, ftpe.Elem(), typable.Items(), seen)
	case *types.Map:
		return p.buildFromFieldMap(ftpe, typable)
	case *types.Named:
		return p.buildNamedField(ftpe, typable)
	case *types.Alias:
		logger.DebugLogf(p.ctx.Debug(), "alias(parameters.buildFromField): got alias %v to %v", ftpe, ftpe.Rhs())
		return p.buildFieldAlias(ftpe, typable, fld, seen)
	default:
		return fmt.Errorf("unknown type for %s: %T: %w", fld.String(), fld.Type(), ErrParameters)
	}
}

func (p *ParameterBuilder) buildFromFieldStruct(tpe *types.Struct, typable ifaces.SwaggerTypable) error {
	sb := schema.NewBuilder(p.ctx, p.decl)
	if err := sb.BuildFromType(tpe, typable); err != nil {
		return err
	}
	p.postDecls = append(p.postDecls, sb.PostDeclarations()...)

	return nil
}

func (p *ParameterBuilder) buildFromFieldMap(ftpe *types.Map, typable ifaces.SwaggerTypable) error {
	sch := new(oaispec.Schema)
	typable.Schema().Typed("object", "").AdditionalProperties = &oaispec.SchemaOrBool{
		Schema: sch,
	}

	sb := schema.NewBuilder(p.ctx, p.decl)
	if err := sb.BuildFromType(ftpe.Elem(), schema.NewTypable(sch, typable.Level()+1, p.ctx.SkipExtensions())); err != nil {
		return err
	}

	return nil
}

func (p *ParameterBuilder) buildFromFieldInterface(tpe *types.Interface, typable ifaces.SwaggerTypable) error {
	sb := schema.NewBuilder(p.ctx, p.decl)
	if err := sb.BuildFromType(tpe, typable); err != nil {
		return err
	}

	p.postDecls = append(p.postDecls, sb.PostDeclarations()...)

	return nil
}

func (p *ParameterBuilder) buildNamedField(ftpe *types.Named, typable ifaces.SwaggerTypable) error {
	o := ftpe.Obj()
	if resolvers.IsAny(o) {
		// e.g. Field interface{} or Field any
		return nil
	}
	if resolvers.IsStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a parameter definition: %w", o.Name(), ErrParameters)
	}
	resolvers.MustNotBeABuiltinType(o)

	decl, found := p.ctx.DeclForType(o.Type())
	if !found {
		return fmt.Errorf("unable to find package and source file for: %s: %w", ftpe.String(), ErrParameters)
	}

	if resolvers.IsStdTime(o) {
		typable.Typed("string", "date-time")
		return nil
	}

	if sfnm, isf := parsers.StrfmtName(decl.Comments); isf {
		typable.Typed("string", sfnm)
		return nil
	}

	sb := schema.NewBuilder(p.ctx, decl)
	sb.InferNames()
	if err := sb.BuildFromType(decl.ObjType(), typable); err != nil {
		return err
	}

	p.postDecls = append(p.postDecls, sb.PostDeclarations()...)

	return nil
}

func (p *ParameterBuilder) buildFieldAlias(tpe *types.Alias, typable ifaces.SwaggerTypable, fld *types.Var, seen map[string]oaispec.Parameter) error {
	o := tpe.Obj()
	if resolvers.IsAny(o) {
		// e.g. Field interface{} or Field any
		_ = typable.Schema()

		return nil // just leave an empty schema
	}
	if resolvers.IsStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a parameter definition: %w", o.Name(), ErrParameters)
	}
	resolvers.MustNotBeABuiltinType(o)
	resolvers.MustHaveRightHandSide(tpe)

	rhs := tpe.Rhs()

	// If transparent aliases are enabled, use the underlying type directly without creating a definition
	if p.ctx.TransparentAliases() {
		sb := schema.NewBuilder(p.ctx, p.decl)
		if err := sb.BuildFromType(rhs, typable); err != nil {
			return err
		}
		p.postDecls = append(p.postDecls, sb.PostDeclarations()...)
		return nil
	}

	decl, ok := p.ctx.FindModel(o.Pkg().Path(), o.Name())
	if !ok {
		return fmt.Errorf("can't find source file for aliased type: %v -> %v: %w", tpe, rhs, ErrParameters)
	}
	p.postDecls = append(p.postDecls, decl) // mark the left-hand side as discovered

	if typable.In() != inBody || !p.ctx.RefAliases() {
		// if ref option is disabled, and always for non-body parameters: just expand the alias
		unaliased := types.Unalias(tpe)
		return p.buildFromField(fld, unaliased, typable, seen)
	}

	// for parameters that are full-fledged schemas, consider expanding or ref'ing
	switch rtpe := rhs.(type) {
	// load declaration for named RHS type (might be an alias itself)
	case *types.Named:
		o := rtpe.Obj()
		if o.Pkg() == nil {
			break // builtin
		}

		decl, found := p.ctx.FindModel(o.Pkg().Path(), o.Name())
		if !found {
			return fmt.Errorf("can't find source file for target type of alias: %v -> %v: %w", tpe, rtpe, ErrParameters)
		}

		return p.makeRef(decl, typable)
	case *types.Alias:
		o := rtpe.Obj()
		if o.Pkg() == nil {
			break // builtin
		}

		decl, found := p.ctx.FindModel(o.Pkg().Path(), o.Name())
		if !found {
			return fmt.Errorf("can't find source file for target type of alias: %v -> %v: %w", tpe, rtpe, ErrParameters)
		}

		return p.makeRef(decl, typable)
	}

	// anonymous type: just expand it
	return p.buildFromField(fld, rhs, typable, seen)
}

func (p *ParameterBuilder) buildFromStruct(decl *scanner.EntityDecl, tpe *types.Struct, op *oaispec.Operation, seen map[string]oaispec.Parameter) error {
	numFields := tpe.NumFields()

	if numFields == 0 {
		return nil
	}

	sequence := make([]string, 0, numFields)
	for fld := range tpe.Fields() {
		if fld.Embedded() {
			if err := p.buildFromType(fld.Type(), op, seen); err != nil {
				return err
			}
			continue
		}

		name, err := p.processParamField(fld, decl, seen)
		if err != nil {
			return err
		}

		if name != "" {
			sequence = append(sequence, name)
		}
	}

	for _, k := range sequence {
		p := seen[k]
		for i, v := range op.Parameters {
			if v.Name == k {
				op.Parameters = append(op.Parameters[:i], op.Parameters[i+1:]...)
				break
			}
		}
		op.Parameters = append(op.Parameters, p)
	}

	return nil
}

// processParamField processes a single non-embedded struct field for parameter building.
// Returns the parameter name if the field was processed, or "" if it was skipped.
func (p *ParameterBuilder) processParamField(fld *types.Var, decl *scanner.EntityDecl, seen map[string]oaispec.Parameter) (string, error) {
	if !fld.Exported() {
		logger.DebugLogf(p.ctx.Debug(), "skipping field %s because it's not exported", fld.Name())
		return "", nil
	}

	afld := resolvers.FindASTField(decl.File, fld.Pos())
	if afld == nil {
		logger.DebugLogf(p.ctx.Debug(), "can't find source associated with %s", fld.String())
		return "", nil
	}

	if parsers.Ignored(afld.Doc) {
		return "", nil
	}

	name, ignore, _, _, err := resolvers.ParseJSONTag(afld)
	if err != nil {
		return "", err
	}
	if ignore {
		return "", nil
	}

	in := "query"
	// scan for param location first, this changes some behavior down the line
	if afld.Doc != nil {
		inOverride, ok := parsers.ParamLocation(afld.Doc)
		if ok {
			in = inOverride
		}
	}

	ps := seen[name]
	ps.In = in
	var pty ifaces.SwaggerTypable = paramTypable{&ps, p.ctx.SkipExtensions()}
	if in == inBody {
		pty = schema.NewTypable(pty.Schema(), 0, p.ctx.SkipExtensions())
	}

	if in == "formData" && afld.Doc != nil && parsers.FileParam(afld.Doc) {
		pty.Typed("file", "")
	} else if err := p.buildFromField(fld, fld.Type(), pty, seen); err != nil {
		return "", err
	}

	if strfmtName, ok := parsers.StrfmtName(afld.Doc); ok {
		ps.Typed("string", strfmtName)
		ps.Ref = oaispec.Ref{}
		ps.Items = nil
	}

	taggers, err := setupParamTaggers(&ps, name, afld, p.ctx.SkipExtensions(), p.ctx.Debug())
	if err != nil {
		return "", err
	}

	sp := parsers.NewSectionedParser(
		parsers.WithSetDescription(func(lines []string) {
			ps.Description = parsers.JoinDropLast(lines)
			enumDesc := parsers.GetEnumDesc(ps.Extensions)
			if enumDesc != "" {
				ps.Description += "\n" + enumDesc
			}
		}),
		parsers.WithTaggers(taggers...),
	)

	if err := sp.Parse(afld.Doc); err != nil {
		return "", err
	}
	if ps.In == "path" {
		ps.Required = true
	}

	if ps.Name == "" {
		ps.Name = name
	}

	if name != fld.Name() {
		resolvers.AddExtension(&ps.VendorExtensible, "x-go-name", fld.Name(), p.ctx.SkipExtensions())
	}

	seen[name] = ps
	return name, nil
}

func (p *ParameterBuilder) makeRef(decl *scanner.EntityDecl, prop ifaces.SwaggerTypable) error {
	nm, _ := decl.Names()
	ref, err := oaispec.NewRef("#/definitions/" + nm)
	if err != nil {
		return err
	}

	prop.SetRef(ref)
	p.postDecls = append(p.postDecls, decl) // mark the $ref target as discovered

	return nil
}

func spExtensionsSetter(ps *oaispec.Parameter, skipExt bool) func(*oaispec.Extensions) {
	return func(exts *oaispec.Extensions) {
		for name, value := range *exts {
			resolvers.AddExtension(&ps.VendorExtensible, name, value, skipExt)
		}
	}
}
