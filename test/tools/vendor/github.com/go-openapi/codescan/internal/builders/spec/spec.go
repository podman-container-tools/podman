// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"iter"
	"sort"
	"strconv"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/godoclink"
	"github.com/go-openapi/codescan/internal/builders/operations"
	"github.com/go-openapi/codescan/internal/builders/parameters"
	"github.com/go-openapi/codescan/internal/builders/responses"
	"github.com/go-openapi/codescan/internal/builders/routes"
	"github.com/go-openapi/codescan/internal/builders/schema"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// ErrInternalPanic is the base error for a builder panic recovered while processing a single
// declaration.
//
// Rather than surfacing a raw Go stack trace, the scan names the offending source declaration
// (file:line) and aborts with this error.
// See go-swagger/go-swagger#2886.
var ErrInternalPanic = errors.New("internal panic during build")

type Builder struct {
	scanModels  bool
	input       *oaispec.Swagger
	ctx         *scanner.ScanCtx
	discovered  []*scanner.EntityDecl
	definitions map[string]oaispec.Schema
	responses   map[string]oaispec.Response
	parameters  map[string]oaispec.Parameter
	operations  map[string]*oaispec.Operation
	// paramRefIntents are `swagger:parameters * opid …` references collected during buildParameters
	// and applied (as #/parameters/{name} $refs into operations) once the shared map is complete.
	//
	// See applyParameterRefs.
	paramRefIntents []paramRefIntent
	// pathItemInlines are `swagger:parameters /path` inline registrations collected during
	// buildParameters and applied to PathItem.Parameters once all paths exist.
	//
	// See applyPathItemParameters.
	pathItemInlines []pathItemInlineIntent
	// declPos records the source position of each discovered definition, keyed by its fully-qualified
	// DefKey, so the prune (scan.pruned-unused) and rename (scan.renamed-definition) Hints can be
	// located at the originating Go type even though the spec node may by then be gone or renamed.
	declPos map[string]token.Position
	// sharedParamPos / sharedRespPos record the source position of each scanned shared parameter /
	// response, keyed by its registered name, so the shared prune (scan.pruned-unused, C4) can locate
	// its Hint at the originating Go declaration.
	sharedParamPos map[string]token.Position
	sharedRespPos  map[string]token.Position
	// pinnedParams / pinnedResponses are the shared parameters / responses supplied via InputSpec
	// (present before the scan).
	//
	// They are pinned: never pruned, mirroring the definitions rule.
	pinnedParams    map[string]struct{}
	pinnedResponses map[string]struct{}
}

func NewBuilder(input *oaispec.Swagger, sc *scanner.ScanCtx, scanModels bool) *Builder {
	if input == nil {
		input = new(oaispec.Swagger)
		input.Swagger = "2.0"
	}

	if input.Paths == nil {
		input.Paths = new(oaispec.Paths)
	}
	if input.Definitions == nil {
		input.Definitions = make(map[string]oaispec.Schema)
	}
	if input.Responses == nil {
		input.Responses = make(map[string]oaispec.Response)
	}
	if input.Parameters == nil {
		input.Parameters = make(map[string]oaispec.Parameter)
	}
	if input.Extensions == nil {
		input.Extensions = make(oaispec.Extensions)
	}

	// Snapshot the InputSpec-supplied shared parameters / responses before the scan adds any: these
	// are pinned (never pruned by C4).
	pinnedParams := make(map[string]struct{}, len(input.Parameters))
	for name := range input.Parameters {
		pinnedParams[name] = struct{}{}
	}
	pinnedResponses := make(map[string]struct{}, len(input.Responses))
	for name := range input.Responses {
		pinnedResponses[name] = struct{}{}
	}

	return &Builder{
		ctx:             sc,
		input:           input,
		scanModels:      scanModels,
		operations:      collectOperationsFromInput(input),
		definitions:     input.Definitions,
		responses:       input.Responses,
		parameters:      input.Parameters,
		declPos:         make(map[string]token.Position),
		sharedParamPos:  make(map[string]token.Position),
		sharedRespPos:   make(map[string]token.Position),
		pinnedParams:    pinnedParams,
		pinnedResponses: pinnedResponses,
	}
}

func (s *Builder) Build() (*oaispec.Swagger, error) {
	// Resolve same-package duplicate swagger:model names up front so that every later DefKey() /
	// MakeRef() observes a consistent, conflict-free key for each declaration (D-4).
	// Must run before anything emits a ref.
	s.resolveSamePackageDuplicates()

	// this initial scan step is skipped if !scanModels.
	// Discovered dependencies should however be resolved.
	if err := s.buildModels(); err != nil {
		return nil, err
	}

	if err := s.buildParameters(); err != nil {
		return nil, err
	}

	// Wire shared-parameter references (#/parameters/{name} $refs) into operations now that the
	// top-level #/parameters map is complete, and before buildRoutes attaches operations to paths.
	s.applyParameterRefs()

	if err := s.buildResponses(); err != nil {
		return nil, err
	}

	// build definitions dictionary
	if err := s.buildDiscovered(); err != nil {
		return nil, err
	}

	if err := s.buildRoutes(); err != nil {
		return nil, err
	}

	if err := s.buildOperations(); err != nil {
		return nil, err
	}

	// Apply path-item parameters now that all paths exist (they are created by buildRoutes /
	// buildOperations).
	// `swagger:parameters /path` inlines fields into the path-item; `swagger:parameters /path name`
	// adds a #/parameters/{name} $ref.
	s.applyPathItemParameters()

	// Validate shared-namespace references across all paths now that the #/parameters and #/responses
	// maps are complete.
	// Catches dangling refs from swagger:operation wholesale-YAML bodies (which unmarshal verbatim)
	// and acts as a uniform safety net; a dangling ref is dropped, never emitted.
	s.validateSharedRefs()

	if err := s.buildMeta(); err != nil {
		return nil, err
	}

	// Cross-ref linkage: parameter anchors are resolved here, once paths, methods and final array
	// indices are all known (see emitParameterAnchors).
	// Param anchors live under /paths/.../parameters/{i}, independent of the definition-name reduction
	// below.
	s.emitParameterAnchors()

	// Prune discovered definitions not reachable from a root, when the caller opted in
	// (PruneUnusedModels).
	// Runs before name reduction so an unused model cannot force a spurious collision rename on a used
	// one.
	//
	s.pruneUnusedModels()

	// Fire the deferred shared-response provenance anchors buffered during buildResponses (only when
	// PruneUnusedModels + OnProvenance are both on); anchors of responses dropped by the prune above
	// were already discarded, so none dangle.
	// A no-op otherwise.
	s.ctx.FlushDeferredOrigins()

	// Final stage: shorten the fully-qualified, collision-proof definition keys produced during
	// discovery back to user-facing names and re-point every $ref.
	// Runs last because buildRoutes / buildOperations also emit definition refs.
	// §9/§12.
	renames := s.reduceDefinitionNames()

	// Definition provenance was buffered under each definition's fully-qualified key while building
	// (BeginDefOrigins); now that names are final, re-point every buffered anchor to its final name
	// and emit it.
	// Anchors for pruned definitions were already dropped, so none dangle.
	//
	s.ctx.FlushDefOrigins(func(defKey string) string {
		if final, ok := renames[defKey]; ok {
			return final
		}
		return defKey
	})

	// CleanGoDoc idiom recomposition: doc-links resolved at the consumption seam were encoded as
	// markers carrying a definition key; now that names are final (post-reduce), substitute each
	// marker for its schema's exposed name.
	// A no-op when CleanGoDoc is off (no markers were emitted).
	//
	// §3.
	if s.ctx.CleanGoDoc() {
		s.substituteGodocMarkers(renames)
	}

	if s.input.Swagger == "" {
		s.input.Swagger = "2.0"
	}

	return s.input, nil
}

// emitParameterAnchors fires the deferred /paths/{path}/{method}/parameters/{i} anchors.
//
// Parameters are built before their operation is bound to a path and before the parameter array
// order is final, so the parameters builder only captured (opid → name → position) via
// ScanCtx.RecordParamOrigin; here the absolute pointer is assembled from the finished paths tree.
// Finer parameter sub-nodes resolve to the parameter (or the operation) anchor.
func (s *Builder) emitParameterAnchors() {
	if !s.ctx.OriginEnabled() || s.input.Paths == nil {
		return
	}

	for path, item := range s.input.Paths.Paths {
		for method, op := range operationsByMethod(&item) {
			for i, prm := range op.Parameters {
				pos, ok := s.ctx.ParamOrigin(op.ID, prm.Name)
				if !ok {
					continue
				}
				s.ctx.RecordOrigin(
					scanner.JSONPointer("paths", path, method, "parameters", strconv.Itoa(i)),
					pos,
				)
			}
		}
	}
}

// operationsByMethod yields each populated (lowercase method → operation) slot of a path item, in
// a stable order. (Distinct from reduce.go's operationsOf, which returns the bare operation
// pointers for ref rewriting; here the method name is needed to build the
// /paths/{path}/{method}/... anchor pointer.)
func operationsByMethod(item *oaispec.PathItem) iter.Seq2[string, *oaispec.Operation] {
	return func(yield func(string, *oaispec.Operation) bool) {
		slots := []struct {
			method string
			op     *oaispec.Operation
		}{
			{"get", item.Get},
			{"put", item.Put},
			{"post", item.Post},
			{"delete", item.Delete},
			{"options", item.Options},
			{"head", item.Head},
			{"patch", item.Patch},
		}
		for _, slot := range slots {
			if slot.op == nil {
				continue
			}
			if !yield(slot.method, slot.op) {
				return
			}
		}
	}
}

// guard runs a per-declaration build step under panic recovery.
//
// On panic it emits a located scan.internal-panic diagnostic naming the offending source (pos +
// what), then returns an aborting error wrapping ErrInternalPanic — so a builder bug surfaces as
// a clean, located failure instead of a raw stack trace (#2886).
//
// A non-panicking run is transparent. pos/what identify the declaration being built; the
// spec.Builder build loops wrap each per-decl step.
// Panics outside any decl loop are caught by the Run backstop.
func (s *Builder) guard(pos token.Position, what string, run func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if cb := s.ctx.OnDiagnostic(); cb != nil {
				cb(grammar.Errorf(pos, grammar.CodeInternalPanic,
					"panic while building %s: %v", what, r))
			}
			err = fmt.Errorf("%w: %s: %s: %v", ErrInternalPanic, pos, what, r)
		}
	}()

	return run()
}

// declLabel names an EntityDecl for a diagnostic — the fully-qualified Go type when the package
// path is known, else the bare identifier.
func declLabel(d *scanner.EntityDecl) string {
	if d.Pkg != nil && d.Pkg.PkgPath != "" {
		return d.Pkg.PkgPath + "." + d.Ident.Name
	}

	return d.Ident.Name
}

func (s *Builder) buildDiscovered() error {
	// loop over discovered until all the items are in definitions
	keepGoing := len(s.discovered) > 0
	for keepGoing {
		var queue []*scanner.EntityDecl
		// Dedupe by name within this pass.
		// The same decl can appear multiple times in s.discovered (one entry per reference site that
		// called AppendPostDecl); without this, both copies get queued and Build runs twice, each
		// appending to the existing schema's AllOf and producing doubled entries.
		queued := make(map[string]struct{})
		for _, d := range s.discovered {
			// Dedup by the fully-qualified identity key (pkgpath/name), matching the definitions-map key
			// written by the schema builder.
			// A cycle / re-discovery of the SAME decl resolves to the same key and is skipped (reuse); two
			// distinct decls that merely share a short name now get distinct keys and both build.
			//
			// See name-identity design §9.1/§12.1.
			key := d.DefKey()
			if _, alreadyDone := s.definitions[key]; alreadyDone {
				continue
			}
			if _, dupInPass := queued[key]; dupInPass {
				continue
			}
			queued[key] = struct{}{}
			queue = append(queue, d)
		}
		s.discovered = nil
		for _, sd := range queue {
			if err := s.guard(s.ctx.PosOf(sd.Ident.Pos()), declLabel(sd), func() error {
				return s.buildDiscoveredSchema(sd)
			}); err != nil {
				return err
			}
		}
		keepGoing = len(s.discovered) > 0
	}

	return nil
}

func (s *Builder) buildDiscoveredSchema(decl *scanner.EntityDecl) error {
	sb := schema.NewBuilder(s.ctx, decl)
	sb.SetDiscovered(s.discovered)

	// Stash the definition's source position for the prune / rename Hints, which fire after the spec
	// node may have been dropped or renamed.
	s.declPos[decl.DefKey()] = s.ctx.PosOf(decl.Ident.Pos())

	// Cross-ref linkage: initiate the base pointer for this definition so the schema builder
	// path-joins its members (properties, …) under it, and anchor the definition node itself to its
	// type declaration.
	//
	// The base uses the fully-qualified DefKey, not the user-facing name: the definition is keyed by
	// DefKey throughout discovery and only renamed to its final name at the end of the build
	// (reduceDefinitionNames), after a possible prune.
	//
	// Anchors are buffered under DefKey (BeginDefOrigins) and re-pointed to the final name by
	// FlushDefOrigins once names are settled, so every emitted pointer resolves against the final
	// document.
	var defPtr string
	opts := []schema.Option{schema.WithDefinitions(s.definitions)}
	if s.ctx.OriginEnabled() {
		defPtr = scanner.JSONPointer("definitions", decl.DefKey())
		opts = append(opts, schema.WithPath(defPtr))
		s.ctx.BeginDefOrigins(decl.DefKey())
		defer s.ctx.EndDefOrigins()
	}

	if err := sb.Build(opts...); err != nil {
		return err
	}

	if defPtr != "" {
		s.ctx.RecordOrigin(defPtr, s.ctx.PosOf(decl.Ident.Pos()))
	}

	s.discovered = append(s.discovered, sb.PostDeclarations()...)

	return nil
}

// cleanGoDoc applies godoc-syntax filtering (Options.CleanGoDoc) to godoc- derived prose, returning
// text unchanged when the option is off.
//
// The spec builder owns its own ScanCtx (it does not embed common.Builder), so it carries a sibling
// of common.Builder.CleanGoDoc for the swagger:meta site.
func (s *Builder) cleanGoDoc(text string) string {
	if !s.ctx.CleanGoDoc() {
		return text
	}

	return godoclink.Clean(text, godoclink.Options{Mangler: s.ctx.Mangler()})
}

func (s *Builder) buildMeta() error {
	parser := grammar.NewParser(s.ctx.FileSet(),
		grammar.WithSingleLineCommentAsDescription(s.ctx.SingleLineCommentAsDescription()))
	for cg := range s.ctx.Meta() {
		block := parser.Parse(cg)
		if err := applyMetaBlock(s.input, block, s.cleanGoDoc); err != nil {
			return err
		}

		// Cross-ref linkage: anchor the info node to its swagger:meta block, then each meta keyword to
		// its own line (the top-level fields — host/basePath/consumes/… — have no ancestor anchor
		// otherwise, since /info is their sibling, not parent).
		if s.ctx.OriginEnabled() {
			s.ctx.RecordOrigin(scanner.JSONPointer("info"), s.ctx.PosOf(cg.Pos()))
			s.recordMetaOrigins(block)
		}
	}

	return nil
}

// recordMetaOrigins anchors each meta keyword in block to its source line.
//
// The keyword→pointer knowledge lives in the grammar ([grammar.PointerPath]); meta pointers are
// absolute (Info.* under /info, the rest at the document root), so there is no base to prepend.
func (s *Builder) recordMetaOrigins(block grammar.Block) {
	for p := range block.Properties() {
		if p.ItemsDepth != 0 {
			continue
		}
		if segs, ok := grammar.PointerPath(p.Keyword, grammar.CtxMeta); ok {
			s.ctx.RecordOrigin(scanner.JSONPointer(segs...), p.Pos)
		}
	}
}

func (s *Builder) buildOperations() error {
	for pp := range s.ctx.Operations() {
		if err := s.guard(s.ctx.PosOf(pp.Pos), "operation "+pp.Method+" "+pp.Path, func() error {
			ob := operations.NewBuilder(s.ctx, pp, s.operations)

			return ob.Build(s.input.Paths)
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *Builder) buildRoutes() error {
	// build paths dictionary
	for pp := range s.ctx.Routes() {
		if err := s.guard(s.ctx.PosOf(pp.Pos), "route "+pp.Method+" "+pp.Path, func() error {
			rb := routes.NewBuilder(
				s.ctx,
				pp,
				routes.Inputs{
					Responses:   s.responses,
					Operations:  s.operations,
					Definitions: s.definitions,
				},
			)

			return rb.Build(s.input.Paths)
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *Builder) buildResponses() error {
	// Order response declarations deterministically (package import path, then position) so the
	// keep-first conflict winner does not depend on package-load order.
	//
	// Distinct names are independent, so the order does not otherwise affect output.
	var decls []*scanner.EntityDecl
	for decl := range s.ctx.Responses() {
		decls = append(decls, decl)
	}
	sort.Slice(decls, func(i, j int) bool {
		pi, pj := declPkgPath(decls[i]), declPkgPath(decls[j])
		if pi != pj {
			return pi < pj
		}
		a, b := s.ctx.PosOf(decls[i].Ident.Pos()), s.ctx.PosOf(decls[j].Ident.Pos())
		if a.Filename != b.Filename {
			return a.Filename < b.Filename
		}
		return a.Offset < b.Offset
	})

	// scanned tracks the response short names already registered by a scanned struct in this pass.
	// A later scanned struct with the same name is a keep-first conflict; an InputSpec (overlay)
	// response of that name is NOT in the set, so a scanned struct still legitimately extends it.
	scanned := make(map[string]string)

	// Under PruneUnusedModels a shared response may be pruned after the build (C4).
	// Buffer its provenance anchors (top-level node + headers + inline body sub-anchors) so a pruned
	// response leaves none dangling; survivors are flushed verbatim by FlushDeferredOrigins after the
	// prune.
	deferOrigins := s.ctx.OriginEnabled() && s.ctx.PruneUnusedModels()

	// build responses dictionary
	for _, decl := range decls {
		if err := s.guard(s.ctx.PosOf(decl.Ident.Pos()), declLabel(decl), func() error {
			rb := responses.NewBuilder(s.ctx, decl)
			name := rb.ResponseName()

			if kept, dup := scanned[name]; dup {
				if onDiag := s.ctx.OnDiagnostic(); onDiag != nil {
					onDiag(grammar.Warnf(s.ctx.PosOf(decl.Ident.Pos()), grammar.CodeSharedResponseConflict,
						"shared response %q is already registered by %s; this declaration (%s) is dropped (keep-first)",
						name, kept, declLabel(decl)))
				}
				return nil
			}

			if deferOrigins && name != "" {
				s.ctx.BeginDeferredOrigins(responseOriginKey(name))
				defer s.ctx.EndDeferredOrigins()
			}

			if err := rb.Build(s.responses); err != nil {
				return err
			}
			s.discovered = append(s.discovered, rb.PostDeclarations()...)
			scanned[name] = declLabel(decl)
			s.sharedRespPos[name] = s.ctx.PosOf(decl.Ident.Pos())

			// Cross-ref linkage: anchor the top-level response node to its swagger:response declaration;
			// headers/body resolve to it.
			if s.ctx.OriginEnabled() && name != "" {
				s.ctx.RecordOrigin(scanner.JSONPointer("responses", name), s.ctx.PosOf(decl.Ident.Pos()))
			}

			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

// responseOriginKey is the deferred-provenance buffer key for a shared response (see
// ScanCtx.BeginDeferredOrigins).
//
// Keyed by the registered response name.
func responseOriginKey(name string) string { return "responses/" + name }

// sharedParamCandidate is one `swagger:parameters *` registration awaiting merge into the top-level
// #/parameters map.
//
// Candidates are collected from every declaration first, then resolved in a deterministic order so
// the keep-first conflict winner does not depend on package-load order.
type sharedParamCandidate struct {
	name  string
	param oaispec.Parameter
	pkg   string
	pos   token.Position
	label string
}

func (s *Builder) buildParameters() error {
	var sharedCandidates []sharedParamCandidate

	// build parameters dictionary
	for decl := range s.ctx.Parameters() {
		if err := s.guard(s.ctx.PosOf(decl.Ident.Pos()), declLabel(decl), func() error {
			pb := parameters.NewBuilder(s.ctx, decl)
			if err := pb.Build(s.operations); err != nil {
				return err
			}
			s.discovered = append(s.discovered, pb.PostDeclarations()...)

			pkg := declPkgPath(decl)
			pos := s.ctx.PosOf(decl.Ident.Pos())
			for name, prm := range pb.SharedParameters() {
				sharedCandidates = append(sharedCandidates, sharedParamCandidate{
					name: name, param: prm, pkg: pkg, pos: pos, label: declLabel(decl),
				})
			}

			// `swagger:parameters * opid …` also references the struct's shared parameters into the listed
			// operations.
			// Collect the intents now; they are applied once the shared map is complete.
			for _, op := range pb.SharedRefOperations() {
				for name := range pb.SharedParameters() {
					s.paramRefIntents = append(s.paramRefIntents, paramRefIntent{
						op: op, name: name, pos: pos, label: declLabel(decl),
					})
				}
			}

			// `swagger:parameters /path` inlines the struct's fields into a path-item.
			// Collect now; applied after paths are built.
			for path, params := range pb.PathItemParameters() {
				s.pathItemInlines = append(s.pathItemInlines, pathItemInlineIntent{
					path: path, params: params, pkg: pkg, pos: pos, label: declLabel(decl),
				})
			}

			return nil
		}); err != nil {
			return err
		}
	}

	s.registerSharedParameters(sharedCandidates)

	return nil
}

// registerSharedParameters merges the collected `swagger:parameters *` registrations into the
// spec's top-level #/parameters map.
//
// Shared parameters are referenced only by short name and are therefore never renamed (unlike
// definitions): on a duplicate short name the first registration is kept and the later one dropped
// with a CodeSharedParameterConflict warning (keep-first).
// InputSpec-supplied entries seed the map before the scan, so they win any collision.
//
// Candidates are resolved in a deterministic order — package import path, then source position,
// then parameter name — so the kept/dropped choice and the emitted diagnostic are stable
// regardless of package-load order.
func (s *Builder) registerSharedParameters(candidates []sharedParamCandidate) {
	if len(candidates) == 0 {
		return
	}

	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.pkg != b.pkg {
			return a.pkg < b.pkg
		}
		if a.pos.Filename != b.pos.Filename {
			return a.pos.Filename < b.pos.Filename
		}
		if a.pos.Offset != b.pos.Offset {
			return a.pos.Offset < b.pos.Offset
		}
		return a.name < b.name
	})

	owners := make(map[string]string, len(candidates))
	for _, c := range candidates {
		if _, exists := s.parameters[c.name]; exists {
			if onDiag := s.ctx.OnDiagnostic(); onDiag != nil {
				kept := owners[c.name]
				if kept == "" {
					kept = "an overlay (InputSpec) definition"
				}
				onDiag(grammar.Warnf(c.pos, grammar.CodeSharedParameterConflict,
					"shared parameter %q is already registered by %s; this declaration (%s) is dropped (keep-first)",
					c.name, kept, c.label))
			}
			continue
		}
		s.parameters[c.name] = c.param
		owners[c.name] = c.label
		s.sharedParamPos[c.name] = c.pos
	}
}

// declPkgPath returns the import path of the declaration's package, or "" when unknown.
func declPkgPath(d *scanner.EntityDecl) string {
	if d.Pkg != nil {
		return d.Pkg.PkgPath
	}
	return ""
}

// paramRefIntent is one shared-parameter reference to apply as a #/parameters/{name} $ref on an
// operation.
type paramRefIntent struct {
	op    string
	name  string
	pos   token.Position
	label string
}

// applyParameterRefs wires shared-parameter references into operations as #/parameters/{name}
// $refs.
//
// Two sources feed it: `swagger:parameters * opid …` markers (collected in paramRefIntents during
// buildParameters) and standalone `swagger:parameters opid name …` reference markers on func
// declarations (ScanCtx.ParameterRefs, discovered by the scanner).
//
// A reference to an unregistered shared parameter is dropped with a scan.dangling-parameter-ref
// warning rather than emitting a dangling $ref.
//
// Runs after the top-level #/parameters map is complete and before buildRoutes attaches operations
// to paths.
// Intents are applied in a deterministic order (operation id, then parameter name).
func (s *Builder) applyParameterRefs() {
	intents := append([]paramRefIntent(nil), s.paramRefIntents...)
	intents = append(intents, s.collectStandaloneParameterRefs()...)
	if len(intents) == 0 {
		return
	}

	sort.Slice(intents, func(i, j int) bool {
		if intents[i].op != intents[j].op {
			return intents[i].op < intents[j].op
		}
		return intents[i].name < intents[j].name
	})

	for _, in := range intents {
		s.addParameterRef(in)
	}
}

// collectStandaloneParameterRefs turns each standalone `swagger:parameters` reference marker on a
// func (ScanCtx.ParameterRefs) into per-name ref intents.
//
// The marker's first token is the target operation id and the remaining tokens are shared-parameter
// names.
// Path-item references (`/path` target) are handled in a later phase.
// Duplicate names dropped by the grammar raise a duplicate-ref warning (C2).
func (s *Builder) collectStandaloneParameterRefs() []paramRefIntent {
	var out []paramRefIntent
	for ref := range s.ctx.ParameterRefs() {
		pb := s.parametersBlockOf(ref.Comments)
		if pb == nil || pb.Target != grammar.ParamTargetOperations {
			continue
		}
		label := "a swagger:parameters reference"
		if ref.Pkg != nil {
			label = "a swagger:parameters reference in " + ref.Pkg.PkgPath
		}
		for _, dup := range pb.Dups {
			if onDiag := s.ctx.OnDiagnostic(); onDiag != nil {
				onDiag(grammar.Warnf(pb.Pos(), grammar.CodeDuplicateRef,
					"swagger:parameters: duplicate reference %q dropped", dup))
			}
		}
		// Args[0] is the target operation id; Args[1:] are shared names.
		//
		// A reference needs at least a target plus one name.
		const minReferenceTokens = 2
		if len(pb.Args) < minReferenceTokens {
			continue
		}
		op := pb.Args[0]
		for _, name := range pb.Args[1:] {
			out = append(out, paramRefIntent{op: op, name: name, pos: pb.Pos(), label: label})
		}
	}
	return out
}

// parametersBlockOf returns the first ParametersBlock parsed from cg, or nil.
//
// References are parsed straight from the grammar (the targeting parse lives there); the spec
// builder owns no parameters.Builder for a func-hosted marker.
func (s *Builder) parametersBlockOf(cg *ast.CommentGroup) *grammar.ParametersBlock {
	for _, b := range grammar.NewParser(s.ctx.FileSet()).ParseAll(cg) {
		if pb, ok := b.(*grammar.ParametersBlock); ok {
			return pb
		}
	}
	return nil
}

// addParameterRef appends a #/parameters/{name} $ref to the target operation (creating the
// operation entry when absent), or drops the reference with a scan.dangling-parameter-ref warning
// when no such shared parameter is registered.
//
// The same $ref is never added twice to one operation.
func (s *Builder) addParameterRef(in paramRefIntent) {
	if _, ok := s.parameters[in.name]; !ok {
		if onDiag := s.ctx.OnDiagnostic(); onDiag != nil {
			onDiag(grammar.Warnf(in.pos, grammar.CodeDanglingParameterRef,
				"%s references shared parameter %q, but no #/parameters/%s is registered; dropped",
				in.label, in.name, in.name))
		}
		return
	}

	op, ok := s.operations[in.op]
	if !ok {
		op = new(oaispec.Operation)
		op.ID = in.op
		s.operations[in.op] = op
	}

	ref, err := oaispec.NewRef("#/parameters/" + in.name)
	if err != nil {
		return
	}
	for _, p := range op.Parameters {
		if p.Ref.String() == ref.String() {
			return
		}
	}
	op.Parameters = append(op.Parameters, oaispec.Parameter{Refable: oaispec.Refable{Ref: ref}})
}

// pathItemInlineIntent is one `swagger:parameters /path` registration: the struct's fields, inlined
// into the named path-item.
type pathItemInlineIntent struct {
	path   string
	params []oaispec.Parameter
	pkg    string
	pos    token.Position
	label  string
}

// pathItemRefIntent is one `swagger:parameters /path name` reference: a #/parameters/{name} $ref
// added to the named path-item.
type pathItemRefIntent struct {
	path  string
	name  string
	pos   token.Position
	label string
}

// applyPathItemParameters applies path-item parameters once all paths exist.
//
// `swagger:parameters /path` inlines a struct's fields into the path-item; `swagger:parameters
// /path name` adds a #/parameters/{name} $ref.
// Per-path the inline parameters come first (ordered by package path then position), then the $refs
// (ordered by name).
//
// The path must already exist as a route/operation target (OAS2 has no path hierarchy — the match
// is exact); a reference to an unknown path or unregistered shared parameter is dropped with a
// warning.
//
// Operation-level parameters are left untouched: path-item and operation parameters co-exist (the
// operation one wins at resolution per OAS2).
func (s *Builder) applyPathItemParameters() {
	if s.input.Paths == nil {
		return
	}

	// Accumulate the ordered parameter list to append per path.
	perPath := map[string][]oaispec.Parameter{}

	inlines := append([]pathItemInlineIntent(nil), s.pathItemInlines...)
	sort.Slice(inlines, func(i, j int) bool {
		a, b := inlines[i], inlines[j]
		if a.path != b.path {
			return a.path < b.path
		}
		if a.pkg != b.pkg {
			return a.pkg < b.pkg
		}
		if a.pos.Filename != b.pos.Filename {
			return a.pos.Filename < b.pos.Filename
		}
		return a.pos.Offset < b.pos.Offset
	})
	for _, in := range inlines {
		perPath[in.path] = append(perPath[in.path], in.params...)
	}

	refs := s.collectPathItemRefs()
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].path != refs[j].path {
			return refs[i].path < refs[j].path
		}
		return refs[i].name < refs[j].name
	})
	onDiag := s.ctx.OnDiagnostic()
	for _, r := range refs {
		if _, ok := s.parameters[r.name]; !ok {
			if onDiag != nil {
				onDiag(grammar.Warnf(r.pos, grammar.CodeDanglingParameterRef,
					"%s references shared parameter %q, but no #/parameters/%s is registered; dropped",
					r.label, r.name, r.name))
			}
			continue
		}
		ref, err := oaispec.NewRef("#/parameters/" + r.name)
		if err != nil {
			continue
		}
		if !containsParamRef(perPath[r.path], ref) {
			perPath[r.path] = append(perPath[r.path], oaispec.Parameter{Refable: oaispec.Refable{Ref: ref}})
		}
	}

	for path, params := range perPath {
		pi, ok := s.input.Paths.Paths[path]
		if !ok {
			if onDiag != nil {
				onDiag(grammar.Warnf(token.Position{}, grammar.CodeInvalidAnnotation,
					"swagger:parameters %s names a path with no operations; path-item parameters dropped", path))
			}
			continue
		}
		pi.Parameters = append(pi.Parameters, params...)
		s.input.Paths.Paths[path] = pi
	}
}

// collectPathItemRefs turns each standalone `swagger:parameters /path name …` reference marker
// (ScanCtx.ParameterRefs with a path target) into per-name path-item ref intents.
//
// Duplicate names dropped by the grammar raise a duplicate-ref warning (C2).
func (s *Builder) collectPathItemRefs() []pathItemRefIntent {
	var out []pathItemRefIntent
	for ref := range s.ctx.ParameterRefs() {
		pb := s.parametersBlockOf(ref.Comments)
		if pb == nil || pb.Target != grammar.ParamTargetPath {
			continue
		}
		label := "a swagger:parameters path reference"
		if ref.Pkg != nil {
			label = "a swagger:parameters path reference in " + ref.Pkg.PkgPath
		}
		for _, dup := range pb.Dups {
			if onDiag := s.ctx.OnDiagnostic(); onDiag != nil {
				onDiag(grammar.Warnf(pb.Pos(), grammar.CodeDuplicateRef,
					"swagger:parameters: duplicate reference %q dropped", dup))
			}
		}
		// For a path target the path is pb.Path; every Arg is a shared name.
		for _, name := range pb.Args {
			out = append(out, pathItemRefIntent{path: pb.Path, name: name, pos: pb.Pos(), label: label})
		}
	}
	return out
}

// containsParamRef reports whether params already holds a $ref to the same target.
func containsParamRef(params []oaispec.Parameter, ref oaispec.Ref) bool {
	for _, p := range params {
		if p.Ref.String() == ref.String() {
			return true
		}
	}
	return false
}

// validateSharedRefs drops shared-namespace references that resolve to nothing — a
// #/parameters/{name} or #/responses/{name} $ref whose target is not registered.
//
// The main source is a swagger:operation wholesale-YAML body, which unmarshals its
// parameters/responses (and their $refs) verbatim; this pass validates them against the completed
// #/parameters and #/responses maps and drops danglers with a warning rather than leaving an
// invalid reference in the output.
//
// Valid refs (and non-shared refs such as #/definitions/…) are left untouched.
func (s *Builder) validateSharedRefs() {
	if s.input.Paths == nil {
		return
	}
	onDiag := s.ctx.OnDiagnostic()

	for path, pi := range s.input.Paths.Paths {
		pi.Parameters = s.keepResolvableParamRefs(path, pi.Parameters, onDiag)
		for _, op := range pathItemOperations(&pi) {
			if op == nil {
				continue
			}
			op.Parameters = s.keepResolvableParamRefs(path, op.Parameters, onDiag)
			s.dropDanglingResponseRefs(path, op.Responses, onDiag)
		}
		s.input.Paths.Paths[path] = pi
	}
}

// pathItemOperations returns the operation pointers of a path-item (nil where a method is absent).
func pathItemOperations(pi *oaispec.PathItem) []*oaispec.Operation {
	return []*oaispec.Operation{pi.Get, pi.Put, pi.Post, pi.Delete, pi.Options, pi.Head, pi.Patch}
}

// keepResolvableParamRefs returns params with any dangling #/parameters/{name} $ref dropped (and
// warned).
//
// Non-ref and resolvable-ref parameters are preserved in order.
func (s *Builder) keepResolvableParamRefs(path string, params []oaispec.Parameter, onDiag func(grammar.Diagnostic)) []oaispec.Parameter {
	kept := params[:0:0]
	for _, p := range params {
		if name, ok := sharedRefName(p.Ref, "#/parameters/"); ok {
			if _, exists := s.parameters[name]; !exists {
				if onDiag != nil {
					onDiag(grammar.Warnf(token.Position{}, grammar.CodeDanglingParameterRef,
						"path %s references #/parameters/%s, which is not registered; dropped", path, name))
				}
				continue
			}
		}
		kept = append(kept, p)
	}
	return kept
}

// dropDanglingResponseRefs removes any dangling #/responses/{name} $ref from an operation's
// responses (default + per-status), warning on each.
func (s *Builder) dropDanglingResponseRefs(path string, resp *oaispec.Responses, onDiag func(grammar.Diagnostic)) {
	if resp == nil {
		return
	}
	if resp.Default != nil {
		if name, dangling := s.danglingResponseRef(*resp.Default); dangling {
			if onDiag != nil {
				onDiag(grammar.Warnf(token.Position{}, grammar.CodeDanglingResponseRef,
					"path %s default response references #/responses/%s, which is not registered; dropped", path, name))
			}
			resp.Default = nil
		}
	}
	for code, r := range resp.StatusCodeResponses {
		if name, dangling := s.danglingResponseRef(r); dangling {
			if onDiag != nil {
				onDiag(grammar.Warnf(token.Position{}, grammar.CodeDanglingResponseRef,
					"path %s response %d references #/responses/%s, which is not registered; dropped", path, code, name))
			}
			delete(resp.StatusCodeResponses, code)
		}
	}
}

// danglingResponseRef reports whether r is a #/responses/{name} $ref whose target is not
// registered, returning the unresolved name.
func (s *Builder) danglingResponseRef(r oaispec.Response) (string, bool) {
	name, ok := sharedRefName(r.Ref, "#/responses/")
	if !ok {
		return "", false
	}
	if _, exists := s.responses[name]; exists {
		return "", false
	}
	return name, true
}

// sharedRefName returns the {name} suffix of a $ref of the form "{prefix}{name}" (e.g. prefix
// "#/parameters/"), and whether ref matched.
func sharedRefName(ref oaispec.Ref, prefix string) (string, bool) {
	if after, ok := strings.CutPrefix(ref.String(), prefix); ok && after != "" {
		return after, true
	}
	return "", false
}

// resolveSamePackageDuplicates detects two distinct annotated models in the SAME package that claim
// the same definition name — necessarily via a `swagger:model <name>` override, since Go type
// names are unique per package.
//
// The first (in a deterministic order) keeps the name; later ones have their override suppressed
// (reverting to the Go type name) and get a diagnostic.
// This is the build-side half of D-4; cross-package same-name collisions are handled later by the
// reduce stage.
//
// §9.1/§12.1.
func (s *Builder) resolveSamePackageDuplicates() {
	models := make([]*scanner.EntityDecl, 0)
	for _, d := range s.ctx.Models() {
		models = append(models, d)
	}
	// Deterministic order so "first wins" is stable across runs: by key, then by Go name within a
	// colliding group.
	sort.Slice(models, func(i, j int) bool {
		ki, kj := models[i].DefKey(), models[j].DefKey()
		if ki != kj {
			return ki < kj
		}
		return models[i].Ident.Name < models[j].Ident.Name
	})

	seen := make(map[string]*scanner.EntityDecl, len(models))
	for _, d := range models {
		key := d.DefKey()
		if first, dup := seen[key]; dup && first.Ident != d.Ident {
			d.SuppressModelOverride()
			if onDiag := s.ctx.OnDiagnostic(); onDiag != nil {
				_, goName := d.Names()
				onDiag(grammar.Warnf(
					s.ctx.PosOf(d.Spec.Pos()),
					grammar.CodeDuplicateModelName,
					"duplicate swagger:model name %q in package %q (already used by type %q); using Go name %q instead",
					leafName(key), d.Obj().Pkg().Path(), first.Ident.Name, goName,
				))
			}
			continue
		}
		seen[key] = d
	}
}

func (s *Builder) buildModels() error {
	// build models dictionary
	if !s.scanModels {
		return nil
	}

	for _, decl := range s.ctx.Models() {
		if err := s.guard(s.ctx.PosOf(decl.Ident.Pos()), declLabel(decl), func() error {
			return s.buildDiscoveredSchema(decl)
		}); err != nil {
			return err
		}
	}

	return s.joinExtraModels()
}

func (s *Builder) joinExtraModels() error {
	l := s.ctx.NumExtraModels()
	if l == 0 {
		return nil
	}

	tmp := make(map[*ast.Ident]*scanner.EntityDecl, l)
	for k, v := range s.ctx.ExtraModels() {
		tmp[k] = v
		s.ctx.MoveExtraToModel(k)
	}

	// process extra models and see if there is any reference to a new extra one
	for _, decl := range tmp {
		if err := s.buildDiscoveredSchema(decl); err != nil {
			return err
		}
	}

	if s.ctx.NumExtraModels() > 0 {
		return s.joinExtraModels()
	}

	return nil
}

func collectOperationsFromInput(input *oaispec.Swagger) map[string]*oaispec.Operation {
	operations := make(map[string]*oaispec.Operation)
	if input == nil || input.Paths == nil {
		return operations
	}

	for _, pth := range input.Paths.Paths {
		if pth.Get != nil {
			operations[pth.Get.ID] = pth.Get
		}
		if pth.Post != nil {
			operations[pth.Post.ID] = pth.Post
		}
		if pth.Put != nil {
			operations[pth.Put.ID] = pth.Put
		}
		if pth.Patch != nil {
			operations[pth.Patch.ID] = pth.Patch
		}
		if pth.Delete != nil {
			operations[pth.Delete.ID] = pth.Delete
		}
		if pth.Head != nil {
			operations[pth.Head.ID] = pth.Head
		}
		if pth.Options != nil {
			operations[pth.Options.ID] = pth.Options
		}
	}

	return operations
}
