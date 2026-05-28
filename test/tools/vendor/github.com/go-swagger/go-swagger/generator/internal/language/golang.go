// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package language

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

var moduleRe = regexp.MustCompile(`module[ \t]+([^\s]+)`)

// GolangOpts returns [Options] for rendering items as golang code.
func GolangOpts() *Options {
	opts := new(Options)
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
		o := FormatOptsWithDefault(fmtOpts)
		imports.LocalPrefix = strings.Join(o.LocalPrefixes, ",") // regroup these packages
		return imports.Process(ffn, content, &o.Options)
	}
}

func defaultGoFilenameFunc(reservedSuffixes map[string]bool) MangleFunc {
	return func(name string) string {
		parts := strings.Split(swag.ToFileName(name), "_") //nolint:staticcheck // tracked for migration to mangling.NameMangler
		if reservedSuffixes[parts[len(parts)-1]] {
			parts = append(parts, "swagger")
		}
		return strings.Join(parts, "_")
	}
}

func defaultGoDirnameFunc() MangleFunc {
	return func(name string) string {
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
		b, err := json.Marshal(data)
		if err != nil {
			return "", err
		}
		return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(string(b), "}", ",}"), "[", "{"), "]", ",}"), "{,}", "{}"), nil
	}
}

func defaultGoBaseImportFunc() MangleFunc {
	return func(target string) string {
		base, err := defaultGoBaseImportErr(target)
		if err != nil {
			// NOTE: historically this called log.Fatalln. We now panic to avoid
			// pulling in generator-specific logging, while preserving the "fail hard" semantics.
			panic(fmt.Sprintf("base import resolution failed: %v", err))
		}

		return base
	}
}

// DefaultGoBaseImportErr resolves the Go import path for the given target directory.
func DefaultGoBaseImportErr(target string) (string, error) {
	return defaultGoBaseImportErr(target)
}

func defaultGoBaseImportErr(target string) (string, error) {
	target = filepath.Clean(target)
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
		_, err := os.Stat(filepath.Join(gp, "src")) //nolint:gosec // GOPATH traversal is expected
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			return "", err
		}

		gopathExtended, err := filepath.EvalSymlinks(gp)
		if err != nil {
			return "", err
		}

		gopathExtended = filepath.Join(gopathExtended, "src")
		gp = filepath.Join(gp, "src")

		if ok, relativepath := CheckPrefixAndFetchRelativePath(targetAbsPath, gp); ok {
			pth = relativepath
			break
		}

		if ok, relativepath := CheckPrefixAndFetchRelativePath(targetAbsPath, gopathExtended); ok {
			pth = relativepath
			break
		}

		if ok, relativepath := CheckPrefixAndFetchRelativePath(targetAbsPathExtended, gopathExtended); ok {
			pth = relativepath
			break
		}
	}

	return pth, nil
}

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

// CheckPrefixAndFetchRelativePath checks if childpath is under parentpath
// and returns the relative path if so.
func CheckPrefixAndFetchRelativePath(childpath string, parentpath string) (bool, string) {
	cp, pp := childpath, parentpath
	if goruntime.GOOS == "windows" {
		cp = strings.ToLower(cp)
		pp = strings.ToLower(pp)
	}

	if strings.HasPrefix(cp, pp) {
		pth, err := filepath.Rel(parentpath, childpath)
		if err != nil {
			return false, ""
		}
		return true, pth
	}

	return false, ""
}

func goOtherReservedSuffixes() map[string]bool {
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
