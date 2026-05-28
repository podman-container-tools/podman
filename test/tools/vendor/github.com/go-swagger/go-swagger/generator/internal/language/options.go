// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

// Package language provides the language-specific options used by the
// go-swagger code generator. The primary type is [Options], which describes
// formatting, naming, and import resolution rules for a target language.
package language

import (
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/tools/imports"

	"github.com/go-openapi/swag/mangling"

	golangfuncs "github.com/go-swagger/go-swagger/generator/internal/funcmaps/golang"
)

// DefaultIndent is the default tab width used for Go source formatting.
const DefaultIndent = 2

// FormatterFunc is a function that processes go code to reformat it, e.g. [golang.org/x/tools/imports.Process]).
//
// Formatting options allow for injecting a custom formatter for the generated code. See [WithCustomFormatter].
type FormatterFunc func(filename string, src []byte, opts ...FormatOption) ([]byte, error)

// MangleFunc is a function that transforms a name string.
type MangleFunc func(string) string

// FormatOption allows for more flexible code formatting settings.
type FormatOption func(*FormatOpts)

// FormatOpts holds options for code formatting.
type FormatOpts struct {
	imports.Options

	LocalPrefixes []string
}

// WithFormatLocalPrefixes adds local prefixes to group imports.
func WithFormatLocalPrefixes(prefixes ...string) FormatOption {
	return func(o *FormatOpts) {
		o.LocalPrefixes = append(o.LocalPrefixes, prefixes...)
	}
}

// WithFormatOnly tells the formatter to skip imports processing.
func WithFormatOnly(enabled bool) FormatOption {
	return func(o *FormatOpts) {
		o.FormatOnly = enabled
	}
}

// DefaultFormatOpts is the default set of formatting options.
var DefaultFormatOpts = FormatOpts{
	Options: imports.Options{
		TabIndent: true,
		TabWidth:  DefaultIndent,
		Fragment:  true,
		Comments:  true,
	},
	LocalPrefixes: []string{"github.com/go-openapi"},
}

// FormatOptsWithDefault applies the given options on top of [DefaultFormatOpts].
func FormatOptsWithDefault(opts []FormatOption) FormatOpts {
	o := DefaultFormatOpts

	for _, apply := range opts {
		apply(&o)
	}

	return o
}

// Options describes a target language to the code generator.
type Options struct {
	ReservedWords        []string
	BaseImportFunc       MangleFunc                     `json:"-"`
	ImportsFunc          func(map[string]string) string `json:"-"`
	ArrayInitializerFunc func(any) (string, error)      `json:"-"`
	FormatOnly           bool
	ExtraInitialisms     []string
	Mangler              mangling.NameMangler

	reservedWordsSet map[string]struct{}
	initialized      bool
	formatFunc       FormatterFunc
	fileNameFunc     MangleFunc // language specific source file naming rules
	dirNameFunc      MangleFunc // language specific directory naming rules
}

// SetFormatFunc sets the formatting function for this language.
func (l *Options) SetFormatFunc(fn FormatterFunc) {
	l.formatFunc = fn
}

// Init the language option.
func (l *Options) Init() {
	if l.initialized {
		return
	}

	l.Mangler = mangling.NewNameMangler(
		mangling.WithGoNamePrefixFunc(golangfuncs.PrefixForName),
		mangling.WithAdditionalInitialisms(l.ExtraInitialisms...),
	)

	l.initialized = true
	l.reservedWordsSet = make(map[string]struct{})

	for _, rw := range l.ReservedWords {
		l.reservedWordsSet[rw] = struct{}{}
	}
}

// MangleName makes sure a reserved word gets a safe name.
func (l *Options) MangleName(name, suffix string) string {
	if _, ok := l.reservedWordsSet[l.Mangler.ToFileName(name)]; !ok {
		return name
	}

	return strings.Join([]string{name, suffix}, "_")
}

// MangleVarName makes sure a reserved word gets a safe name.
func (l *Options) MangleVarName(name string) string {
	nm := l.Mangler.ToVarName(name)
	if _, ok := l.reservedWordsSet[nm]; !ok {
		return nm
	}

	return nm + "Var"
}

// MangleFileName makes sure a file name gets a safe name.
func (l *Options) MangleFileName(name string) string {
	if l.fileNameFunc != nil {
		return l.fileNameFunc(name)
	}

	return l.Mangler.ToFileName(name)
}

// ManglePackageName makes sure a package gets a safe name.
// In case of a file system path (e.g. name contains "/" or "\" on Windows), this return only the last element.
func (l *Options) ManglePackageName(name, suffix string) string {
	if name == "" {
		return suffix
	}
	if l.dirNameFunc != nil {
		name = l.dirNameFunc(name)
	}
	pth := filepath.ToSlash(filepath.Clean(name)) // preserve path
	pkg := importAlias(pth)                       // drop path
	return l.MangleName(l.Mangler.ToFileName(golangfuncs.PrefixForName(pkg)+pkg), suffix)
}

// ManglePackagePath makes sure a full package path gets a safe name.
// Only the last part of the path is altered.
func (l *Options) ManglePackagePath(name string, suffix string) string {
	if name == "" {
		return suffix
	}
	target := filepath.ToSlash(filepath.Clean(name)) // preserve path
	parts := strings.Split(target, "/")
	parts[len(parts)-1] = l.ManglePackageName(parts[len(parts)-1], suffix)

	return strings.Join(parts, "/")
}

// FormatContent formats a file with a language specific formatter.
func (l *Options) FormatContent(name string, content []byte, opts ...FormatOption) ([]byte, error) {
	if l.formatFunc != nil {
		return l.formatFunc(name, content, opts...)
	}

	// unformatted content
	return content, nil
}

// Imports generates the code to import some external packages, possibly aliased.
func (l *Options) Imports(imports map[string]string) string {
	if l.ImportsFunc != nil {
		return l.ImportsFunc(imports)
	}

	return ""
}

// ArrayInitializer builds a literal array.
func (l *Options) ArrayInitializer(data any) (string, error) {
	if l.ArrayInitializerFunc != nil {
		return l.ArrayInitializerFunc(data)
	}

	return "", nil
}

// BaseImport figures out the base path to generate import statements.
func (l *Options) BaseImport(tgt string) string {
	if l.BaseImportFunc != nil {
		return l.BaseImportFunc(tgt)
	}

	return ""
}

// importAlias extracts the last path component from a package import path.
func importAlias(pkg string) string {
	_, k := path.Split(pkg)
	return k
}
