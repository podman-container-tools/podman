// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/ast"
	"go/types"

	"github.com/go-openapi/codescan/internal/ifaces"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

func (s *Builder) buildFromInterface(decl *scanner.EntityDecl, it *types.Interface, schema *oaispec.Schema, nameByJSON map[string]propOwner) error {
	if it.Empty() {
		// return an empty schema for empty interfaces
		return nil
	}

	var (
		target   *oaispec.Schema
		hasAllOf bool
	)

	var flist []*ast.Field
	if specType, ok := decl.Spec.Type.(*ast.InterfaceType); ok {
		flist = make([]*ast.Field, it.NumEmbeddeds()+it.NumExplicitMethods())
		copy(flist, specType.Methods.List)
	}

	// First collect the embedded interfaces create refs when:
	//
	//   1. the embedded interface is decorated with an allOf annotation
	//   2. the embedded interface is an alias
	for fld := range it.EmbeddedTypes() {
		if target == nil {
			target = &oaispec.Schema{}
		}

		fieldHasAllOf, err := s.processEmbeddedType(fld, flist, decl, schema, nameByJSON)
		if err != nil {
			return err
		}
		hasAllOf = hasAllOf || fieldHasAllOf
	}

	if target == nil {
		target = schema
	}

	// We can finally build the actual schema for the struct
	if target.Properties == nil {
		target.Properties = make(map[string]oaispec.Schema)
	}
	target.Typed("object", "")

	// Cross-ref linkage: same divergence guard as buildFromStruct — methods landing in a fresh allOf
	// member resolve to schema's anchor.
	if target != schema {
		defer s.repath("")()
	}

	for fld := range it.ExplicitMethods() {
		if err := s.processInterfaceMethod(fld, decl, target, nameByJSON); err != nil {
			return err
		}
	}

	if target == nil {
		return nil
	}
	if hasAllOf && len(target.Properties) > 0 {
		schema.AllOf = append(schema.AllOf, *target)
	}

	return nil
}

func (s *Builder) processInterfaceMethod(fld *types.Func, decl *scanner.EntityDecl, target *oaispec.Schema, nameByJSON map[string]propOwner) error {
	c, ok := s.methodCarrier(fld, decl)
	if !ok {
		return nil
	}
	return s.applyFieldCarrier(c, target, nameByJSON)
}

func (s *Builder) buildNamedInterface(
	ftpe *types.Named, flist []*ast.Field, decl *scanner.EntityDecl, schema *oaispec.Schema, nameByJSON map[string]propOwner,
) (hasAllOf bool, err error) {
	o := ftpe.Obj()
	var afld *ast.Field

	for _, an := range flist {
		if len(an.Names) != 0 {
			continue
		}

		tpp := decl.Pkg.TypesInfo.Types[an.Type]
		if tpp.Type.String() != o.Type().String() {
			continue
		}

		// decl.
		afld = an
		break
	}

	if afld == nil {
		return hasAllOf, nil
	}

	fd := s.scanFieldDoc(afld)
	if fd.Ignored {
		return hasAllOf, nil
	}

	if !fd.IsAllOfMember {
		var newSch oaispec.Schema
		if err = s.buildEmbedded(o.Type(), &newSch, nameByJSON); err != nil {
			return hasAllOf, err
		}
		schema.AllOf = append(schema.AllOf, newSch)
		hasAllOf = true

		return hasAllOf, nil
	}

	hasAllOf = true

	var newSch oaispec.Schema
	// when the embedded struct is annotated with swagger:allOf it will be used as allOf property
	// otherwise the fields will just be included as normal properties.
	if err = s.buildAllOf(o.Type(), &newSch); err != nil {
		return hasAllOf, err
	}

	if fd.AllOfClass != "" {
		schema.AddExtension("x-class", fd.AllOfClass)
	}

	schema.AllOf = append(schema.AllOf, newSch)

	return hasAllOf, nil
}

func (s *Builder) buildAnonymousInterface(it *types.Interface, target ifaces.SwaggerTypable, decl *scanner.EntityDecl) error {
	target.Typed("object", "")

	for fld := range it.ExplicitMethods() {
		if err := s.processAnonInterfaceMethod(fld, decl, target.Schema()); err != nil {
			return err
		}
	}

	return nil
}

func (s *Builder) processAnonInterfaceMethod(fld *types.Func, decl *scanner.EntityDecl, schema *oaispec.Schema) error {
	c, ok := s.methodCarrier(fld, decl)
	if !ok {
		return nil
	}
	return s.applyFieldCarrier(c, schema, nil)
}
