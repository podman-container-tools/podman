// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package packages

import "strings"

// GoEnv is the part of the go environment that decides WHAT gets built, as opposed to how fast or where the output
// lands.
//
// Every field here changes the set of files a package is made of, or the set of packages a pattern matches, so it
// changes the emitted spec.
//
// That is why they are parameters rather than ambient state: a scan that silently inherits them from whatever shell it
// happened to start in is not reproducible, and — as GOOS/GOARCH proved — an inherited value reaches one loading
// strategy and not the other.
//
// An empty field means "whatever Config.Env says, and failing that the process environment", which is what
// packages.Load does.
// Set a field to pin it.
type GoEnv struct {
	// GOOS and GOARCH select the platform the scanned code is built for: //go:build lines and _linux.go / _amd64.go
	// filename suffixes resolve against them.
	GOOS   string
	GOARCH string

	// GOFLAGS supplies default command-line flags, in the go command's own format ("-tags=integration -mod=vendor").
	//
	// Flags given explicitly in Config.BuildFlags win, as they do for the go command.
	//
	// Only the flags that change what is built are interpreted by the toolchain-free strategy; the go/packages strategy
	// passes the whole string through to `go list`, which understands all of them.
	GOFLAGS string

	// GOWORK selects the workspace.
	//
	// "off" disables workspace mode; a path names a go.work file explicitly; empty means search upwards from Config.Dir,
	// as the go command does.
	//
	// It matters most for imports: inside a workspace, a module listed in `use` resolves to its directory rather than to
	// the module cache, and missing that means reading a stale copy — or synthesizing an empty one.
	GOWORK string

	// GOEXPERIMENT enables toolchain experiments, in the go command's format ("jsonv2,noaliastypeparams").
	//
	// Each enabled experiment contributes a goexperiment.<name> build tag.
	//
	// The baseline is the set the codescan binary was itself built with, since go/build computes ToolTags at init from its
	// own build configuration and there is no way to ask it about another toolchain.
	// This is exact when both were built by the same Go release, which is the ordinary case, and approximate otherwise.
	GOEXPERIMENT string
}

// WithGoEnv pins the parts of the go environment that decide what gets built.
//
// It replaces the older WithTarget: GOOS and GOARCH were never the only environment variables that change the answer,
// and having one of them explicit while the rest stayed ambient let the two strategies build for different
// platforms without anyone noticing.
func WithGoEnv(env GoEnv) Option {
	return func(o *options) { o.goEnv = env }
}

// assignments renders the pinned fields as environment assignments, for a child `go` process.
func (e GoEnv) assignments() []string {
	var out []string
	for _, kv := range [][2]string{
		{"GOOS", e.GOOS},
		{"GOARCH", e.GOARCH},
		{"GOFLAGS", e.GOFLAGS},
		{"GOWORK", e.GOWORK},
		{"GOEXPERIMENT", e.GOEXPERIMENT},
	} {
		if kv[1] != "" {
			out = append(out, kv[0]+"="+kv[1])
		}
	}

	return out
}

// resolve fills empty fields from env, which is Config.Env or the process environment.
//
// The precedence — explicit field, then Config.Env, then the process — mirrors packages.Load, where an assignment
// in Config.Env overrides the inherited one.
func (e GoEnv) resolve(env map[string]string) GoEnv {
	for _, f := range []struct {
		dst *string
		key string
	}{
		{&e.GOOS, "GOOS"},
		{&e.GOARCH, "GOARCH"},
		{&e.GOFLAGS, "GOFLAGS"},
		{&e.GOWORK, "GOWORK"},
		{&e.GOEXPERIMENT, "GOEXPERIMENT"},
	} {
		if *f.dst == "" {
			*f.dst = env[f.key]
		}
	}

	return e
}

// experimentTags renders GOEXPERIMENT as build tags to add and remove.
//
// The go command spells a disabled experiment "nojsonv2", and "none" clears the lot.
// Removals matter as much as additions here: the baseline is whatever codescan itself was built with, so an experiment
// the caller did NOT ask for may still be present and has to be taken away.
func (e GoEnv) experimentTags() (add, remove []string) {
	for name := range strings.SplitSeq(e.GOEXPERIMENT, ",") {
		name = strings.TrimSpace(name)
		switch {
		case name == "":
		case name == "none":
			return nil, nil // handled by the caller, which drops every goexperiment tag
		case strings.HasPrefix(name, "no"):
			remove = append(remove, "goexperiment."+strings.TrimPrefix(name, "no"))
		default:
			add = append(add, "goexperiment."+name)
		}
	}

	return add, remove
}

// clearsExperiments reports whether GOEXPERIMENT asks for a clean slate.
func (e GoEnv) clearsExperiments() bool {
	for name := range strings.SplitSeq(e.GOEXPERIMENT, ",") {
		if strings.TrimSpace(name) == "none" {
			return true
		}
	}

	return false
}
