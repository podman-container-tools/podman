// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/imports"
)

func formatGo(filename string, content []byte, opts ...FormatOption) ([]byte, error) {
	fset, file, clean, err := parseGoOrFragment(filename, content)
	if err != nil {
		return nil, err
	}

	removeBlankLines(fset, file) // so that goimports sorts all imports together
	fixImports(fset, file)
	removeUnecessaryImportParens(file)

	printConfig := &printer.Config{
		Mode:     printer.UseSpaces | printer.TabIndent,
		Tabwidth: defaultIndent,
	}
	var buf bytes.Buffer
	err = printConfig.Fprint(&buf, fset, file)
	if err != nil {
		return nil, err
	}

	tmp := buf.Bytes()
	if clean != nil {
		tmp = clean(tmp)
	}

	out, err := formatByImports(filename, tmp, formatOptionsWithDefault(opts))
	if err != nil {
		return nil, err
	}

	return out, nil
}

func parseGoOrFragment(filename string, content []byte) (*token.FileSet, *ast.File, func([]byte) []byte, error) {
	fset, file, err := parseGo(filename, content)
	if err == nil {
		return fset, file, nil, nil
	}

	// In case content doesn't have a package statement, we consider it may be a fragment and try to parse with package statement.
	// For other cases, we give up and return the error.
	if !strings.Contains(err.Error(), "expected 'package'") {
		return nil, nil, nil, err
	}

	content = append([]byte("package main;\n"), content...)
	fset, file, err = parseGo(filename, content)
	if err != nil {
		return nil, nil, nil, err
	}

	cleanup := func(out []byte) []byte {
		out = bytes.TrimPrefix(out, []byte("package main;\n"))
		return out
	}
	return fset, file, cleanup, nil
}

func parseGo(ffn string, content []byte) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	mode := parser.ParseComments | parser.AllErrors
	file, err := parser.ParseFile(fset, ffn, content, mode)
	if err != nil {
		return nil, nil, err
	}
	return fset, file, nil
}

// fixImports
// - removes unused imports
// - adds missing imports for top-level names.
func fixImports(fset *token.FileSet, file *ast.File) {
	seen := make(map[string]*ast.ImportSpec)
	shouldRemove := []*ast.ImportSpec{}
	usedNames := collectTopNames(file)
	for _, impt := range file.Imports {
		name := importPathToAssumedName(importPath(impt))
		if impt.Name != nil {
			name = impt.Name.String()
		}
		if name == "_" || name == "." {
			continue
		}

		// astutil.UsesImport is not precise enough for our needs: https://github.com/golang/go/issues/30331#issuecomment-466174437
		if !usedNames[name] {
			shouldRemove = append(shouldRemove, impt)
			continue
		}

		// latter import wins for same name. this is heuristic and might be incorrect for some cases.
		if prev := seen[name]; prev != nil {
			shouldRemove = append(shouldRemove, prev)
		}
		seen[name] = impt
	}

	for name := range usedNames {
		if name == "_" || name == "." {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		if pkg, ok := autoImports[name]; ok {
			if !astutil.AddImport(fset, file, pkg) {
				panic("failed to add import " + pkg + " for " + name)
			}
		}
	}

	for _, impt := range shouldRemove {
		deleteImportSpec(fset, file, impt)
	}
}

func deleteImportSpec(fset *token.FileSet, file *ast.File, spec *ast.ImportSpec) {
	// remove from file.Imports
	i := slices.IndexFunc(file.Imports, func(i *ast.ImportSpec) bool {
		return i == spec
	})
	if i >= 0 {
		file.Imports = slices.Delete(file.Imports, i, i+1)
	}

	// remove from file.Decls
	gen := importDecl(file)
	if gen == nil {
		return
	}
	i = slices.IndexFunc(gen.Specs, func(i ast.Spec) bool {
		return i == spec
	})
	if i < 0 {
		return
	}
	if i > 0 && gen.Rparen.IsValid() {
		impspec, ok := gen.Specs[i].(*ast.ImportSpec)
		if !ok {
			panic(fmt.Errorf("expected specs to be *ast.ImportSpec, but got %T instead", gen.Specs[i]))
		}
		line := fset.PositionFor(impspec.Path.ValuePos, false).Line
		fset.File(gen.Rparen).MergeLine(line)
	}
	gen.Specs = slices.Delete(gen.Specs, i, i+1)
}

func removeBlankLines(fset *token.FileSet, file *ast.File) {
	gen := importDecl(file)
	if gen == nil {
		return
	}
	specs := gen.Specs
	for i := 0; i+1 < len(specs); i++ {
		spec, ok1 := specs[i].(*ast.ImportSpec)
		nextSpec, ok2 := specs[i+1].(*ast.ImportSpec)
		if !ok1 || !ok2 {
			continue
		}
		line := fset.PositionFor(spec.Path.ValuePos, false).Line
		nextLine := fset.PositionFor(nextSpec.Path.ValuePos, false).Line
		if nextLine-line > 1 {
			fset.File(gen.Rparen).MergeLine(line)
		}
	}
}

func importDecl(file *ast.File) *ast.GenDecl {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		return gen
	}
	return nil
}

func removeUnecessaryImportParens(file *ast.File) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			break
		}
		if gen.Tok != token.IMPORT {
			break
		}
		if len(gen.Specs) != 1 {
			continue
		}
		gen.Lparen = token.NoPos
		gen.Rparen = token.NoPos
	}
}

// importPath returns the unquoted import path of s,
// or "" if the path is not properly quoted.
// Taken from [golang.org/x/tools/ast/astutil](https://cs.opensource.google/go/x/tools/+/refs/tags/v0.32.0:go/ast/astutil/imports.go;l=424).
func importPath(s *ast.ImportSpec) string {
	t, err := strconv.Unquote(s.Path.Value)
	if err != nil {
		return ""
	}
	return t
}

func collectTopNames(n ast.Node) map[string]bool {
	names := make(map[string]bool)
	ast.Walk(visitFn(func(n ast.Node) {
		s, ok := n.(*ast.SelectorExpr)
		if !ok {
			return
		}
		id, ok := s.X.(*ast.Ident)
		if !ok {
			return
		}
		if id.Obj != nil {
			return
		}
		names[id.Name] = true
	}), n)
	return names
}

type visitFn func(node ast.Node)

func (fn visitFn) Visit(node ast.Node) ast.Visitor {
	fn(node)
	return fn
}

// importPathToAssumedName returns the assumed package name of an import path.
// it is taken from [tools/internal/imports/fix.go](https://github.com/golang/tools/blob/v0.33.0/internal/imports/fix.go#L1233)
func importPathToAssumedName(importPath string) string {
	base := path.Base(importPath)
	if strings.HasPrefix(base, "v") {
		if _, err := strconv.Atoi(base[1:]); err == nil {
			dir := path.Dir(importPath)
			if dir != "." {
				base = path.Base(dir)
			}
		}
	}
	base = strings.TrimPrefix(base, "go-")
	if i := strings.IndexFunc(base, notIdentifier); i >= 0 {
		base = base[:i]
	}
	return base
}

// notIdentifier reports whether ch is an invalid identifier character.
// it is taken from [tools/internal/imports/fix.go](https://github.com/golang/tools/blob/v0.33.0/internal/imports/fix.go#L1233)
func notIdentifier(ch rune) bool {
	if 'a' <= ch && ch <= 'z' {
		return false
	}
	if 'A' <= ch && ch <= 'Z' {
		return false
	}
	if '0' <= ch && ch <= '9' {
		return false
	}
	if ch == '_' {
		return false
	}
	if ch < utf8.RuneSelf {
		return true
	}
	return !unicode.IsLetter(ch) && !unicode.IsDigit(ch)
}

var autoImports map[string]string

func init() {
	autoImports = make(map[string]string)

	stdlibs := []string{
		"bytes",
		"context",
		"encoding/json",
		"fmt",
		"io",
		"mime/multipart",
		"os",
		"strconv",
	}

	for _, pkg := range stdlibs {
		autoImports[importPathToAssumedName((pkg))] = pkg
	}

	goOpenAPIs := []string{
		"github.com/go-openapi/loads/fmts",
		"github.com/go-openapi/runtime",
		"github.com/go-openapi/runtime/client",
		"github.com/go-openapi/runtime/yamlpc",
		"github.com/go-openapi/strfmt",
	}
	for _, pkg := range goOpenAPIs {
		autoImports[importPathToAssumedName((pkg))] = pkg
	}
}

// mutex for imports.LocalPrfix global variable.
var localPrefixMutex sync.RWMutex

// run import.Process to sort imports.
func formatByImports(filename string, content []byte, opts formatOptions) ([]byte, error) {
	lp := strings.Join(opts.localPrefixes, ",")
	localPrefixMutex.RLock()
	if lp == imports.LocalPrefix {
		defer localPrefixMutex.RUnlock()
		return imports.Process(filename, content, &opts.Options)
	}
	localPrefixMutex.RUnlock()

	localPrefixMutex.Lock()
	defer localPrefixMutex.Unlock()
	imports.LocalPrefix = lp
	return imports.Process(filename, content, &opts.Options)
}
