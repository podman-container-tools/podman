// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-openapi/analysis"
	"github.com/go-openapi/runtime"

	"github.com/go-swagger/go-swagger/generator/internal/language"
	templatesrepo "github.com/go-swagger/go-swagger/generator/internal/templates-repo"
)

// Prepare finalizes a set of generation options so they are ready for use.
//
// It is the single entry point that turns a freshly populated GenOpts into a
// fully usable one: it validates the inputs, builds the derived machinery
// (language options, template func map, templates repository), resolves the
// render plan (sections), normalizes paths, checks the generation target
// and loads any user-provided templates.
//
// Because every input is known by the time Prepare runs, the derived state is
// built exactly once, in a deterministic order — which removes the historical
// ordering pitfalls of the EnsureDefaults / CheckOpts / setTemplates sequence.
//
// Prepare is idempotent: calling it again is a no-op.
func (g *GenOpts) Prepare() error {
	// validate first: it is pure and tolerates a nil receiver, so a bad-input
	// (or nil) options value is reported before any mutation.
	if err := g.validate(); err != nil {
		return err
	}

	if g.prepared {
		return nil
	}

	g.buildMachinery()

	if err := g.resolveSections(); err != nil {
		return err
	}

	if err := g.normalize(); err != nil {
		return err
	}

	// the target is checked after the spec has been resolved,
	// so a run that fails early doesn't leave an empty target tree behind.
	if err := g.ensureTarget(); err != nil {
		return err
	}

	if err := g.loadTemplates(); err != nil {
		return err
	}

	g.prepared = true

	return nil
}

// validate carries out the pure consistency checks on the options.
//
// It performs no mutation, so a failure here never leaves the options in a
// half-built state.
func (g *GenOpts) validate() error {
	if g == nil {
		return errors.New("gen opts are required")
	}

	if !filepath.IsAbs(g.Target) {
		if _, err := filepath.Abs(g.Target); err != nil {
			return fmt.Errorf("could not locate target %s: %w", g.Target, err)
		}
	}

	if filepath.IsAbs(g.ServerPackage) {
		return fmt.Errorf("you shouldn't specify an absolute path in --server-package: %s", g.ServerPackage)
	}

	return nil
}

// buildMachinery builds the deterministic, infallible derived state from the
// options: language options (including custom formatter and extra initialisms),
// the template func map and the templates repository preloaded with the
// embedded default assets.
//
// It is guarded so the machinery is built exactly once, regardless of how many
// times it is reached (the second call is a no-op).
//
// Failure to load the embedded default assets is a build-time impossibility and
// is treated as a panic, hence the absence of an error return.
func (g *GenOpts) buildMachinery() {
	if g.machineryBuilt {
		return
	}

	if g.LanguageOpts == nil {
		g.LanguageOpts = language.GolangOpts(g.WithExtraInitialisms...)
	}

	g.funcMap = DefaultFuncMap(g.LanguageOpts)
	g.templates = templatesrepo.NewRepository(g.funcMap)
	if err := g.templates.LoadDefaults(defaultAssets()); err != nil {
		panic(
			fmt.Errorf("cannot load default assets: %w", err),
		)
	}
	g.templates.SetProtectedTemplates(defaultProtectedTemplates())

	// set defaults for flattening options
	if g.FlattenOpts == nil {
		g.FlattenOpts = &analysis.FlattenOpts{
			Minimal:      true,
			Verbose:      true,
			RemoveUnused: false,
			Expand:       false,
		}
	}

	if g.DefaultScheme == "" {
		g.DefaultScheme = defaultScheme
	}

	if g.DefaultConsumes == "" {
		g.DefaultConsumes = runtime.JSONMime
	}

	if g.DefaultProduces == "" {
		g.DefaultProduces = runtime.JSONMime
	}

	// always include validator with models
	g.IncludeValidator = true

	if g.Principal == "" {
		g.Principal = iface
		g.PrincipalCustomIface = false
	}

	if g.WithCustomFormatter {
		// whenever opting for the custom formatter, we leave the basic formatting to the standard
		// imports.Process and focus on a custom handling of imports.
		g.LanguageOpts.FormatOnly = true
		g.LanguageOpts.SetFormatFunc(language.FormatLite)
	}

	if len(g.WithExtraInitialisms) > 0 {
		g.LanguageOpts.ExtraInitialisms = g.WithExtraInitialisms
	}

	g.machineryBuilt = true
}

// resolveSections computes the render plan (which templates produce which
// files): it fills the default sections from the include flags and package
// layout, then layers any config-file `layout:` overrides on top.
//
// It is guarded so the plan is resolved exactly once.
func (g *GenOpts) resolveSections() error {
	if g.sectionsResolved {
		return nil
	}

	DefaultSectionOpts(g)

	if g.Viper != nil {
		var def LanguageDefinition
		if err := g.Viper.Unmarshal(&def); err != nil {
			return err
		}

		g.Sections = g.Sections.overrideWith(def.Layout)
	}

	g.sectionsResolved = true

	return nil
}

// normalize resolves and absolutizes the spec path.
//
// Remote specs (http/https) are left untouched. Local specs are located on
// disk and rewritten to an absolute path.
//
// It is guarded so the spec is resolved exactly once.
func (g *GenOpts) normalize() error {
	if g.specNormalized {
		return nil
	}

	if strings.HasPrefix(g.Spec, "http://") || strings.HasPrefix(g.Spec, "https://") {
		g.specNormalized = true

		return nil
	}

	pth, err := findSwaggerSpec(g.Spec)
	if err != nil {
		return err
	}

	// ensure spec path is absolute
	g.Spec, err = filepath.Abs(pth)
	if err != nil {
		return fmt.Errorf("could not locate spec: %s", g.Spec)
	}

	g.specNormalized = true

	return nil
}

// ensureTarget checks that the generation target is a directory this process may write to.
//
// An empty target means the current directory. A target that does not exist is an error,
// unless [GenOpts.EnsureTarget] is set: the directory is then created, with its parents.
//
// The check is skipped when dumping the template data ([GenOpts.DumpData]),
// since nothing is written to the target then.
//
// It is guarded so the target is checked exactly once.
func (g *GenOpts) ensureTarget() error {
	if g.targetEnsured {
		return nil
	}

	if g.Target == "" {
		g.Target = "."
	}

	if g.DumpData {
		return nil
	}

	info, err := os.Stat(g.Target)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) || !g.EnsureTarget {
			return fmt.Errorf("could not open target dir %q. Make sure it exists or use --ensure-target: %w", g.Target, err)
		}

		if err = os.MkdirAll(g.Target, readAllDir); err != nil {
			return fmt.Errorf("could not create target directory %q: %w", g.Target, err)
		}

		g.targetEnsured = true

		return nil
	}

	if !info.IsDir() { // Stat resolves symlinks
		return fmt.Errorf("target %q already exists and is not a directory. The target must be a writable directory", g.Target)
	}

	// check that this process may write files there
	probe, err := os.CreateTemp(g.Target, ".probe-")
	if err != nil {
		return fmt.Errorf(
			"target %q is not writeable. Make sure your command has the proper permissions to write in this folder: %w",
			g.Target, err,
		)
	}
	_ = probe.Close()
	_ = os.Remove(probe.Name())

	g.targetEnsured = true

	return nil
}
