// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"go/token"
	"sort"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
	oaispec "github.com/go-openapi/spec"
)

const defRefPrefix = "#/definitions/"

// pruneUnusedModels drops every discovered definition not transitively referenced from a
// reachability root, when the caller set PruneUnusedModels.
//
// It is the middle ground between the default route-reachable-only mode and the emit-everything
// ScanModels mode: run ScanModels discovery, then prune the unreachable tail
// (go-swagger/go-swagger#2639).
//
// It runs BEFORE reduceDefinitionNames, in the fully-qualified definition-key namespace, so a
// pruned (unused) model can no longer force a spurious cross-package name collision on a model that
// IS used — the survivor keeps its clean short name.
//
// Buffered provenance for each pruned definition is dropped so no anchor dangles, and each prune
// raises a scan.pruned-unused Hint.
//
// Reachability roots are the paths (operation parameters + responses), the shared responses and
// parameters, and every definition supplied via InputSpec (pinned: never pruned, and seeded as
// roots so their $ref targets survive).
//
// Definitions themselves are not roots — a model referenced only by another unreferenced model is
// pruned too.
func (s *Builder) pruneUnusedModels() {
	if !s.ctx.PruneUnusedModels() {
		return
	}

	onDiag := s.ctx.OnDiagnostic()

	if !s.scanModels {
		// Without ScanModels the emitted set is already reference-reachable, so there is nothing to
		// prune.
		// Surface one Hint so a caller who set the flag alone learns it did nothing.
		if onDiag != nil {
			onDiag(grammar.Hintf(token.Position{}, grammar.CodePrunedUnused,
				"PruneUnusedModels has no effect without ScanModels: "+
					"the emitted definition set is already reference-reachable"))
		}
		return
	}

	// C4: prune unreferenced shared parameters / responses FIRST, so a definition kept alive only by a
	// now-pruned shared object becomes prunable in the definition pass below.
	// §6b.
	s.pruneUnusedSharedObjects(onDiag)

	if len(s.input.Definitions) == 0 {
		return
	}

	reachable := make(map[string]struct{}, len(s.input.Definitions))
	var queue []string
	mark := func(key string) {
		if key == "" {
			return
		}
		if _, seen := reachable[key]; seen {
			return
		}
		if _, ok := s.input.Definitions[key]; !ok {
			return // a $ref to something that is not a definition we hold
		}
		reachable[key] = struct{}{}
		queue = append(queue, key)
	}

	// Seed roots: refs from paths / shared responses / shared parameters.
	for _, key := range s.rootRefs() {
		mark(key)
	}
	// Pin overlay-supplied definitions (those built from no source declaration) as roots: the caller
	// put them there deliberately.
	for key := range s.input.Definitions {
		if _, built := s.declPos[key]; !built {
			mark(key)
		}
	}

	// Transitive closure over the definition graph.
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		sch := s.input.Definitions[cur]
		collectDefRefs(&sch, mark)
	}

	// Prune the unreachable, deterministically.
	pruned := make([]string, 0, len(s.input.Definitions))
	for key := range s.input.Definitions {
		if _, keep := reachable[key]; !keep {
			pruned = append(pruned, key)
		}
	}
	sort.Strings(pruned)

	for _, key := range pruned {
		delete(s.input.Definitions, key)
		s.ctx.DropDefOrigins(key) // no orphan provenance for a pruned node
		if onDiag != nil {
			onDiag(grammar.Hintf(s.declPos[key], grammar.CodePrunedUnused,
				"definition %q pruned: not referenced from any path, response, "+
					"parameter or overlay definition", key))
		}
	}
}

// rootRefs returns every definition key referenced from a reachability root: path operations (body
// parameters + response schemas), shared responses and shared parameters.
//
// Definitions are deliberately excluded — they stay only if reached transitively from here.
func (s *Builder) rootRefs() []string {
	var out []string
	collect := func(sch *oaispec.Schema) {
		collectDefRefs(sch, func(key string) { out = append(out, key) })
	}

	for _, p := range s.input.Parameters {
		collect(p.Schema)
	}
	for _, r := range s.input.Responses {
		collect(r.Schema)
	}
	if s.input.Paths != nil {
		for _, pi := range s.input.Paths.Paths {
			for _, op := range operationsOf(pi) {
				if op == nil {
					continue
				}
				for i := range op.Parameters {
					collect(op.Parameters[i].Schema)
				}
				if op.Responses == nil {
					continue
				}
				if op.Responses.Default != nil {
					collect(op.Responses.Default.Schema)
				}
				for _, resp := range op.Responses.StatusCodeResponses {
					collect(resp.Schema)
				}
			}
		}
	}

	return out
}

// pruneUnusedSharedObjects drops shared parameters (#/parameters/*) and shared responses
// (#/responses/*) that no operation or path-item references (C4).
//
// It runs BEFORE the definition reachability walk so the surviving shared objects (which are
// reachability roots) seed that walk, and a definition kept alive only by a now-pruned shared
// object becomes prunable in turn.
//
// InputSpec-supplied shared objects are pinned (never pruned), mirroring the definitions rule.
// Each drop raises a located scan.pruned-unused Hint.
func (s *Builder) pruneUnusedSharedObjects(onDiag func(grammar.Diagnostic)) {
	refParams, refResponses := s.collectSharedRefs()

	// Shared parameters.
	prunedParams := make([]string, 0, len(s.parameters))
	for name := range s.parameters {
		if _, pinned := s.pinnedParams[name]; pinned {
			continue
		}
		if _, used := refParams[name]; used {
			continue
		}
		prunedParams = append(prunedParams, name)
	}
	sort.Strings(prunedParams)
	for _, name := range prunedParams {
		delete(s.parameters, name)
		if onDiag != nil {
			onDiag(grammar.Hintf(s.sharedParamPos[name], grammar.CodePrunedUnused,
				"shared parameter %q pruned: not referenced by any operation or path-item", name))
		}
	}

	// Shared responses.
	prunedResponses := make([]string, 0, len(s.responses))
	for name := range s.responses {
		if _, pinned := s.pinnedResponses[name]; pinned {
			continue
		}
		if _, used := refResponses[name]; used {
			continue
		}
		prunedResponses = append(prunedResponses, name)
	}
	sort.Strings(prunedResponses)
	for _, name := range prunedResponses {
		delete(s.responses, name)
		s.ctx.DropDeferredOrigins(responseOriginKey(name)) // no orphan provenance
		if onDiag != nil {
			onDiag(grammar.Hintf(s.sharedRespPos[name], grammar.CodePrunedUnused,
				"shared response %q pruned: not referenced by any operation or path-item", name))
		}
	}
}

// collectSharedRefs scans every path-item and operation for references into the shared namespaces,
// returning the set of shared-parameter names reached by a #/parameters/{name} $ref and the set of
// shared-response names reached by a #/responses/{name} $ref.
//
// It is the read-only "is referenced" mirror of the ref-application passes (applyParameterRefs /
// applyPathItemParameters / route response binding) and the validateSharedRefs safety net.
func (s *Builder) collectSharedRefs() (params, responses map[string]struct{}) {
	params = map[string]struct{}{}
	responses = map[string]struct{}{}
	if s.input.Paths == nil {
		return params, responses
	}

	markParam := func(ref oaispec.Ref) {
		if name, ok := sharedRefName(ref, "#/parameters/"); ok {
			params[name] = struct{}{}
		}
	}
	markResp := func(ref oaispec.Ref) {
		if name, ok := sharedRefName(ref, "#/responses/"); ok {
			responses[name] = struct{}{}
		}
	}

	for _, pi := range s.input.Paths.Paths {
		for i := range pi.Parameters {
			markParam(pi.Parameters[i].Ref)
		}
		for _, op := range operationsOf(pi) {
			if op == nil {
				continue
			}
			for i := range op.Parameters {
				markParam(op.Parameters[i].Ref)
			}
			if op.Responses == nil {
				continue
			}
			if op.Responses.Default != nil {
				markResp(op.Responses.Default.Ref)
			}
			for _, r := range op.Responses.StatusCodeResponses {
				markResp(r.Ref)
			}
		}
	}

	return params, responses
}

// collectDefRefs walks sch and every sub-schema, invoking mark for each "#/definitions/<key>"
// reference it carries.
//
// It is the read-only mirror of rewriteSchemaRefs and must cover the same container set; a missed
// container would make a referenced model look unreachable and wrongly prune it.
func collectDefRefs(sch *oaispec.Schema, mark func(key string)) {
	if sch == nil {
		return
	}
	if r := sch.Ref.String(); strings.HasPrefix(r, defRefPrefix) {
		mark(r[len(defRefPrefix):])
	}
	for k := range sch.Properties {
		v := sch.Properties[k]
		collectDefRefs(&v, mark)
	}
	for k := range sch.PatternProperties {
		v := sch.PatternProperties[k]
		collectDefRefs(&v, mark)
	}
	for i := range sch.AllOf {
		collectDefRefs(&sch.AllOf[i], mark)
	}
	for i := range sch.AnyOf {
		collectDefRefs(&sch.AnyOf[i], mark)
	}
	for i := range sch.OneOf {
		collectDefRefs(&sch.OneOf[i], mark)
	}
	collectDefRefs(sch.Not, mark)
	if sch.Items != nil {
		collectDefRefs(sch.Items.Schema, mark)
		for i := range sch.Items.Schemas {
			collectDefRefs(&sch.Items.Schemas[i], mark)
		}
	}
	if sch.AdditionalProperties != nil {
		collectDefRefs(sch.AdditionalProperties.Schema, mark)
	}
	if sch.AdditionalItems != nil {
		collectDefRefs(sch.AdditionalItems.Schema, mark)
	}
	for k := range sch.Definitions {
		v := sch.Definitions[k]
		collectDefRefs(&v, mark)
	}
}
