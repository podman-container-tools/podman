// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codescan

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/go-openapi/spec"
)

type paramTypable struct {
	param *spec.Parameter
}

func (pt paramTypable) In() string { return pt.param.In }

func (pt paramTypable) Level() int { return 0 }

func (pt paramTypable) Typed(tpe, format string) {
	pt.param.Typed(tpe, format)
}

func (pt paramTypable) SetRef(ref spec.Ref) {
	pt.param.Ref = ref
}

func (pt paramTypable) Items() swaggerTypable {
	bdt, schema := bodyTypable(pt.param.In, pt.param.Schema)
	if bdt != nil {
		pt.param.Schema = schema
		return bdt
	}

	if pt.param.Items == nil {
		pt.param.Items = new(spec.Items)
	}
	pt.param.Type = "array"
	return itemsTypable{pt.param.Items, 1, pt.param.In}
}

func (pt paramTypable) Schema() *spec.Schema {
	if pt.param.In != "body" {
		return nil
	}
	if pt.param.Schema == nil {
		pt.param.Schema = new(spec.Schema)
	}
	return pt.param.Schema
}

func (pt paramTypable) AddExtension(key string, value any) {
	if pt.param.In == "body" {
		pt.Schema().AddExtension(key, value)
	} else {
		pt.param.AddExtension(key, value)
	}
}

func (pt paramTypable) WithEnum(values ...any) {
	pt.param.WithEnum(values...)
}

func (pt paramTypable) WithEnumDescription(desc string) {
	if desc == "" {
		return
	}
	pt.param.AddExtension(extEnumDesc, desc)
}

type itemsTypable struct {
	items *spec.Items
	level int
	in    string
}

func (pt itemsTypable) In() string { return pt.in } // TODO(fred): inherit from param

func (pt itemsTypable) Level() int { return pt.level }

func (pt itemsTypable) Typed(tpe, format string) {
	pt.items.Typed(tpe, format)
}

func (pt itemsTypable) SetRef(ref spec.Ref) {
	pt.items.Ref = ref
}

func (pt itemsTypable) Schema() *spec.Schema {
	return nil
}

func (pt itemsTypable) Items() swaggerTypable {
	if pt.items.Items == nil {
		pt.items.Items = new(spec.Items)
	}
	pt.items.Type = "array"
	return itemsTypable{pt.items.Items, pt.level + 1, pt.in}
}

func (pt itemsTypable) AddExtension(key string, value any) {
	pt.items.AddExtension(key, value)
}

func (pt itemsTypable) WithEnum(values ...any) {
	pt.items.WithEnum(values...)
}

func (pt itemsTypable) WithEnumDescription(_ string) {
	// no
}

type paramValidations struct {
	current *spec.Parameter
}

func (sv paramValidations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}

func (sv paramValidations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}
func (sv paramValidations) SetMultipleOf(val float64)      { sv.current.MultipleOf = &val }
func (sv paramValidations) SetMinItems(val int64)          { sv.current.MinItems = &val }
func (sv paramValidations) SetMaxItems(val int64)          { sv.current.MaxItems = &val }
func (sv paramValidations) SetMinLength(val int64)         { sv.current.MinLength = &val }
func (sv paramValidations) SetMaxLength(val int64)         { sv.current.MaxLength = &val }
func (sv paramValidations) SetPattern(val string)          { sv.current.Pattern = val }
func (sv paramValidations) SetUnique(val bool)             { sv.current.UniqueItems = val }
func (sv paramValidations) SetCollectionFormat(val string) { sv.current.CollectionFormat = val }
func (sv paramValidations) SetEnum(val string) {
	sv.current.Enum = parseEnum(val, &spec.SimpleSchema{Type: sv.current.Type, Format: sv.current.Format})
}
func (sv paramValidations) SetDefault(val any) { sv.current.Default = val }
func (sv paramValidations) SetExample(val any) { sv.current.Example = val }

type itemsValidations struct {
	current *spec.Items
}

func (sv itemsValidations) SetMaximum(val float64, exclusive bool) {
	sv.current.Maximum = &val
	sv.current.ExclusiveMaximum = exclusive
}

func (sv itemsValidations) SetMinimum(val float64, exclusive bool) {
	sv.current.Minimum = &val
	sv.current.ExclusiveMinimum = exclusive
}
func (sv itemsValidations) SetMultipleOf(val float64)      { sv.current.MultipleOf = &val }
func (sv itemsValidations) SetMinItems(val int64)          { sv.current.MinItems = &val }
func (sv itemsValidations) SetMaxItems(val int64)          { sv.current.MaxItems = &val }
func (sv itemsValidations) SetMinLength(val int64)         { sv.current.MinLength = &val }
func (sv itemsValidations) SetMaxLength(val int64)         { sv.current.MaxLength = &val }
func (sv itemsValidations) SetPattern(val string)          { sv.current.Pattern = val }
func (sv itemsValidations) SetUnique(val bool)             { sv.current.UniqueItems = val }
func (sv itemsValidations) SetCollectionFormat(val string) { sv.current.CollectionFormat = val }
func (sv itemsValidations) SetEnum(val string) {
	sv.current.Enum = parseEnum(val, &spec.SimpleSchema{Type: sv.current.Type, Format: sv.current.Format})
}
func (sv itemsValidations) SetDefault(val any) { sv.current.Default = val }
func (sv itemsValidations) SetExample(val any) { sv.current.Example = val }

type parameterBuilder struct {
	ctx       *scanCtx
	decl      *entityDecl
	postDecls []*entityDecl
}

func (p *parameterBuilder) Build(operations map[string]*spec.Operation) error {
	// check if there is a swagger:parameters tag that is followed by one or more words,
	// these words are the ids of the operations this parameter struct applies to
	// once type name is found convert it to a schema, by looking up the schema in the
	// parameters dictionary that got passed into this parse method
	for _, opid := range p.decl.OperationIDs() {
		operation, ok := operations[opid]
		if !ok {
			operation = new(spec.Operation)
			operations[opid] = operation
			operation.ID = opid
		}
		debugLogf("building parameters for: %s", opid)

		// analyze struct body for fields etc
		// each exported struct field:
		// * gets a type mapped to a go primitive
		// * perhaps gets a format
		// * has to document the validations that apply for the type and the field
		// * when the struct field points to a model it becomes a ref: #/definitions/ModelName
		// * comments that aren't tags is used as the description
		if err := p.buildFromType(p.decl.ObjType(), operation, make(map[string]spec.Parameter)); err != nil {
			return err
		}
	}

	return nil
}

func (p *parameterBuilder) buildFromType(otpe types.Type, op *spec.Operation, seen map[string]spec.Parameter) error {
	switch tpe := otpe.(type) {
	case *types.Pointer:
		return p.buildFromType(tpe.Elem(), op, seen)
	case *types.Named:
		return p.buildNamedType(tpe, op, seen)
	case *types.Alias:
		debugLogf("alias(parameters.buildFromType): got alias %v to %v", tpe, tpe.Rhs())
		return p.buildAlias(tpe, op, seen)
	default:
		return fmt.Errorf("unhandled type (%T): %s", otpe, tpe.String())
	}
}

func (p *parameterBuilder) buildNamedType(tpe *types.Named, op *spec.Operation, seen map[string]spec.Parameter) error {
	o := tpe.Obj()
	if isAny(o) || isStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a parameters section definition", o.Name())
	}
	mustNotBeABuiltinType(o)

	switch stpe := o.Type().Underlying().(type) {
	case *types.Struct:
		debugLogf("build from named type %s: %T", o.Name(), tpe)
		if decl, found := p.ctx.DeclForType(o.Type()); found {
			return p.buildFromStruct(decl, stpe, op, seen)
		}

		return p.buildFromStruct(p.decl, stpe, op, seen)
	default:
		return fmt.Errorf("unhandled type (%T): %s", stpe, o.Type().Underlying().String())
	}
}

func (p *parameterBuilder) buildAlias(tpe *types.Alias, op *spec.Operation, seen map[string]spec.Parameter) error {
	o := tpe.Obj()
	if isAny(o) || isStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a parameters section definition", o.Name())
	}
	mustNotBeABuiltinType(o)
	mustHaveRightHandSide(tpe)

	rhs := tpe.Rhs()

	// If transparent aliases are enabled, use the underlying type directly without creating a definition
	if p.ctx.app.transparentAliases {
		return p.buildFromType(rhs, op, seen)
	}

	decl, ok := p.ctx.FindModel(o.Pkg().Path(), o.Name())
	if !ok {
		return fmt.Errorf("can't find source file for aliased type: %v -> %v", tpe, rhs)
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
			return fmt.Errorf("can't find source file for target type of alias: %v -> %v", tpe, rtpe)
		}
		p.postDecls = append(p.postDecls, decl)
	case *types.Alias:
		o := rtpe.Obj()
		if o.Pkg() == nil {
			break // builtin
		}
		decl, found := p.ctx.FindModel(o.Pkg().Path(), o.Name())
		if !found {
			return fmt.Errorf("can't find source file for target type of alias: %v -> %v", tpe, rtpe)
		}
		p.postDecls = append(p.postDecls, decl)
	}

	return p.buildFromType(rhs, op, seen)
}

func (p *parameterBuilder) buildFromField(fld *types.Var, tpe types.Type, typable swaggerTypable, seen map[string]spec.Parameter) error {
	debugLogf("build from field %s: %T", fld.Name(), tpe)

	switch ftpe := tpe.(type) {
	case *types.Basic:
		return swaggerSchemaForType(ftpe.Name(), typable)
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
		debugLogf("alias(parameters.buildFromField): got alias %v to %v", ftpe, ftpe.Rhs()) // TODO
		return p.buildFieldAlias(ftpe, typable, fld, seen)
	default:
		return fmt.Errorf("unknown type for %s: %T", fld.String(), fld.Type())
	}
}

func (p *parameterBuilder) buildFromFieldStruct(tpe *types.Struct, typable swaggerTypable) error {
	sb := schemaBuilder{
		decl: p.decl,
		ctx:  p.ctx,
	}

	if err := sb.buildFromType(tpe, typable); err != nil {
		return err
	}
	p.postDecls = append(p.postDecls, sb.postDecls...)

	return nil
}

func (p *parameterBuilder) buildFromFieldMap(ftpe *types.Map, typable swaggerTypable) error {
	schema := new(spec.Schema)
	typable.Schema().Typed("object", "").AdditionalProperties = &spec.SchemaOrBool{
		Schema: schema,
	}

	sb := schemaBuilder{
		decl: p.decl,
		ctx:  p.ctx,
	}

	if err := sb.buildFromType(ftpe.Elem(), schemaTypable{schema, typable.Level() + 1}); err != nil {
		return err
	}

	return nil
}

func (p *parameterBuilder) buildFromFieldInterface(tpe *types.Interface, typable swaggerTypable) error {
	sb := schemaBuilder{
		decl: p.decl,
		ctx:  p.ctx,
	}

	if err := sb.buildFromType(tpe, typable); err != nil {
		return err
	}

	p.postDecls = append(p.postDecls, sb.postDecls...)

	return nil
}

func (p *parameterBuilder) buildNamedField(ftpe *types.Named, typable swaggerTypable) error {
	o := ftpe.Obj()
	if isAny(o) {
		// e.g. Field interface{} or Field any
		return nil
	}
	if isStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a parameter definition", o.Name())
	}
	mustNotBeABuiltinType(o)

	decl, found := p.ctx.DeclForType(o.Type())
	if !found {
		return fmt.Errorf("unable to find package and source file for: %s", ftpe.String())
	}

	if isStdTime(o) {
		typable.Typed("string", "date-time")
		return nil
	}

	if sfnm, isf := strfmtName(decl.Comments); isf {
		typable.Typed("string", sfnm)
		return nil
	}

	sb := &schemaBuilder{ctx: p.ctx, decl: decl}
	sb.inferNames()
	if err := sb.buildFromType(decl.ObjType(), typable); err != nil {
		return err
	}

	p.postDecls = append(p.postDecls, sb.postDecls...)

	return nil
}

func (p *parameterBuilder) buildFieldAlias(tpe *types.Alias, typable swaggerTypable, fld *types.Var, seen map[string]spec.Parameter) error {
	o := tpe.Obj()
	if isAny(o) {
		// e.g. Field interface{} or Field any
		_ = typable.Schema()

		return nil // just leave an empty schema
	}
	if isStdError(o) {
		return fmt.Errorf("%s type not supported in the context of a parameter definition", o.Name())
	}
	mustNotBeABuiltinType(o)
	mustHaveRightHandSide(tpe)

	rhs := tpe.Rhs()

	// If transparent aliases are enabled, use the underlying type directly without creating a definition
	if p.ctx.app.transparentAliases {
		sb := schemaBuilder{
			decl: p.decl,
			ctx:  p.ctx,
		}
		if err := sb.buildFromType(rhs, typable); err != nil {
			return err
		}
		p.postDecls = append(p.postDecls, sb.postDecls...)
		return nil
	}

	decl, ok := p.ctx.FindModel(o.Pkg().Path(), o.Name())
	if !ok {
		return fmt.Errorf("can't find source file for aliased type: %v -> %v", tpe, rhs)
	}
	p.postDecls = append(p.postDecls, decl) // mark the left-hand side as discovered

	if typable.In() != "body" || !p.ctx.app.refAliases {
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
			return fmt.Errorf("can't find source file for target type of alias: %v -> %v", tpe, rtpe)
		}

		return p.makeRef(decl, typable)
	case *types.Alias:
		o := rtpe.Obj()
		if o.Pkg() == nil {
			break // builtin
		}

		decl, found := p.ctx.FindModel(o.Pkg().Path(), o.Name())
		if !found {
			return fmt.Errorf("can't find source file for target type of alias: %v -> %v", tpe, rtpe)
		}

		return p.makeRef(decl, typable)
	}

	// anonymous type: just expand it
	return p.buildFromField(fld, rhs, typable, seen)
}

func spExtensionsSetter(ps *spec.Parameter) func(*spec.Extensions) {
	return func(exts *spec.Extensions) {
		for name, value := range *exts {
			addExtension(&ps.VendorExtensible, name, value)
		}
	}
}

func (p *parameterBuilder) buildFromStruct(decl *entityDecl, tpe *types.Struct, op *spec.Operation, seen map[string]spec.Parameter) error {
	numFields := tpe.NumFields()

	if numFields == 0 {
		return nil
	}

	sequence := make([]string, 0, numFields)
	for i := range numFields {
		fld := tpe.Field(i)

		if fld.Embedded() {
			if err := p.buildFromType(fld.Type(), op, seen); err != nil {
				return err
			}
			continue
		}

		if !fld.Exported() {
			debugLogf("skipping field %s because it's not exported", fld.Name())
			continue
		}

		tg := tpe.Tag(i)

		var afld *ast.Field
		ans, _ := astutil.PathEnclosingInterval(decl.File, fld.Pos(), fld.Pos())
		for _, an := range ans {
			at, valid := an.(*ast.Field)
			if !valid {
				continue
			}

			debugLogf("field %s: %s(%T) [%q] ==> %s", fld.Name(), fld.Type().String(), fld.Type(), tg, at.Doc.Text())
			afld = at
			break
		}

		if afld == nil {
			debugLogf("can't find source associated with %s for %s", fld.String(), tpe.String())
			continue
		}

		// if the field is annotated with swagger:ignore, ignore it
		if ignored(afld.Doc) {
			continue
		}

		name, ignore, _, _, err := parseJSONTag(afld)
		if err != nil {
			return err
		}
		if ignore {
			continue
		}

		in := "query"
		// scan for param location first, this changes some behavior down the line
		if afld.Doc != nil {
			for _, cmt := range afld.Doc.List {
				for line := range strings.SplitSeq(cmt.Text, "\n") {
					matches := rxIn.FindStringSubmatch(line)
					if len(matches) > 0 && len(strings.TrimSpace(matches[1])) > 0 {
						in = strings.TrimSpace(matches[1])
					}
				}
			}
		}

		ps := seen[name]
		ps.In = in
		var pty swaggerTypable = paramTypable{&ps}
		if in == "body" {
			pty = schemaTypable{pty.Schema(), 0}
		}
		if in == "formData" && afld.Doc != nil && fileParam(afld.Doc) {
			pty.Typed("file", "")
		} else if err := p.buildFromField(fld, fld.Type(), pty, seen); err != nil {
			return err
		}

		if strfmtName, ok := strfmtName(afld.Doc); ok {
			ps.Typed("string", strfmtName)
			ps.Ref = spec.Ref{}
			ps.Items = nil
		}

		sp := new(sectionedParser)
		sp.setDescription = func(lines []string) {
			ps.Description = joinDropLast(lines)
			enumDesc := getEnumDesc(ps.Extensions)
			if enumDesc != "" {
				ps.Description += "\n" + enumDesc
			}
		}
		if ps.Ref.String() == "" {
			sp.taggers = []tagParser{
				newSingleLineTagParser("in", &matchOnlyParam{&ps, rxIn}),
				newSingleLineTagParser("maximum", &setMaximum{paramValidations{&ps}, rxf(rxMaximumFmt, "")}),
				newSingleLineTagParser("minimum", &setMinimum{paramValidations{&ps}, rxf(rxMinimumFmt, "")}),
				newSingleLineTagParser("multipleOf", &setMultipleOf{paramValidations{&ps}, rxf(rxMultipleOfFmt, "")}),
				newSingleLineTagParser("minLength", &setMinLength{paramValidations{&ps}, rxf(rxMinLengthFmt, "")}),
				newSingleLineTagParser("maxLength", &setMaxLength{paramValidations{&ps}, rxf(rxMaxLengthFmt, "")}),
				newSingleLineTagParser("pattern", &setPattern{paramValidations{&ps}, rxf(rxPatternFmt, "")}),
				newSingleLineTagParser("collectionFormat", &setCollectionFormat{paramValidations{&ps}, rxf(rxCollectionFormatFmt, "")}),
				newSingleLineTagParser("minItems", &setMinItems{paramValidations{&ps}, rxf(rxMinItemsFmt, "")}),
				newSingleLineTagParser("maxItems", &setMaxItems{paramValidations{&ps}, rxf(rxMaxItemsFmt, "")}),
				newSingleLineTagParser("unique", &setUnique{paramValidations{&ps}, rxf(rxUniqueFmt, "")}),
				newSingleLineTagParser("enum", &setEnum{paramValidations{&ps}, rxf(rxEnumFmt, "")}),
				newSingleLineTagParser("default", &setDefault{&ps.SimpleSchema, paramValidations{&ps}, rxf(rxDefaultFmt, "")}),
				newSingleLineTagParser("example", &setExample{&ps.SimpleSchema, paramValidations{&ps}, rxf(rxExampleFmt, "")}),
				newSingleLineTagParser("required", &setRequiredParam{&ps}),
				newMultiLineTagParser("Extensions", newSetExtensions(spExtensionsSetter(&ps)), true),
			}

			itemsTaggers := func(items *spec.Items, level int) []tagParser {
				// the expression is 1-index based not 0-index
				itemsPrefix := fmt.Sprintf(rxItemsPrefixFmt, level+1)

				return []tagParser{
					newSingleLineTagParser(fmt.Sprintf("items%dMaximum", level), &setMaximum{itemsValidations{items}, rxf(rxMaximumFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dMinimum", level), &setMinimum{itemsValidations{items}, rxf(rxMinimumFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dMultipleOf", level), &setMultipleOf{itemsValidations{items}, rxf(rxMultipleOfFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dMinLength", level), &setMinLength{itemsValidations{items}, rxf(rxMinLengthFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dMaxLength", level), &setMaxLength{itemsValidations{items}, rxf(rxMaxLengthFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dPattern", level), &setPattern{itemsValidations{items}, rxf(rxPatternFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dCollectionFormat", level), &setCollectionFormat{itemsValidations{items}, rxf(rxCollectionFormatFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dMinItems", level), &setMinItems{itemsValidations{items}, rxf(rxMinItemsFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dMaxItems", level), &setMaxItems{itemsValidations{items}, rxf(rxMaxItemsFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dUnique", level), &setUnique{itemsValidations{items}, rxf(rxUniqueFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dEnum", level), &setEnum{itemsValidations{items}, rxf(rxEnumFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dDefault", level), &setDefault{&items.SimpleSchema, itemsValidations{items}, rxf(rxDefaultFmt, itemsPrefix)}),
					newSingleLineTagParser(fmt.Sprintf("items%dExample", level), &setExample{&items.SimpleSchema, itemsValidations{items}, rxf(rxExampleFmt, itemsPrefix)}),
				}
			}

			var parseArrayTypes func(expr ast.Expr, items *spec.Items, level int) ([]tagParser, error)
			parseArrayTypes = func(expr ast.Expr, items *spec.Items, level int) ([]tagParser, error) {
				if items == nil {
					return []tagParser{}, nil
				}
				switch iftpe := expr.(type) {
				case *ast.ArrayType:
					eleTaggers := itemsTaggers(items, level)
					sp.taggers = append(eleTaggers, sp.taggers...)
					otherTaggers, err := parseArrayTypes(iftpe.Elt, items.Items, level+1)
					if err != nil {
						return nil, err
					}
					return otherTaggers, nil
				case *ast.SelectorExpr:
					otherTaggers, err := parseArrayTypes(iftpe.Sel, items.Items, level+1)
					if err != nil {
						return nil, err
					}
					return otherTaggers, nil
				case *ast.Ident:
					taggers := []tagParser{}
					if iftpe.Obj == nil {
						taggers = itemsTaggers(items, level)
					}
					otherTaggers, err := parseArrayTypes(expr, items.Items, level+1)
					if err != nil {
						return nil, err
					}
					return append(taggers, otherTaggers...), nil
				case *ast.StarExpr:
					otherTaggers, err := parseArrayTypes(iftpe.X, items, level)
					if err != nil {
						return nil, err
					}
					return otherTaggers, nil
				default:
					return nil, fmt.Errorf("unknown field type ele for %q", name)
				}
			}

			// check if this is a primitive, if so parse the validations from the
			// doc comments of the slice declaration.
			if ftped, ok := afld.Type.(*ast.ArrayType); ok {
				taggers, err := parseArrayTypes(ftped.Elt, ps.Items, 0)
				if err != nil {
					return err
				}
				sp.taggers = append(taggers, sp.taggers...)
			}
		} else {
			sp.taggers = []tagParser{
				newSingleLineTagParser("in", &matchOnlyParam{&ps, rxIn}),
				newSingleLineTagParser("required", &matchOnlyParam{&ps, rxRequired}),
				newMultiLineTagParser("Extensions", newSetExtensions(spExtensionsSetter(&ps)), true),
			}
		}
		if err := sp.Parse(afld.Doc); err != nil {
			return err
		}
		if ps.In == "path" {
			ps.Required = true
		}

		if ps.Name == "" {
			ps.Name = name
		}

		if name != fld.Name() {
			addExtension(&ps.VendorExtensible, "x-go-name", fld.Name())
		}
		seen[name] = ps
		sequence = append(sequence, name)
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

func (p *parameterBuilder) makeRef(decl *entityDecl, prop swaggerTypable) error {
	nm, _ := decl.Names()
	ref, err := spec.NewRef("#/definitions/" + nm)
	if err != nil {
		return err
	}

	prop.SetRef(ref)
	p.postDecls = append(p.postDecls, decl) // mark the $ref target as discovered

	return nil
}
