// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strings"

	"golang.org/x/tools/imports"

	"github.com/go-openapi/swag"
)

var (
	// DefaultLanguageFunc defines the default generation language.
	DefaultLanguageFunc func() *LanguageOpts

	moduleRe *regexp.Regexp
)

const defaultIndent = 2

// FormatterFunc is a function that processes go code to reformat it, e.g. [golang.org/x/tools/imports.Process]).
//
// Formatting options allow for injecting a custom formatter for the generated code. See [WithCustomFormatter].
type FormatterFunc func(filename string, src []byte, opts ...FormatOption) ([]byte, error)

type mangleFunc func(string) string

func initLanguage() {
	DefaultLanguageFunc = GolangOpts

	moduleRe = regexp.MustCompile(`module[ \t]+([^\s]+)`)
}

// FormatOption allows for more flexible code formatting settings.
type FormatOption func(*formatOptions)

type formatOptions struct {
	imports.Options

	localPrefixes []string
}

// WithFormatLocalPrefixes adds local prefixes to group imports.
func WithFormatLocalPrefixes(prefixes ...string) FormatOption {
	return func(o *formatOptions) {
		o.localPrefixes = append(o.localPrefixes, prefixes...)
	}
}

// WithFormatOnly tells the formatter to skip imports processing.
func WithFormatOnly(enabled bool) FormatOption {
	return func(o *formatOptions) {
		o.FormatOnly = enabled
	}
}

var defaultFormatOptions = formatOptions{
	Options: imports.Options{
		TabIndent: true,
		TabWidth:  defaultIndent,
		Fragment:  true,
		Comments:  true,
	},
	localPrefixes: []string{"github.com/go-openapi"},
}

func formatOptionsWithDefault(opts []FormatOption) formatOptions {
	o := defaultFormatOptions

	for _, apply := range opts {
		apply(&o)
	}

	return o
}

// LanguageOpts to describe a language to the code generator.
type LanguageOpts struct {
	ReservedWords        []string
	BaseImportFunc       mangleFunc                     `json:"-"`
	ImportsFunc          func(map[string]string) string `json:"-"`
	ArrayInitializerFunc func(any) (string, error)      `json:"-"`
	FormatOnly           bool
	reservedWordsSet     map[string]struct{}
	initialized          bool
	formatFunc           FormatterFunc
	fileNameFunc         mangleFunc // language specific source file naming rules
	dirNameFunc          mangleFunc // language specific directory naming rules
}

// Init the language option.
func (l *LanguageOpts) Init() {
	if l.initialized {
		return
	}
	l.initialized = true
	l.reservedWordsSet = make(map[string]struct{})
	for _, rw := range l.ReservedWords {
		l.reservedWordsSet[rw] = struct{}{}
	}
}

// MangleName makes sure a reserved word gets a safe name.
func (l *LanguageOpts) MangleName(name, suffix string) string {
	if _, ok := l.reservedWordsSet[swag.ToFileName(name)]; !ok {
		return name
	}
	return strings.Join([]string{name, suffix}, "_")
}

// MangleVarName makes sure a reserved word gets a safe name.
func (l *LanguageOpts) MangleVarName(name string) string {
	nm := swag.ToVarName(name)
	if _, ok := l.reservedWordsSet[nm]; !ok {
		return nm
	}
	return nm + "Var"
}

// MangleFileName makes sure a file name gets a safe name.
func (l *LanguageOpts) MangleFileName(name string) string {
	if l.fileNameFunc != nil {
		return l.fileNameFunc(name)
	}
	return swag.ToFileName(name)
}

// ManglePackageName makes sure a package gets a safe name.
// In case of a file system path (e.g. name contains "/" or "\" on Windows), this return only the last element.
func (l *LanguageOpts) ManglePackageName(name, suffix string) string {
	if name == "" {
		return suffix
	}
	if l.dirNameFunc != nil {
		name = l.dirNameFunc(name)
	}
	pth := filepath.ToSlash(filepath.Clean(name)) // preserve path
	pkg := importAlias(pth)                       // drop path
	return l.MangleName(swag.ToFileName(prefixForName(pkg)+pkg), suffix)
}

// ManglePackagePath makes sure a full package path gets a safe name.
// Only the last part of the path is altered.
func (l *LanguageOpts) ManglePackagePath(name string, suffix string) string {
	if name == "" {
		return suffix
	}
	target := filepath.ToSlash(filepath.Clean(name)) // preserve path
	parts := strings.Split(target, "/")
	parts[len(parts)-1] = l.ManglePackageName(parts[len(parts)-1], suffix)
	return strings.Join(parts, "/")
}

// FormatContent formats a file with a language specific formatter.
func (l *LanguageOpts) FormatContent(name string, content []byte, opts ...FormatOption) ([]byte, error) {
	if l.formatFunc != nil {
		return l.formatFunc(name, content, opts...)
	}

	// unformatted content
	return content, nil
}

// imports generate the code to import some external packages, possibly aliased.
func (l *LanguageOpts) imports(imports map[string]string) string {
	if l.ImportsFunc != nil {
		return l.ImportsFunc(imports)
	}
	return ""
}

// arrayInitializer builds a litteral array.
func (l *LanguageOpts) arrayInitializer(data any) (string, error) {
	if l.ArrayInitializerFunc != nil {
		return l.ArrayInitializerFunc(data)
	}
	return "", nil
}

// baseImport figures out the base path to generate import statements.
func (l *LanguageOpts) baseImport(tgt string) string {
	if l.BaseImportFunc != nil {
		return l.BaseImportFunc(tgt)
	}
	debugLogf("base import func is nil")
	return ""
}

// GolangOpts for rendering items as golang code.
func GolangOpts() *LanguageOpts {
	opts := new(LanguageOpts)
	opts.ReservedWords = []string{
		"break", "default", "func", "interface", "select",
		"case", "defer", "go", "map", "struct",
		"chan", "else", "goto", "package", "switch",
		"const", "fallthrough", "if", "range", "type",
		"continue", "for", "import", "return", "var",
	}

	opts.formatFunc = defaultGoFormatFunc() // this default may be overridden by [GenOptsCommon]
	opts.fileNameFunc = defaultGoFilenameFunc(goOtherReservedSuffixes())
	opts.dirNameFunc = defaultGoDirnameFunc()
	opts.ImportsFunc = defaultGoImportsFunc()
	opts.ArrayInitializerFunc = defaultGoArrayInitializerFunc()
	opts.BaseImportFunc = defaultGoBaseImportFunc()

	opts.Init()

	return opts
}

func defaultGoFormatFunc() FormatterFunc {
	return func(ffn string, content []byte, fmtOpts ...FormatOption) ([]byte, error) {
		o := formatOptionsWithDefault(fmtOpts)
		imports.LocalPrefix = strings.Join(o.localPrefixes, ",") // regroup these packages
		return imports.Process(ffn, content, &o.Options)
	}
}

func defaultGoFilenameFunc(reservedSuffixes map[string]bool) mangleFunc {
	return func(name string) string {
		// whenever a generated file name ends with a suffix
		// that is meaningful to go build, adds a "swagger"
		// suffix
		parts := strings.Split(swag.ToFileName(name), "_")
		if reservedSuffixes[parts[len(parts)-1]] {
			// file name ending with a reserved arch or os name
			// are appended an innocuous suffix "swagger"
			parts = append(parts, "swagger")
		}
		return strings.Join(parts, "_")
	}
}

func defaultGoDirnameFunc() mangleFunc {
	return func(name string) string {
		// whenever a generated directory name is a special
		// golang directory, append an innocuous suffix
		switch name {
		case "vendor", "internal":
			return strings.Join([]string{name, "swagger"}, "_")
		}
		return name
	}
}

func defaultGoImportsFunc() func(map[string]string) string {
	return func(imports map[string]string) string {
		if len(imports) == 0 {
			return ""
		}
		result := make([]string, 0, len(imports))
		for k, v := range imports {
			_, name := path.Split(v)
			if name != k {
				result = append(result, fmt.Sprintf("\t%s %q", k, v))
			} else {
				result = append(result, fmt.Sprintf("\t%q", v))
			}
		}
		sort.Strings(result)
		return strings.Join(result, "\n")
	}
}

func defaultGoArrayInitializerFunc() func(any) (string, error) {
	return func(data any) (string, error) {
		// ArrayInitializer constructs a Go literal initializer from any literals.
		// e.g. []any{"a", "b"} is transformed in {"a","b",}
		// e.g. map[string]any{ "a": "x", "b": "y"} is transformed in {"a":"x","b":"y",}.
		//
		// NOTE: this is currently used to construct simple slice intializers for default values.
		// This allows for nicer slice initializers for slices of primitive types and avoid systematic use for json.Unmarshal().
		b, err := json.Marshal(data)
		if err != nil {
			return "", err
		}
		return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(string(b), "}", ",}"), "[", "{"), "]", ",}"), "{,}", "{}"), nil
	}
}

func defaultGoBaseImportFunc() mangleFunc {
	return func(target string) string {
		base, err := defaultGoBaseImportErr(target)
		if err != nil {
			fatalln(err)

			return ""
		}

		return base
	}
}

func defaultGoBaseImportErr(target string) (string, error) {
	target = filepath.Clean(target)
	// On Windows, filepath.Abs("") behaves differently than on Unix.
	// Windows: yields an error, since Abs() does not know the volume.
	// UNIX: returns current working directory
	if target == "" {
		target = "."
	}

	targetAbsPath, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("could not evaluate base import path with target %q: %w", target, err)
	}

	targetAbsPathExtended, err := filepath.EvalSymlinks(targetAbsPath)
	if err != nil {
		return "", fmt.Errorf("could not evaluate base import path with target %q (with symlink resolution): %w", targetAbsPath, err)
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		homeDir, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("could not evaluate home dir for current user: %w", herr)
		}

		gopath = filepath.Join(homeDir, "go")
	}

	pth, err := exploreGoPath(gopath, targetAbsPath, targetAbsPathExtended)
	if err != nil {
		return "", err
	}

	mod, goModuleAbsPath, err := tryResolveModule(targetAbsPath)
	switch {
	case err != nil:
		return "", fmt.Errorf("failed to resolve module using go.mod file: %w", err)
	case mod != "":
		relTgt := relPathToRelGoPath(goModuleAbsPath, targetAbsPath)
		if !strings.HasSuffix(mod, relTgt) {
			return filepath.ToSlash(mod + relTgt), nil
		}

		return filepath.ToSlash(mod), nil
	}

	if pth == "" {
		return "", errors.New("target must reside inside a location within $GOPATH/src or be a module")
	}

	return filepath.ToSlash(pth), nil
}

func exploreGoPath(gopath, targetAbsPath, targetAbsPathExtended string) (pth string, err error) {
	for _, gp := range filepath.SplitList(gopath) {
		_, err := os.Stat(filepath.Join(gp, "src"))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return "", err
		}

		// EvalSymLinks also calls the Clean
		gopathExtended, err := filepath.EvalSymlinks(gp)
		if err != nil {
			return "", err
		}

		gopathExtended = filepath.Join(gopathExtended, "src")
		gp = filepath.Join(gp, "src")

		// At this stage we have expanded and unexpanded target path. GOPATH is fully expanded.
		// Expanded means symlink free.
		// We compare both types of targetpath<s> with gopath.
		// If any one of them coincides with gopath , it is imperative that
		// target path lies inside gopath. How?
		// 		- Case 1: Irrespective of symlinks paths coincide. Both non-expanded paths.
		// 		- Case 2: Symlink in target path points to location inside GOPATH. (Expanded Target Path)
		//    - Case 3: Symlink in target path points to directory outside GOPATH (Unexpanded target path)

		// Case 1: - Do nothing case. If non-expanded paths match just generate base import path as if
		//				   there are no symlinks.

		// Case 2: - Symlink in target path points to location inside GOPATH. (Expanded Target Path)
		//					 First if will fail. Second if will succeed.

		// Case 3: - Symlink in target path points to directory outside GOPATH (Unexpanded target path)
		// 					 First if will succeed and break.

		// compares non expanded path for both
		if ok, relativepath := checkPrefixAndFetchRelativePath(targetAbsPath, gp); ok {
			pth = relativepath
			break
		}

		// Compares non-expanded target path
		if ok, relativepath := checkPrefixAndFetchRelativePath(targetAbsPath, gopathExtended); ok {
			pth = relativepath
			break
		}

		// Compares expanded target path.
		if ok, relativepath := checkPrefixAndFetchRelativePath(targetAbsPathExtended, gopathExtended); ok {
			pth = relativepath
			break
		}
	}

	return pth, nil
}

// resolveGoModFile walks up the directory tree starting from 'dir' until it
// finds a go.mod file. If go.mod is found it will return the related file
// object. If no go.mod file is found it will return an error.
func resolveGoModFile(dir string) (*os.File, string, error) {
	goModPath := filepath.Join(dir, "go.mod")
	f, err := os.Open(goModPath)
	if err != nil {
		if os.IsNotExist(err) && dir != filepath.Dir(dir) {
			return resolveGoModFile(filepath.Dir(dir))
		}

		return nil, "", err
	}

	return f, dir, nil
}

// relPathToRelGoPath takes a relative os path and returns the relative go
// package path. For unix nothing will change but for windows \ will be
// converted to /.
func relPathToRelGoPath(modAbsPath, absPath string) string {
	if absPath == "." {
		return ""
	}

	path := strings.TrimPrefix(absPath, modAbsPath)
	pathItems := strings.Split(path, string(filepath.Separator))
	return strings.Join(pathItems, "/")
}

func tryResolveModule(baseTargetPath string) (string, string, error) {
	f, goModAbsPath, err := resolveGoModFile(baseTargetPath)
	switch {
	case os.IsNotExist(err):
		return "", "", nil
	case err != nil:
		return "", "", err
	}
	defer func() {
		_ = f.Close()
	}()

	src, err := io.ReadAll(f)
	if err != nil {
		return "", "", err
	}

	match := moduleRe.FindSubmatch(src)
	const matchSubExpression = 2
	if len(match) != matchSubExpression {
		return "", "", nil
	}

	return string(match[1]), goModAbsPath, nil
}

// 1. Checks if the child path and parent path coincide.
// 2. If they do return child path  relative to parent path.
// 3. Everything else return false.
func checkPrefixAndFetchRelativePath(childpath string, parentpath string) (bool, string) {
	// Windows (local) file systems - NTFS, as well as FAT and variants
	// are case insensitive.
	cp, pp := childpath, parentpath
	if goruntime.GOOS == winOS {
		cp = strings.ToLower(cp)
		pp = strings.ToLower(pp)
	}

	if strings.HasPrefix(cp, pp) {
		pth, err := filepath.Rel(parentpath, childpath)
		if err != nil {
			fatalln(err)
		}
		return true, pth
	}

	return false, ""
}

func goOtherReservedSuffixes() map[string]bool {
	// see:
	// https://golang.org/src/go/build/syslist.go
	// https://golang.org/doc/install/source#environment

	return map[string]bool{
		// goos
		"aix":       true,
		"android":   true,
		"darwin":    true,
		"dragonfly": true,
		"freebsd":   true,
		"hurd":      true,
		"illumos":   true,
		"ios":       true,
		"js":        true,
		"linux":     true,
		"nacl":      true,
		"netbsd":    true,
		"openbsd":   true,
		"plan9":     true,
		"solaris":   true,
		"windows":   true,
		"zos":       true,

		// arch
		"386":         true,
		"amd64":       true,
		"amd64p32":    true,
		"arm":         true,
		"armbe":       true,
		"arm64":       true,
		"arm64be":     true,
		"loong64":     true,
		"mips":        true,
		"mipsle":      true,
		"mips64":      true,
		"mips64le":    true,
		"mips64p32":   true,
		"mips64p32le": true,
		"ppc":         true,
		"ppc64":       true,
		"ppc64le":     true,
		"riscv":       true,
		"riscv64":     true,
		"s390":        true,
		"s390x":       true,
		"sparc":       true,
		"sparc64":     true,
		"wasm":        true,

		// other reserved suffixes
		"test": true,
	}
}
