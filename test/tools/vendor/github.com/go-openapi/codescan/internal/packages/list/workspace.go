// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package list

import (
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// workspace is the part of a go.work file that changes where imports resolve.
//
// A workspace exists to make sibling modules resolve to their working copies rather than to the module cache — that
// is the whole feature.
// Missing it does not fail loudly: the import is looked up in the cache, missed, and synthesized, so the sibling's
// types come back with no fields and no method set, and the spec comes out quietly thinner.
type workspace struct {
	// dirs maps a module path to the directory holding it, for every module named by a `use` directive.
	dirs map[string]string

	// replaces holds go.work `replace` directives, which override the members' own go.mod replaces.
	replaces map[string]string
}

// findWorkspace locates and reads the go.work governing dir.
//
// GOWORK follows the go command: "off" disables workspace mode, a path names the file, and empty means search upwards
// from dir.
// It returns nil whenever there is no workspace to honour, which is the common case and not an error.
func (r *Resolver) findWorkspace(gowork string) *workspace {
	if gowork == "off" {
		return nil
	}

	path := gowork
	if path == "" {
		path = r.searchUp("go.work")
	}
	if path == "" {
		return nil
	}

	blob, err := r.vfs.ReadFile(path)
	if err != nil {
		return nil
	}
	wf, err := modfile.ParseWork(path, blob, nil)
	if err != nil {
		return nil // an unreadable go.work degrades to no workspace rather than failing the scan
	}

	root := r.vfs.Join(path, "..")
	ws := &workspace{dirs: map[string]string{}, replaces: map[string]string{}}

	for _, use := range wf.Use {
		dir := use.Path
		if !r.vfs.IsAbs(dir) {
			dir = r.vfs.Join(root, dir)
		}
		// ModulePath is the authority: go.work's optional ModulePath field is only a hint, and a `use` directive names a
		// directory, not a module.
		if mp := r.modulePathAt(dir); mp != "" {
			ws.dirs[mp] = dir
		}
	}

	for _, rep := range wf.Replace {
		if rep.New.Version == "" { // filesystem replacement
			dir := rep.New.Path
			if !r.vfs.IsAbs(dir) {
				dir = r.vfs.Join(root, dir)
			}
			ws.replaces[rep.Old.Path] = dir

			continue
		}
		if cache := r.moduleCache(); cache != "" {
			if esc, err := module.EscapePath(rep.New.Path); err == nil {
				ws.replaces[rep.Old.Path] = r.vfs.Join(cache, esc+"@"+rep.New.Version)
			}
		}
	}

	return ws
}

// modulePathAt reads the module path declared by the go.mod in dir, or "" if there is none.
func (r *Resolver) modulePathAt(dir string) string {
	blob, err := r.vfs.ReadFile(r.vfs.Join(dir, "go.mod"))
	if err != nil {
		return ""
	}

	return modfile.ModulePath(blob)
}

// searchUp walks up from r.dir looking for name, returning its full path or "".
func (r *Resolver) searchUp(name string) string {
	dir := r.dir
	for range maxParentWalk {
		candidate := r.vfs.Join(dir, name)
		if _, err := r.vfs.ReadFile(candidate); err == nil {
			return candidate
		}

		parent := r.vfs.Join(dir, "..")
		if parent == dir || dir == "." || dir == "/" {
			return ""
		}
		dir = parent
	}

	return ""
}

// lookup maps an import path onto a workspace module directory.
//
// `replace` in go.work wins over `use`, matching the go command: a workspace-level replacement is meant to override
// whatever the members agreed among themselves.
func (w *workspace) lookup(importPath string) (modPath, dir string, ok bool) {
	if w == nil {
		return "", "", false
	}

	if p, d, found := longestModuleMatch(w.replaces, importPath); found {
		return p, d, true
	}

	return longestModuleMatch(w.dirs, importPath)
}

// longestModuleMatch finds the entry whose module path is the longest prefix of importPath.
//
// Longest wins because module paths nest: with both example.com/m and example.com/m/sub in a workspace, an import of
// example.com/m/sub/pkg belongs to the second, and map iteration order must not decide which.
func longestModuleMatch(m map[string]string, importPath string) (modPath, dir string, ok bool) {
	best := ""
	for p := range m {
		if importPath != p && !hasPathPrefix(importPath, p) {
			continue
		}
		if len(p) > len(best) {
			best = p
		}
	}
	if best == "" {
		return "", "", false
	}

	return best, m[best], true
}

// hasPathPrefix reports whether p starts with prefix at a path-segment boundary.
func hasPathPrefix(p, prefix string) bool {
	return len(p) > len(prefix) && p[len(prefix)] == '/' && p[:len(prefix)] == prefix
}
