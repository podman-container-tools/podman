// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers"
	"golang.org/x/tools/go/packages"
)

type EntityDecl struct {
	Comments               *ast.CommentGroup
	Type                   *types.Named
	Alias                  *types.Alias // added to supplement Named, after go1.22
	Ident                  *ast.Ident
	Spec                   *ast.TypeSpec
	File                   *ast.File
	Pkg                    *packages.Package
	hasModelAnnotation     bool
	hasResponseAnnotation  bool
	hasParameterAnnotation bool
}

// Obj returns the type name for the declaration defining the named type or alias t.
func (d *EntityDecl) Obj() *types.TypeName {
	if d.Type != nil {
		return d.Type.Obj()
	}
	if d.Alias != nil {
		return d.Alias.Obj()
	}

	panic("invalid EntityDecl: Type and Alias are both nil")
}

func (d *EntityDecl) ObjType() types.Type {
	if d.Type != nil {
		return d.Type
	}
	if d.Alias != nil {
		return d.Alias
	}

	panic("invalid EntityDecl: Type and Alias are both nil")
}

func (d *EntityDecl) Names() (name, goName string) {
	goName = d.Ident.Name
	model, ok := parsers.ModelOverride(d.Comments)
	if !ok {
		return goName, goName
	}

	d.hasModelAnnotation = true
	if model == "" {
		return goName, goName
	}

	return model, goName
}

func (d *EntityDecl) ResponseNames() (name, goName string) {
	goName = d.Ident.Name
	response, ok := parsers.ResponseOverride(d.Comments)
	if !ok {
		return name, goName
	}

	d.hasResponseAnnotation = true
	if response == "" {
		return goName, goName
	}

	return response, goName
}

func (d *EntityDecl) OperationIDs() (result []string) {
	if d == nil {
		return nil
	}

	parameters, ok := parsers.ParametersOverride(d.Comments)
	if !ok {
		return nil
	}

	d.hasParameterAnnotation = true

	for _, parameter := range parameters {
		for param := range strings.SplitSeq(parameter, " ") {
			name := strings.TrimSpace(param)
			if len(name) > 0 {
				result = append(result, name)
			}
		}
	}

	return result
}

func (d *EntityDecl) HasModelAnnotation() bool {
	if d.hasModelAnnotation {
		return true
	}

	_, ok := parsers.ModelOverride(d.Comments)
	if !ok {
		return false
	}

	d.hasModelAnnotation = true

	return true
}

func (d *EntityDecl) HasResponseAnnotation() bool {
	if d.hasResponseAnnotation {
		return true
	}

	_, ok := parsers.ResponseOverride(d.Comments)
	if !ok {
		return false
	}

	d.hasResponseAnnotation = true

	return true
}

func (d *EntityDecl) HasParameterAnnotation() bool {
	if d.hasParameterAnnotation {
		return true
	}

	_, ok := parsers.ParametersOverride(d.Comments)
	if !ok {
		return false
	}

	d.hasParameterAnnotation = true

	return true
}
