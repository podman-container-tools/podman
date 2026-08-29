// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package scanner

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"iter"
	"maps"
	"slices"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/swag/mangling"
	"golang.org/x/tools/go/packages"
)

// ErrDegradedLoad is the base error for a degraded package load detected by detectDegradedLoad (no
// packages matched, or a scanned package failed to load / type-check).
//
// It is wrapped with the per-package detail and, at the public API boundary, with ErrCodeScan.
var ErrDegradedLoad = errors.New("degraded package load")

const pkgLoadMode = packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo

type node uint32

const (
	metaNode node = 1 << iota
	routeNode
	operationNode
	modelNode
	parametersNode
	responseNode
)

type ScanCtx struct {
	pkgs []*packages.Package
	app  *TypeIndex

	opts *Options

	// paramOrigins captures (operationID → parameterName → source position) during the parameters
	// build.
	//
	// Parameter anchors can't be emitted inline: at parameters-build time the operation isn't yet
	// bound to a path/method and the array index isn't final.
	// They are resolved in a deferred pass (see the spec builder) once paths are built.
	// Cross-ref linkage only.
	paramOrigins map[string]map[string]token.Position

	// defOrigins buffers definition-scoped provenance anchors (the definition node and every
	// field/enum sub-anchor under it), keyed by the fully-qualified definition key
	// (EntityDecl.DefKey).
	//
	// They cannot fire inline: the definition is keyed by its fqn during discovery but renamed to its
	// final user-facing name only at the end of the build (reduceDefinitionNames), and an unreferenced
	// definition may be pruned before that.
	//
	// Buffered here, then re-pointed to the final name and emitted by FlushDefOrigins after prune +
	// name reduction, so every pointer handed to OnProvenance resolves against the final document.
	// curDefKey marks the definition currently being built (empty outside a definition build);
	// non-reentrant, since each definition is built in its own pass.
	defOrigins map[string][]Provenance
	curDefKey  string

	// deferredOrigins buffers provenance anchors for top-level spec nodes that may be pruned after the
	// build (shared responses under PruneUnusedModels), keyed by an arbitrary caller-chosen key.
	//
	// Unlike defOrigins these are flushed verbatim — the nodes are never renamed, only possibly
	// dropped — so a pruned node's anchors can be discarded (DropDeferredOrigins) before
	// FlushDeferredOrigins fires the survivors. curDeferredKey marks the node currently being built
	// (empty outside such a window).
	//
	// Distinct from defOrigins: a definition build is never nested inside a deferred window.
	deferredOrigins map[string][]Provenance
	curDeferredKey  string

	// seenDiags suppresses exact-duplicate diagnostics on the OnDiagnostic stream over one scan (see
	// EmitDiagnostic).
	seenDiags map[diagKey]struct{}

	// mangler is the shared name mangler used for godoc humanization (the CleanGoDoc option) and
	// reusable by any builder needing swag-style name transforms.
	//
	// Constructed once per scan; its methods are pool-backed and safe to call across the (currently
	// sequential) per-decl builds.
	mangler *mangling.NameMangler
}

func NewScanCtx(opts *Options) (*ScanCtx, error) {
	cfg := &packages.Config{
		Dir:   opts.WorkDir,
		Mode:  pkgLoadMode,
		Tests: false,
	}
	if opts.BuildTags != "" {
		cfg.BuildFlags = []string{"-tags", opts.BuildTags}
	}

	pkgs, err := packages.Load(cfg, opts.Packages...)
	if err != nil {
		return nil, err
	}
	if err := detectDegradedLoad(pkgs, opts); err != nil {
		return nil, err
	}

	app, err := NewTypeIndex(pkgs,
		WithExcludeDeps(opts.ExcludeDeps),
		WithIncludeTags(sliceToSet(opts.IncludeTags)),
		WithExcludeTags(sliceToSet(opts.ExcludeTags)),
		WithIncludePkgs(opts.Include),
		WithExcludePkgs(opts.Exclude),
		WithXNullableForPointers(opts.SetXNullableForPointers),
		WithRefAliases(opts.RefAliases),
		WithTransparentAliases(opts.TransparentAliases),
		WithAfterDeclComments(opts.AfterDeclComments),
		WithOnDiagnostic(opts.OnDiagnostic),
	)
	if err != nil {
		return nil, err
	}

	mangler := mangling.NewNameMangler()

	return &ScanCtx{
		pkgs:    pkgs,
		app:     app,
		opts:    opts,
		mangler: &mangler,
	}, nil
}

// detectDegradedLoad reacts to a degraded `packages.Load` result. packages.Load only returns the
// catastrophic error; degraded-but-loaded states otherwise pass silently and produce an incomplete
// spec.
//
// The reaction is tiered by what is still recoverable — only the pattern-matched root packages
// are inspected (transitive deps live in the import graph and are not scanned, so dep noise does
// not trip the check):
//
//   - ABORT (Error + returned error) when nothing usable loaded: no packages
//     matched the patterns; a root package could not be loaded at all (a
//     packages.ListError — e.g. a missing directory or unresolved import,
//     where "code must build" cannot even be met); or a root package came back
//     without type information (Types/TypesInfo nil — the #2874 wholesale
//     type-check failure where swagger:allOf silently stops resolving).
//   - WARN (and continue) when a root package carries only parse/type errors
//     but still has usable type information. go/packages type-checks
//     best-effort, so its scannable definitions remain usable; a single
//     non-building package must not sink a whole `./...` scan. The spec is
//     emitted from what loaded, with the affected package flagged.
//
// Every observation is reported through opts.OnDiagnostic as a scan.degraded-load diagnostic; abort
// observations are also summarised in the returned (wrapped ErrDegradedLoad) error.
func detectDegradedLoad(pkgs []*packages.Package, opts *Options) error {
	emit := func(sev grammar.Severity, format string, args ...any) string {
		ctor := grammar.Errorf
		if sev == grammar.SeverityWarning {
			ctor = grammar.Warnf
		}
		d := ctor(token.Position{}, grammar.CodeDegradedLoad, format, args...)
		if cb := opts.OnDiagnostic; cb != nil {
			cb(d)
		}
		return d.Message
	}

	if len(pkgs) == 0 {
		return fmt.Errorf("%w: %s", ErrDegradedLoad,
			emit(grammar.SeverityError, "no packages matched the scan patterns %v in %q", opts.Packages, opts.WorkDir))
	}

	var fatal []string
	for _, pkg := range pkgs {
		switch {
		case hasListError(pkg.Errors):
			fatal = append(fatal, emit(grammar.SeverityError,
				"package %q could not be loaded: %s", pkg.PkgPath, firstListError(pkg.Errors)))
		case pkg.Types == nil || pkg.TypesInfo == nil:
			fatal = append(fatal, emit(grammar.SeverityError,
				"package %q loaded without type information; swagger:allOf / $ref resolution would be incomplete",
				pkg.PkgPath))
		case len(pkg.Errors) > 0:
			emit(grammar.SeverityWarning,
				"package %q did not fully type-check: %s (%d error(s)); its definitions may be incomplete",
				pkg.PkgPath, pkg.Errors[0], len(pkg.Errors))
		}
	}
	if len(fatal) > 0 {
		return fmt.Errorf("%w: %s", ErrDegradedLoad, strings.Join(fatal, "; "))
	}

	return nil
}

// hasListError reports whether any error is a packages.ListError — the package or pattern could
// not be loaded at all (vs. a parse/type error on code that did load).
func hasListError(errs []packages.Error) bool {
	for _, e := range errs {
		if e.Kind == packages.ListError {
			return true
		}
	}

	return false
}

// firstListError returns the first packages.ListError for messaging; callers guard with
// hasListError.
func firstListError(errs []packages.Error) packages.Error {
	for _, e := range errs {
		if e.Kind == packages.ListError {
			return e
		}
	}

	return packages.Error{}
}

func (s *ScanCtx) SkipExtensions() bool {
	return s.opts.SkipExtensions
}

func (s *ScanCtx) SkipEnumDescriptions() bool {
	return s.opts.SkipEnumDescriptions
}

func (s *ScanCtx) EmitXGoType() bool {
	return s.opts.EmitXGoType
}

func (s *ScanCtx) SingleLineCommentAsDescription() bool {
	return s.opts.SingleLineCommentAsDescription
}

// CleanGoDoc reports whether godoc-syntax filtering is enabled (Options.CleanGoDoc).
func (s *ScanCtx) CleanGoDoc() bool {
	return s.opts.CleanGoDoc
}

// Mangler returns the scan's shared name mangler (swag-style name transforms).
func (s *ScanCtx) Mangler() *mangling.NameMangler {
	return s.mangler
}

func (s *ScanCtx) DescWithRef() bool {
	return s.opts.DescWithRef
}

func (s *ScanCtx) SkipAllOfCompounding() bool {
	return s.opts.SkipAllOfCompounding
}

// DefaultAllOfForEmbeds reports whether plain struct embeds should render as allOf composition
// instead of inlined properties (Options.DefaultAllOfForEmbeds).
func (s *ScanCtx) DefaultAllOfForEmbeds() bool {
	return s.opts.DefaultAllOfForEmbeds
}

func (s *ScanCtx) EmitRefSiblings() bool {
	return s.opts.EmitRefSiblings
}

func (s *ScanCtx) SetXNullableForPointers() bool {
	return s.opts.SetXNullableForPointers
}

// NameFromTags returns the ordered list of struct-tag types consulted to derive a field's emitted
// name.
//
// A nil/unset option defaults to ["json"] (the historic behaviour); an explicit empty slice means
// no tag is consulted and names fall back to the Go field name.
func (s *ScanCtx) NameFromTags() []string {
	if s.opts.NameFromTags == nil {
		return []string{"json"}
	}
	return s.opts.NameFromTags
}

// SkipJSONifyInterfaceMethods reports whether the interface-method auto-jsonify mangler is disabled
// (Options.SkipJSONifyInterfaceMethods).
//
// A `swagger:name` override is honored verbatim regardless.
func (s *ScanCtx) SkipJSONifyInterfaceMethods() bool {
	return s.opts.SkipJSONifyInterfaceMethods
}

func (s *ScanCtx) TransparentAliases() bool {
	return s.opts.TransparentAliases
}

func (s *ScanCtx) RefAliases() bool {
	return s.opts.RefAliases
}

// FileSet returns the shared *token.FileSet used by the scan's loaded packages.
//
// Callers that construct a grammar.Parser for comment groups not owned by a single EntityDecl's
// *packages.Package (notably operation and route path-level annotations aggregated across packages)
// read the FileSet from here so the produced positions resolve against the same file table the rest
// of the scan uses.
func (s *ScanCtx) FileSet() *token.FileSet {
	if len(s.pkgs) == 0 {
		return nil
	}
	return s.pkgs[0].Fset
}

// PosOf resolves p to a token.Position via the active FileSet.
//
// Returns the zero token.Position when p is invalid or no FileSet is available.
// Useful for attaching a source location to a Diagnostic without each caller re-deriving the
// FileSet.
func (s *ScanCtx) PosOf(p token.Pos) token.Position {
	if !p.IsValid() {
		return token.Position{}
	}
	fset := s.FileSet()
	if fset == nil {
		return token.Position{}
	}
	return fset.Position(p)
}

// diagKey identifies a diagnostic by its source location and content, for suppressing exact
// duplicates over the lifetime of one scan.
type diagKey struct {
	pos  string
	code grammar.Code
	msg  string
}

// EmitDiagnostic delivers d to the consumer's [Options.OnDiagnostic] sink, suppressing exact
// duplicates — same position, code and message — for the lifetime of the scan.
//
// The build re-processes the same field/annotation in several passes (most visibly a
// swagger:parameters struct applied to multiple operation ids, which rebuilds every field once per
// id), so the identical diagnostic would otherwise surface once per visit.
//
// The accumulator returned by common.Builder.Diagnostics() is unaffected — only the callback
// stream dedups.
func (s *ScanCtx) EmitDiagnostic(d grammar.Diagnostic) {
	cb := s.opts.OnDiagnostic
	if cb == nil {
		return
	}
	k := diagKey{pos: d.Pos.String(), code: d.Code, msg: d.Message}
	if _, dup := s.seenDiags[k]; dup { // read from a nil map is safe
		return
	}
	if s.seenDiags == nil {
		s.seenDiags = make(map[diagKey]struct{})
	}
	s.seenDiags[k] = struct{}{}
	cb(d)
}

// OnDiagnostic returns the user-supplied diagnostic sink, or nil when the consumer has not opted
// into diagnostic delivery.
//
// # Details
//
// See [§diagnostics](./README.md#diagnostics) — callback contract, ordering guarantee,
// experimental-API caveat.
func (s *ScanCtx) OnDiagnostic() func(grammar.Diagnostic) {
	return s.opts.OnDiagnostic
}

// NameConcatBudget returns the caller-supplied readability budget for collision-deconflicted
// definition names, or 0 when unset — the spec builder substitutes its built-in default in that
// case.
func (s *ScanCtx) NameConcatBudget() float64 {
	return s.opts.NameConcatBudget
}

// EmitHierarchicalNames reports whether the caller opted into the hierarchical fail-safe for
// over-budget collision groups.
func (s *ScanCtx) EmitHierarchicalNames() bool {
	return s.opts.EmitHierarchicalNames
}

// PruneUnusedModels reports whether the caller opted into pruning discovered definitions that are
// not transitively referenced from a root (paths, shared responses/parameters, overlay
// definitions).
//
// See [Options.PruneUnusedModels].
func (s *ScanCtx) PruneUnusedModels() bool {
	return s.opts.PruneUnusedModels
}

// OriginEnabled reports whether a provenance sink is wired, so callers can skip JSON-pointer
// construction entirely when no consumer is listening.
func (s *ScanCtx) OriginEnabled() bool {
	return s.opts.OnProvenance != nil
}

// RecordOrigin fires the consumer's [Options.OnProvenance] callback for one anchor node, when
// wired.
//
// Unlike diagnostics it accumulates nothing — the cross-ref index is owned by the consumer (see
// the genspec-tui linkage design).
//
// Exception: while a definition build is in progress (between [BeginDefOrigins] and
// [EndDefOrigins]) the anchor is buffered instead of fired, so it can be re-pointed to the
// definition's final name — or dropped if the definition is pruned — by [FlushDefOrigins] at
// the end of the build.
//
// Anchors outside a definition build (paths, responses, info, parameters) fire inline as before;
// name reduction never renames those.
func (s *ScanCtx) RecordOrigin(pointer string, pos token.Position) {
	cb := s.opts.OnProvenance
	if cb == nil {
		return
	}
	if s.curDefKey != "" {
		s.defOrigins[s.curDefKey] = append(s.defOrigins[s.curDefKey], Provenance{Pointer: pointer, Pos: pos})
		return
	}
	if s.curDeferredKey != "" {
		s.deferredOrigins[s.curDeferredKey] = append(s.deferredOrigins[s.curDeferredKey], Provenance{Pointer: pointer, Pos: pos})
		return
	}
	cb(Provenance{Pointer: pointer, Pos: pos})
}

// BeginDefOrigins opens a buffering window for the definition keyed by defKey (its fully-qualified
// [EntityDecl.DefKey]).
//
// Until [EndDefOrigins], every [RecordOrigin] call is buffered under defKey instead of fired.
// No-op when no provenance sink is wired.
// Non-reentrant: each definition is built in its own pass, so windows never nest.
func (s *ScanCtx) BeginDefOrigins(defKey string) {
	if s.opts.OnProvenance == nil {
		return
	}
	if s.defOrigins == nil {
		s.defOrigins = make(map[string][]Provenance)
	}
	s.curDefKey = defKey
}

// EndDefOrigins closes the current definition buffering window.
func (s *ScanCtx) EndDefOrigins() {
	s.curDefKey = ""
}

// DropDefOrigins discards the buffered anchors for a definition that has been pruned, so its
// provenance is never emitted (no orphan pointer into a definition absent from the final document).
func (s *ScanCtx) DropDefOrigins(defKey string) {
	delete(s.defOrigins, defKey)
}

// FlushDefOrigins fires every buffered definition anchor, re-pointing each from its build-time
// fully-qualified base (#/definitions/<defKey>) to the definition's final name. finalName maps a
// definition key to the name the spec emits for it (identity when unchanged).
//
// Pointers are emitted in a deterministic (sorted) order.
// After the flush the buffer is cleared.
func (s *ScanCtx) FlushDefOrigins(finalName func(defKey string) string) {
	cb := s.opts.OnProvenance
	if cb == nil || len(s.defOrigins) == 0 {
		return
	}

	keys := make([]string, 0, len(s.defOrigins))
	for k := range s.defOrigins {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, defKey := range keys {
		oldBase := JSONPointer("definitions", defKey)
		newBase := JSONPointer("definitions", finalName(defKey))
		for _, rec := range s.defOrigins[defKey] {
			cb(Provenance{Pointer: newBase + strings.TrimPrefix(rec.Pointer, oldBase), Pos: rec.Pos})
		}
	}

	s.defOrigins = nil
	s.curDefKey = ""
}

// BeginDeferredOrigins opens a buffering window keyed by key for a top-level spec node that may be
// pruned after the build (a shared response).
//
// Until [EndDeferredOrigins], every [RecordOrigin] call is buffered under key instead of fired, so
// it can be dropped wholesale ([DropDeferredOrigins]) if the node is pruned, or flushed verbatim
// ([FlushDeferredOrigins]) if it survives.
// No-op when no provenance sink is wired.
//
// Non-reentrant.
func (s *ScanCtx) BeginDeferredOrigins(key string) {
	if s.opts.OnProvenance == nil {
		return
	}
	if s.deferredOrigins == nil {
		s.deferredOrigins = make(map[string][]Provenance)
	}
	s.curDeferredKey = key
}

// EndDeferredOrigins closes the current deferred buffering window.
func (s *ScanCtx) EndDeferredOrigins() {
	s.curDeferredKey = ""
}

// DropDeferredOrigins discards the buffered anchors for a deferred node that has been pruned, so
// its provenance is never emitted (no orphan pointer into a node absent from the final document).
func (s *ScanCtx) DropDeferredOrigins(key string) {
	delete(s.deferredOrigins, key)
}

// FlushDeferredOrigins fires every still-buffered deferred anchor verbatim (the nodes are never
// renamed) in a deterministic order, then clears the buffer.
func (s *ScanCtx) FlushDeferredOrigins() {
	cb := s.opts.OnProvenance
	if cb == nil || len(s.deferredOrigins) == 0 {
		return
	}

	keys := make([]string, 0, len(s.deferredOrigins))
	for k := range s.deferredOrigins {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, key := range keys {
		for _, rec := range s.deferredOrigins[key] {
			cb(rec)
		}
	}

	s.deferredOrigins = nil
	s.curDeferredKey = ""
}

// RecordParamOrigin stashes the source position of one parameter field, keyed by the operation id
// it applies to and the parameter name, for deferred anchor emission.
//
// No-op when no provenance sink is wired.
// See [ParamOrigin].
func (s *ScanCtx) RecordParamOrigin(opID, name string, pos token.Position) {
	if s.opts.OnProvenance == nil {
		return
	}
	if s.paramOrigins == nil {
		s.paramOrigins = make(map[string]map[string]token.Position)
	}
	byName := s.paramOrigins[opID]
	if byName == nil {
		byName = make(map[string]token.Position)
		s.paramOrigins[opID] = byName
	}
	byName[name] = pos
}

// ParamOrigin returns the captured source position for parameter name on operation opID, recorded
// earlier via [RecordParamOrigin].
//
// The spec builder's deferred pass uses it to emit /paths/{path}/{method}/parameters/{i} anchors
// once the final path binding and array index are known.
func (s *ScanCtx) ParamOrigin(opID, name string) (token.Position, bool) {
	byName := s.paramOrigins[opID]
	if byName == nil {
		return token.Position{}, false
	}
	pos, ok := byName[name]
	return pos, ok
}

func (s *ScanCtx) Meta() iter.Seq[*ast.CommentGroup] {
	if s.app == nil {
		return nil
	}

	return slices.Values(s.app.Meta)
}

func (s *ScanCtx) Operations() iter.Seq[parsers.ParsedPathContent] {
	if s.app == nil {
		return nil
	}

	return slices.Values(s.app.Operations)
}

func (s *ScanCtx) Routes() iter.Seq[parsers.ParsedPathContent] {
	if s.app == nil {
		return nil
	}

	return slices.Values(s.app.Routes)
}

func (s *ScanCtx) Responses() iter.Seq[*EntityDecl] {
	if s.app == nil {
		return nil
	}

	return slices.Values(s.app.Responses)
}

func (s *ScanCtx) Parameters() iter.Seq[*EntityDecl] {
	if s.app == nil {
		return nil
	}

	return slices.Values(s.app.Parameters)
}

// ParameterRefs iterates the standalone `swagger:parameters` reference markers discovered on func
// declarations (the references that wire shared parameters into operations / path-items as $refs).
//
// See [ParameterRef].
func (s *ScanCtx) ParameterRefs() iter.Seq[*ParameterRef] {
	if s.app == nil {
		return nil
	}

	return slices.Values(s.app.ParameterRefs)
}

func (s *ScanCtx) Models() iter.Seq2[*ast.Ident, *EntityDecl] {
	if s.app == nil {
		return nil
	}

	return maps.All(s.app.Models)
}

func (s *ScanCtx) NumExtraModels() int {
	if s.app == nil {
		return 0
	}

	return len(s.app.ExtraModels)
}

func (s *ScanCtx) ExtraModels() iter.Seq2[*ast.Ident, *EntityDecl] {
	if s.app == nil {
		return nil
	}

	return maps.All(s.app.ExtraModels)
}

func (s *ScanCtx) MoveExtraToModel(k *ast.Ident) {
	v, ok := s.app.ExtraModels[k]
	if !ok {
		return
	}

	s.app.Models[k] = v
	delete(s.app.ExtraModels, k)
}

func (s *ScanCtx) FindDecl(pkgPath, name string) (*EntityDecl, bool) {
	pkg, ok := s.app.AllPackages[pkgPath]
	if !ok {
		return nil, false
	}

	for _, file := range pkg.Syntax {
		for _, d := range file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, sp := range gd.Specs {
				ts, ok := sp.(*ast.TypeSpec)
				if !ok || ts.Name.Name != name {
					continue
				}

				def, ok := pkg.TypesInfo.Defs[ts.Name]
				if !ok {
					continue
				}

				nt, isNamed := def.Type().(*types.Named)
				at, isAliased := def.Type().(*types.Alias)
				if !isNamed && !isAliased {
					continue
				}

				comments := ts.Doc // type ( /* doc */ Foo struct{} )
				if comments == nil {
					comments = gd.Doc // /* doc */  type ( Foo struct{} )
				}

				return &EntityDecl{
					Comments: comments,
					Type:     nt,
					Alias:    at,
					Ident:    ts.Name,
					Spec:     ts,
					File:     file,
					Pkg:      pkg,
				}, true
			}
		}
	}

	return nil, false
}

// GetModel is a pure read: it returns the model decl for (pkgPath, name) without any side effect.
//
// # Details
//
// See [§model-lookup](./README.md#model-lookup) — the three-source lookup order (Models,
// ExtraModels, FindDecl), and how this differs from FindModel.
//
// Returns (nil, false) when no matching decl exists in any of the three sources.
// Callers that want the lookup hit registered as a discovered model must follow up with
// AddDiscoveredModel explicitly.
func (s *ScanCtx) GetModel(pkgPath, name string) (*EntityDecl, bool) {
	for _, cand := range s.app.Models {
		ct := cand.Obj()
		if ct.Name() == name && ct.Pkg().Path() == pkgPath {
			return cand, true
		}
	}

	for _, cand := range s.app.ExtraModels {
		ct := cand.Obj()
		if ct.Name() == name && ct.Pkg().Path() == pkgPath {
			return cand, true
		}
	}

	return s.FindDecl(pkgPath, name)
}

// FindModelsByLeaf returns every annotated swagger:model whose Go type name equals name, across all
// scanned packages, sorted by package path for determinism.
//
// It is the build-time analogue of the reduce stage's resolveDefinitionByLeaf: the type-name
// keyword sites use it to resolve a bare leaf to a model declared in another package (unique ->
// promote; several -> ambiguous).
//
// Only the annotated model set (fixed before building) is searched — not the discovery-grown
// ExtraModels — so the result is a pure function of the source, independent of build order (W6).
func (s *ScanCtx) FindModelsByLeaf(name string) []*EntityDecl {
	var out []*EntityDecl
	for _, cand := range s.app.Models {
		obj := cand.Obj()
		if obj == nil || obj.Name() != name {
			continue
		}
		out = append(out, cand)
	}
	slices.SortFunc(out, func(a, b *EntityDecl) int {
		return strings.Compare(a.Obj().Pkg().Path(), b.Obj().Pkg().Path())
	})
	return out
}

// AddDiscoveredModel registers decl in the ExtraModels index so the spec orchestrator emits a
// top-level definition for it.
//
// No-op when decl is already an annotated swagger:model (in Models); annotated decls are emitted
// unconditionally and re-registering them as "discovered" would create a Models↔ExtraModels
// bouncing loop in joinExtraModels.
// Nil and Ident-less decls are silently ignored.
//
// Use only at sites that explicitly intend the registration — pure-read lookups should use
// GetModel.
// See [§model-lookup](./README.md#model-lookup).
func (s *ScanCtx) AddDiscoveredModel(decl *EntityDecl) {
	if decl == nil || decl.Ident == nil {
		return
	}
	if _, alreadyModel := s.app.Models[decl.Ident]; alreadyModel {
		return
	}
	s.app.ExtraModels[decl.Ident] = decl
}

// FindModel returns the model decl for (pkgPath, name) and, when the hit comes from FindDecl
// fallback, registers it in ExtraModels as a side effect.
//
// Deprecated: prefer the explicit pair GetModel (pure read) and AddDiscoveredModel (explicit
// registration).
//
// The implicit registration side effect surprises readers and pulls stdlib types (notably
// time.Time, json.RawMessage) into the spec's top-level definitions when they should be inlined
// where referenced.
// See [§model-lookup](./README.md#model-lookup).
func (s *ScanCtx) FindModel(pkgPath, name string) (*EntityDecl, bool) {
	for _, cand := range s.app.Models {
		ct := cand.Obj()
		if ct.Name() == name && ct.Pkg().Path() == pkgPath {
			return cand, true
		}
	}

	if decl, found := s.FindDecl(pkgPath, name); found {
		s.app.ExtraModels[decl.Ident] = decl
		return decl, true
	}

	return nil, false
}

func (s *ScanCtx) DeclForType(t types.Type) (*EntityDecl, bool) {
	switch tpe := t.(type) {
	case *types.Pointer:
		return s.DeclForType(tpe.Elem())
	case *types.Named:
		return s.FindDecl(tpe.Obj().Pkg().Path(), tpe.Obj().Name())
	case *types.Alias:
		return s.FindDecl(tpe.Obj().Pkg().Path(), tpe.Obj().Name())
	default:
		s.EmitDiagnostic(grammar.Warnf(token.Position{}, grammar.CodeUnsupportedGoType,
			"unknown Go type %[1]T (%[1]v); cannot resolve its declaring source", t))

		return nil, false
	}
}

func (s *ScanCtx) PkgForType(t types.Type) (*packages.Package, bool) {
	switch tpe := t.(type) {
	// case *types.Basic: case *types.Struct: case *types.Pointer: case *types.Interface: case
	// *types.Array: case *types.Slice: case *types.Map:
	case *types.Named:
		v, ok := s.app.AllPackages[tpe.Obj().Pkg().Path()]
		return v, ok
	case *types.Alias:
		v, ok := s.app.AllPackages[tpe.Obj().Pkg().Path()]
		return v, ok
	default:
		s.EmitDiagnostic(grammar.Warnf(token.Position{}, grammar.CodeUnsupportedGoType,
			"unknown Go type %[1]T (%[1]v); cannot resolve its declaring package", t))
		return nil, false
	}
}

// FileForPos returns the *ast.File in package pkgPath whose source interval contains pos.
//
// Used when a struct's fields are defined in a different file than the decl that carries them —
// e.g. embedding a cross-package defined type (`type AnotherPackageAlias color.Color`), where the
// promoted fields live in the underlying type's source file, not in the embedding type's file.
// See go-swagger#2417.
//
// Matching is done via the shared FileSet: positions and ast.File starts resolve through the same
// *token.File, so the comparison is independent of go/ast's File range accessors.
func (s *ScanCtx) FileForPos(pkgPath string, pos token.Pos) (*ast.File, bool) {
	pkg, ok := s.app.AllPackages[pkgPath]
	if !ok || pkg.Fset == nil {
		return nil, false
	}

	target := pkg.Fset.File(pos)
	if target == nil {
		return nil, false
	}

	for _, file := range pkg.Syntax {
		if pkg.Fset.File(file.Pos()) == target {
			return file, true
		}
	}

	return nil, false
}

func (s *ScanCtx) FindComments(pkg *packages.Package, name string) (*ast.CommentGroup, bool) {
	for _, f := range pkg.Syntax {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, s := range gd.Specs {
				if ts, ok := s.(*ast.TypeSpec); ok {
					if ts.Name.Name == name {
						return gd.Doc, true
					}
				}
			}
		}
	}
	return nil, false
}

// FindEnumValues returns the enum values, per-value descriptions and per-value source positions for
// the constants typed enumName, plus ok.
//
// The positions are parallel to the values (one token.Pos per value, the const identifier) and feed
// the cross-ref /…/enum/{i} anchors; callers that don't need them ignore the third result.
func (s *ScanCtx) FindEnumValues(pkg *packages.Package, enumName string) (list []any, descList []string, posList []token.Pos, _ bool) {
	for _, f := range pkg.Syntax {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}

			if gd.Tok != token.CONST {
				continue
			}

			for _, spec := range gd.Specs {
				values, descriptions, positions := s.findEnumValue(spec, enumName)
				if len(values) == 0 {
					continue
				}

				list = append(list, values...)
				descList = append(descList, descriptions...)
				posList = append(posList, positions...)
			}
		}
	}

	return list, descList, posList, true
}

// findEnumValue extracts one (value, description) pair per (name, value) position in a const spec.
//
// For a multi-name spec like `const A, B T = "a", "b"` it emits two rows — A↔"a" and B↔"b"
// — each sharing the spec's doc comment.
// The Go compiler guarantees len(Names) == len(Values) when Values is non-empty, so out-of-parity
// specs are ignored defensively.
func (s *ScanCtx) findEnumValue(spec ast.Spec, enumName string) (values []any, descriptions []string, positions []token.Pos) {
	vs, ok := spec.(*ast.ValueSpec)
	if !ok {
		return nil, nil, nil
	}

	vsIdent, ok := vs.Type.(*ast.Ident)
	if !ok {
		return nil, nil, nil
	}

	if vsIdent.Name != enumName {
		return nil, nil, nil
	}

	if len(vs.Values) == 0 || len(vs.Values) != len(vs.Names) {
		return nil, nil, nil
	}

	docSuffix := buildEnumDocSuffix(vs.Doc, vs.Names)

	for i, nameIdent := range vs.Names {
		bl, ok := vs.Values[i].(*ast.BasicLit)
		if !ok {
			continue
		}

		literalValue := enumBasicLitValue(bl)

		var desc strings.Builder
		fmt.Fprintf(&desc, "%v %s", literalValue, nameIdent.Name)
		desc.WriteString(docSuffix)

		values = append(values, literalValue)
		descriptions = append(descriptions, desc.String())
		positions = append(positions, nameIdent.Pos())
	}

	return values, descriptions, positions
}

// buildEnumDocSuffix renders the shared doc comment as " <line1> <line2>..." (with a leading single
// space, keeping the per-line leading whitespace that survives TrimPrefix("//")), or the empty
// string if there is no doc.
//
// If the first non-empty doc line begins with one of the spec's names (idiomatic godoc convention:
// "Identifier does X"), that leading identifier is stripped so it does not duplicate the name
// already present in the row.
func buildEnumDocSuffix(doc *ast.CommentGroup, names []*ast.Ident) string {
	if doc == nil || len(doc.List) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(" ")

	stripped := false
	for i, line := range doc.List {
		if line.Text == "" {
			continue
		}

		text := strings.TrimPrefix(line.Text, "//")
		if !stripped {
			text = stripLeadingName(text, names)
			stripped = true
		}
		b.WriteString(text)

		if i < len(doc.List)-1 {
			b.WriteString(" ")
		}
	}

	return b.String()
}

// stripLeadingName removes a leading identifier from text when that identifier matches one of the
// provided names.
//
// Used to drop the godoc convention prefix ("Identifier does X") from an enum value's doc comment
// so the identifier is not printed twice in the rendered description row.
//
// On match, the original leading whitespace (from TrimPrefix("//")) is also dropped so the caller's
// single-space separator is not compounded into a double-space gap between the row's name and the
// remaining prose.
func stripLeadingName(text string, names []*ast.Ident) string {
	trimmed := strings.TrimLeft(text, " \t")

	word, rest, found := strings.Cut(trimmed, " ")
	if !found || word == "" {
		return text
	}

	for _, n := range names {
		if n.Name == word {
			return rest
		}
	}

	return text
}

func sliceToSet(names []string) map[string]bool {
	result := make(map[string]bool)
	for _, v := range names {
		result[v] = true
	}
	return result
}
