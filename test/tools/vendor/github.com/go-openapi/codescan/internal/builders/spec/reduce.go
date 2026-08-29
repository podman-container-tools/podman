// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"fmt"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
	oaispec "github.com/go-openapi/spec"
	"github.com/go-openapi/swag/mangling"
)

// reduceDefinitionNames is the final stage of Build: it shortens the fully-qualified,
// compiler-unique definition keys produced during discovery ("<pkgpath>/<name>", see
// EntityDecl.DefKey) back to user-facing names, then re-points every $ref accordingly.
//
// The name ladder (see .claude/plans/name-identity-cyclic-ref.md §9.2/§12):
//
//   - a definition whose leaf name is GLOBALLY UNIQUE is lifted to that
//     bare leaf — byte-identical to the pre-change output, since the leaf
//     IS the name codescan emitted before (G5 zero-churn common case);
//
//   - definitions whose leaf name COLLIDES across packages are qualified
//     with the minimal-depth PascalCase concat of their nearest package
//     segments (b.Test / c.Test -> BTest / CTest), deepening until every
//     name in the group is globally unique (W4-safe), and diagnosed.
//
//   - when a colliding group's best flat concat exceeds the readability
//     budget AND the caller set EmitHierarchicalNames, the group is
//     emitted as nested container definitions instead
//     (placeHierarchical); by default the always-correct flat concat is
//     kept (the concat deepens to the full path if needed).
//
// The whole pass is a pure function of the definition key set, so the output is deterministic
// regardless of discovery / map-iteration order (G4): the previous silent merge was the only source
// of non-determinism and it is gone now that distinct types get distinct keys.
// reduceDefinitionNames returns the old->new rename map it applied, so the caller can re-point
// buffered provenance anchors to the final names (FlushDefOrigins).
//
// A nil/empty map means no definition was renamed.
func (s *Builder) reduceDefinitionNames() map[string]string {
	if len(s.input.Definitions) == 0 {
		return nil
	}

	renames := s.computeNameReductions()
	if len(renames) == 0 {
		return nil
	}

	rewriteAllRefs(s.input, repointer(renames))
	s.rekeyDefinitions(renames)
	s.placeHierarchical(renames)
	s.diagnoseRenames(renames)

	return renames
}

// diagnoseRenames emits a scan.renamed-definition Hint for every definition the reduce stage
// renamed to deconflict a cross-package collision.
//
// The trivial lift of a globally-unique leaf to its bare name is not a rename and is skipped (final
// == leaf).
//
// Unlike the positionless group Warning (diagnoseCollision), each Hint carries the source position
// of the originating Go type — captured during the build in declPos — so a source<->spec
// consumer (the genspec TUI) can follow a renamed type back to its declaration.
//
// No provenance is emitted here: the renamed node's anchor is the normal flushed one under its
// final name (see FlushDefOrigins).
func (s *Builder) diagnoseRenames(renames map[string]string) {
	onDiag := s.ctx.OnDiagnostic()
	if onDiag == nil {
		return
	}

	keys := make([]string, 0, len(renames))
	for key, final := range renames {
		if final != leafName(key) { // skip unique-leaf lifts, keep true renames
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	for _, key := range keys {
		pos, ok := s.declPos[key]
		if !ok {
			continue // no source declaration (e.g. an overlay-supplied definition)
		}
		onDiag(grammar.Hintf(pos, grammar.CodeRenamedDefinition,
			"definition %q renamed to %q to deconflict a cross-package name collision",
			key, renames[key]))
	}
}

// computeNameReductions groups definition keys by their leaf name (the segment after the last '/')
// and returns the rename map old->new:
//
//   - a unique leaf is lifted to the bare leaf;
//   - a colliding leaf is resolved by resolveCollisionGroup (concat).
//
// Names are reserved in a global "taken" set so a concat can never shadow a bare name or another
// group's chosen name (W4).
// Collision groups are processed in sorted order so the result is a pure function of the key set
// (G4).
func (s *Builder) computeNameReductions() map[string]string {
	groups := make(map[string][]string, len(s.input.Definitions))
	for key := range s.input.Definitions {
		leaf := leafName(key)
		groups[leaf] = append(groups[leaf], key)
	}

	renames := make(map[string]string, len(groups))
	taken := make(map[string]struct{}, len(groups))

	// Pass 1: unique leaves -> bare; reserve every bare name first so the concat pass (which must
	// avoid them) sees the full reserved set.
	collisions := make([]string, 0)
	for leaf, keys := range groups {
		if len(keys) == 1 {
			taken[leaf] = struct{}{}
			if keys[0] != leaf {
				renames[keys[0]] = leaf
			}
			continue
		}
		collisions = append(collisions, leaf)
	}
	sort.Strings(collisions)

	// Pass 2: colliding leaves -> minimal-depth concat, or the hierarchical fail-safe when the best
	// flat concat is over budget and the caller opted in.
	mangler := mangling.NewNameMangler()
	budget := s.concatBudget()
	hierarchical := s.ctx.EmitHierarchicalNames()
	containerRoots := map[string]struct{}{} // top-level keys that are containers (shareable)
	for _, leaf := range collisions {
		keys := append([]string(nil), groups[leaf]...)
		sort.Strings(keys)
		chosen, score := resolveCollisionGroup(keys, taken, &mangler)

		if hierarchical && score > budget {
			paths := resolveHierarchicalGroup(keys, taken, containerRoots)
			for _, k := range keys {
				renames[k] = strings.Join(paths[k], "/")
				root := paths[k][0]
				taken[root] = struct{}{}          // a flat concat must not shadow it
				containerRoots[root] = struct{}{} // but another group may share it
			}
			s.diagnoseHierarchical(leaf, keys, paths)
			continue
		}

		for _, k := range keys {
			renames[k] = chosen[k]
			taken[chosen[k]] = struct{}{}
		}
		s.diagnoseCollision(leaf, keys, chosen)
	}
	return renames
}

// concatBudget resolves the effective readability budget: the caller-supplied
// Options.NameConcatBudget, or the built-in default when the caller left it at its zero value.
func (s *Builder) concatBudget() float64 {
	budget := s.ctx.NameConcatBudget()
	if budget <= 0 {
		budget = defaultNameConcatBudget
	}
	return budget
}

// resolveCollisionGroup picks a globally-unique PascalCase concat name for every key sharing a
// leaf.
//
// It tries increasing parent depth (leaf, leaf+1 parent, leaf+2 parents, …) and accepts the
// shallowest depth at which every name in the group is mutually distinct AND free of the taken set.
// If even the full path is not enough (ToGoName is not injective on arbitrary paths), a
// deterministic numeric suffix breaks the remaining tie.
//
// It also returns the group's worst (least-readable) concatScore, which the caller checks against
// the configured budget — the seam for the hierarchical fallback (Stage 3 / K3).
func resolveCollisionGroup(keys []string, taken map[string]struct{}, m *mangling.NameMangler) (map[string]string, float64) {
	segs := make(map[string][]string, len(keys))
	maxDepth := 0
	for _, k := range keys {
		parts := strings.Split(k, "/")
		segs[k] = parts
		if d := len(parts) - 1; d > maxDepth {
			maxDepth = d
		}
	}

	for d := 1; d <= maxDepth; d++ {
		cand := make(map[string]string, len(keys))
		seen := make(map[string]struct{}, len(keys))
		ok := true
		for _, k := range keys {
			name := concatName(segs[k], d, m)
			if _, clash := taken[name]; clash {
				ok = false
				break
			}
			if _, dup := seen[name]; dup {
				ok = false
				break
			}
			seen[name] = struct{}{}
			cand[k] = name
		}
		if ok {
			return cand, groupWorstScore(keys, cand, segs, d)
		}
	}

	// Pathological fallback: full-path concat with a numeric suffix.
	cand := make(map[string]string, len(keys))
	seen := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		base := concatName(segs[k], maxDepth, m)
		name := base
		for i := 2; ; i++ {
			_, clashTaken := taken[name]
			_, clashSeen := seen[name]
			if !clashTaken && !clashSeen {
				break
			}
			name = base + strconv.Itoa(i)
		}
		seen[name] = struct{}{}
		cand[k] = name
	}
	return cand, groupWorstScore(keys, cand, segs, maxDepth)
}

// concatName PascalCase-concatenates the leaf with its d nearest parent segments (clamped to the
// available depth).
func concatName(segs []string, d int, m *mangling.NameMangler) string {
	return m.ToGoName(strings.Join(concatParts(segs, d), " "))
}

// concatParts returns the leaf plus its d nearest parent segments (clamped to the available depth)
// — the raw words concatName joins.
func concatParts(segs []string, d int) []string {
	start := max(len(segs)-d-1, 0)
	return segs[start:]
}

// defaultNameConcatBudget is the readability cutoff used when the caller leaves
// Options.NameConcatBudget at its zero value.
//
// Empirically chosen (see concat_score_test.go): comfortably accepts the common 2-segment concats
// while fencing off long 3-segment ones.
const defaultNameConcatBudget = 0.65

// concatScore rates a deconflicted concat name in [0,1]: lower is more readable.
//
// It blends the three criteria — total length, part count and longest segment — each normalized
// to [0,1] against a budget, with weights summing to 1, so every criterion pushes the score UP as
// it grows.
// A 4+-part concat is always 1.0 (ruled out).
//
// The weights and budgets are the tuning knobs; the function is exercised against alternatives in
// the (disabled) concat_score_test.go harness.
func concatScore(concat string, parts []string) float64 {
	const (
		maxParts = 3.00
		lBudget  = 24.00
		wBudget  = 12.00

		wLen   = 0.50
		wParts = 0.30
		wWord  = 0.20

		scoreFloor = 0.00
		scoreCeil  = 1.00
	)

	if len(parts) == 0 {
		return scoreFloor
	}
	if len(parts) > int(maxParts) {
		return scoreCeil
	}

	maxWord := 0
	for _, p := range parts {
		if l := len(p); l > maxWord {
			maxWord = l
		}
	}

	lenTerm := float64(len(concat)) / lBudget
	partsTerm := float64(len(parts)-1) / (maxParts - 1) // n=2 -> 0.5, n=3 -> 1.0
	wordTerm := float64(maxWord) / wBudget

	return min(scoreCeil, wLen*lenTerm+wParts*partsTerm+wWord*wordTerm)
}

// groupWorstScore returns the least-readable (highest) concatScore across a resolved collision
// group, the value the budget is checked against.
func groupWorstScore(keys []string, cand map[string]string, segs map[string][]string, d int) float64 {
	worst := 0.0
	for _, k := range keys {
		if sc := concatScore(cand[k], concatParts(segs[k], d)); sc > worst {
			worst = sc
		}
	}
	return worst
}

// resolveHierarchicalGroup picks, per key, the nested container path [parent…, leaf] used for the
// hierarchical fail-safe — the same nearest package segments the flat concat would have joined,
// kept as raw nesting levels instead.
//
// It uses the shallowest depth at which every path is mutually distinct AND no root container
// clashes with an already-taken flat name; the full key path is the always-unique backstop.
//
// Unlike the flat rung this never needs ToGoName (the segments stay raw JSON keys), so distinct
// segment sequences always yield distinct pointers — no numeric-suffix fallback is required.
func resolveHierarchicalGroup(keys []string, taken, containerRoots map[string]struct{}) map[string][]string {
	segs := make(map[string][]string, len(keys))
	maxDepth := 0
	for _, k := range keys {
		parts := strings.Split(k, "/")
		segs[k] = parts
		if d := len(parts) - 1; d > maxDepth {
			maxDepth = d
		}
	}

	for d := 1; d <= maxDepth; d++ {
		cand := make(map[string][]string, len(keys))
		seen := make(map[string]struct{}, len(keys))
		ok := true
		for _, k := range keys {
			path := concatParts(segs[k], d)
			if _, reserved := taken[path[0]]; reserved {
				if _, shareable := containerRoots[path[0]]; !shareable {
					ok = false // root would shadow a flat definition
					break
				}
			}
			joined := strings.Join(path, "/")
			if _, dup := seen[joined]; dup {
				ok = false
				break
			}
			seen[joined] = struct{}{}
			cand[k] = path
		}
		if ok {
			return cand
		}
	}

	// Backstop: the full key path (pkg path + leaf) is unique by construction (DefKey is
	// compiler-unique).
	cand := make(map[string][]string, len(keys))
	for _, k := range keys {
		cand[k] = segs[k]
	}
	return cand
}

// diagnoseCollision reports a cross-package definition-name collision and the distinct names the
// reduce stage minted for it.
//
// Positionless: by reduce time the per-declaration source positions are gone (the spec is the only
// memory), and the collision is a property of the global name set, not a single source line.
func (s *Builder) diagnoseCollision(leaf string, keys []string, chosen map[string]string) {
	onDiag := s.ctx.OnDiagnostic()
	if onDiag == nil {
		return
	}
	mappings := make([]string, 0, len(keys))
	for _, k := range keys {
		pkg := k
		if i := strings.LastIndex(k, "/"); i >= 0 {
			pkg = k[:i]
		}
		mappings = append(mappings, fmt.Sprintf("%s -> %s", pkg, chosen[k]))
	}
	onDiag(grammar.Warnf(token.Position{}, grammar.CodeCollidingModelName,
		"model name %q is declared in %d packages; emitting distinct names [%s]; "+
			"set `swagger:model <name>` to control them",
		leaf, len(keys), strings.Join(mappings, ", ")))
}

// rekeyDefinitions moves each definition from its old key to its new name.
//
// A clash on the new name cannot occur for unique leaves; it is guarded defensively, skipping the
// rekey rather than silently overwriting.
func (s *Builder) rekeyDefinitions(renames map[string]string) {
	for old, nw := range renames {
		if strings.Contains(nw, "/") {
			continue // nested path: placed by placeHierarchical, not a flat rekey
		}
		schema, ok := s.input.Definitions[old]
		if !ok {
			continue
		}
		if _, clash := s.input.Definitions[nw]; clash {
			continue
		}
		s.input.Definitions[nw] = schema
		delete(s.input.Definitions, old)
	}
}

const extGoPackage = "x-go-package"

// diagnoseHierarchical reports that a colliding name was emitted as nested container definitions
// because its flat concat was over budget, listing the nested pointer minted for each package.
//
// Positionless, like diagnoseCollision.
func (s *Builder) diagnoseHierarchical(leaf string, keys []string, paths map[string][]string) {
	onDiag := s.ctx.OnDiagnostic()
	if onDiag == nil {
		return
	}
	mappings := make([]string, 0, len(keys))
	for _, k := range keys {
		pkg := k
		if i := strings.LastIndex(k, "/"); i >= 0 {
			pkg = k[:i]
		}
		mappings = append(mappings, fmt.Sprintf("%s -> #/definitions/%s", pkg, strings.Join(paths[k], "/")))
	}
	onDiag(grammar.Warnf(token.Position{}, grammar.CodeHierarchicalModelName,
		"model name %q collides across %d packages and its flat qualified name "+
			"exceeds the readability budget; emitting nested definitions [%s]; "+
			"set `swagger:model <name>` for a flat name",
		leaf, len(keys), strings.Join(mappings, ", ")))
}

// placeHierarchical realises the nested-container definitions for the renames whose target is a
// slash path (produced by resolveHierarchicalGroup).
//
// For each, it moves the model schema from its flat key into
// definitions[<root>]…ExtraProps[<leaf>], creating the container chain
// (additionalProperties:true, plus x-go-package on the innermost container) and merging into any
// container shared with another model.
//
// The matching $refs were already pointed at the slash path by the shared ref rewrite.
func (s *Builder) placeHierarchical(renames map[string]string) {
	skipExt := s.ctx.SkipExtensions()
	for old, nw := range renames {
		if !strings.Contains(nw, "/") {
			continue
		}
		schema, ok := s.input.Definitions[old]
		if !ok {
			continue
		}
		pkgPath := old
		if i := strings.LastIndex(old, "/"); i >= 0 {
			pkgPath = old[:i]
		}
		placeNested(s.input.Definitions, strings.Split(nw, "/"), schema, pkgPath, skipExt)
		delete(s.input.Definitions, old)
	}
}

// placeNested inserts model at definitions[path[0]]…[path[n-1]] where the last segment is the
// model's leaf key and the earlier segments are container schemas; pkgPath is stamped (unless
// skipExt) on the innermost container.
//
// Existing containers are merged into, not overwritten, so two models sharing a container coexist.
func placeNested(defs oaispec.Definitions, path []string, model oaispec.Schema, pkgPath string, skipExt bool) {
	root := path[0]
	container := defs[root] // zero Schema when absent
	ensureContainer(&container)
	if len(path) == 2 { //nolint:mnd // [container, leaf]
		setContainerChild(&container, path[1], model)
		stampGoPackage(&container, pkgPath, skipExt)
	} else {
		placeNestedExtra(&container, path[1:], model, pkgPath, skipExt)
	}
	defs[root] = container
}

// placeNestedExtra recurses into parent's ExtraProps to realise the remaining container chain. path
// is [container…, leaf] with len >= 2.
func placeNestedExtra(parent *oaispec.Schema, path []string, model oaispec.Schema, pkgPath string, skipExt bool) {
	head := path[0]
	child := childSchema(parent, head)
	ensureContainer(&child)
	if len(path) == 2 { //nolint:mnd // [container, leaf]
		setContainerChild(&child, path[1], model)
		stampGoPackage(&child, pkgPath, skipExt)
	} else {
		placeNestedExtra(&child, path[1:], model, pkgPath, skipExt)
	}
	setContainerChild(parent, head, child)
}

// ensureContainer marks a schema as a lenient container (W2: additional properties cover the nested
// model so validation stays permissive).
func ensureContainer(s *oaispec.Schema) {
	if s.AdditionalProperties == nil {
		s.AdditionalProperties = &oaispec.SchemaOrBool{Allows: true}
	}
}

func stampGoPackage(s *oaispec.Schema, pkgPath string, skipExt bool) {
	if skipExt {
		return
	}
	s.AddExtension(extGoPackage, pkgPath)
}

// setContainerChild stores child under key in parent's ExtraProps — the carrier go-openapi/spec
// uses for nested-definition members (see W2).
func setContainerChild(parent *oaispec.Schema, key string, child oaispec.Schema) {
	if parent.ExtraProps == nil {
		parent.ExtraProps = map[string]any{}
	}
	parent.ExtraProps[key] = child
}

// childSchema returns the existing container stored under key in parent's ExtraProps, or a zero
// Schema when absent / of another type.
func childSchema(parent *oaispec.Schema, key string) oaispec.Schema {
	if parent.ExtraProps != nil {
		if existing, ok := parent.ExtraProps[key].(oaispec.Schema); ok {
			return existing
		}
	}
	return oaispec.Schema{}
}

// leafName returns the segment of a definition key after the last '/', or the whole key when there
// is none.
func leafName(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[i+1:]
	}
	return key
}

// repointer returns a function that rewrites a "#/definitions/<old>" reference to
// "#/definitions/<new>" when <old> is in the rename map, and reports whether it changed.
func repointer(renames map[string]string) func(oaispec.Ref) (oaispec.Ref, bool) {
	const prefix = "#/definitions/"
	return func(ref oaispec.Ref) (oaispec.Ref, bool) {
		s := ref.String()
		if !strings.HasPrefix(s, prefix) {
			return ref, false
		}
		nw, ok := renames[s[len(prefix):]]
		if !ok {
			return ref, false
		}
		nref, err := oaispec.NewRef(prefix + nw)
		if err != nil {
			return ref, false
		}
		return nref, true
	}
}

// rewriteAllRefs walks every schema-bearing node of the document and repoints its $ref in place.
//
// It covers the locations codescan actually emits definition refs: definitions, shared
// parameters/responses schemas, and per-operation parameter / response schemas.
//
// Parameter, response and path-item self-$refs are skipped — codescan never points those at
// #/definitions. (Walk structure mirrors go-openapi/analysis's analyzer, minus the parts we never
// emit.)
func rewriteAllRefs(sw *oaispec.Swagger, repoint func(oaispec.Ref) (oaispec.Ref, bool)) {
	for k, v := range sw.Definitions {
		rewriteSchemaRefs(&v, repoint)
		sw.Definitions[k] = v
	}
	// Shared parameters / responses carry their schema by pointer, so mutating through it needs no
	// write-back to the map.
	for _, p := range sw.Parameters {
		rewriteSchemaRefs(p.Schema, repoint)
	}
	for _, r := range sw.Responses {
		rewriteSchemaRefs(r.Schema, repoint)
	}
	if sw.Paths == nil {
		return
	}
	for _, pi := range sw.Paths.Paths {
		for _, op := range operationsOf(pi) {
			if op == nil {
				continue
			}
			for i := range op.Parameters {
				rewriteSchemaRefs(op.Parameters[i].Schema, repoint)
			}
			if op.Responses == nil {
				continue
			}
			if op.Responses.Default != nil {
				rewriteSchemaRefs(op.Responses.Default.Schema, repoint)
			}
			for code, resp := range op.Responses.StatusCodeResponses {
				if resp.Schema != nil {
					rewriteSchemaRefs(resp.Schema, repoint)
					op.Responses.StatusCodeResponses[code] = resp
				}
			}
		}
	}
}

// operationsOf returns the (possibly nil) operation pointers of a path item, in a fixed order.
func operationsOf(pi oaispec.PathItem) []*oaispec.Operation {
	return []*oaispec.Operation{
		pi.Get, pi.Put, pi.Post, pi.Delete, pi.Options, pi.Head, pi.Patch,
	}
}

// rewriteSchemaRefs repoints the $ref on sch and recurses into every sub-schema.
//
// Schema-by-value containers (Properties, nested Definitions, …) are written back after mutation;
// pointer- and slice-addressable containers are mutated in place.
//
// nil-guards and the SchemaOrArray (single vs tuple) / SchemaOrBool (.Schema may be nil) handling
// follow the quirks documented in go-openapi/analysis.
func rewriteSchemaRefs(sch *oaispec.Schema, repoint func(oaispec.Ref) (oaispec.Ref, bool)) {
	if sch == nil {
		return
	}
	if sch.Ref.String() != "" {
		if nref, ok := repoint(sch.Ref); ok {
			sch.Ref = nref
		}
	}
	for k, v := range sch.Properties {
		rewriteSchemaRefs(&v, repoint)
		sch.Properties[k] = v
	}
	for k, v := range sch.PatternProperties {
		rewriteSchemaRefs(&v, repoint)
		sch.PatternProperties[k] = v
	}
	for i := range sch.AllOf {
		rewriteSchemaRefs(&sch.AllOf[i], repoint)
	}
	for i := range sch.AnyOf {
		rewriteSchemaRefs(&sch.AnyOf[i], repoint)
	}
	for i := range sch.OneOf {
		rewriteSchemaRefs(&sch.OneOf[i], repoint)
	}
	rewriteSchemaRefs(sch.Not, repoint)
	if sch.Items != nil {
		rewriteSchemaRefs(sch.Items.Schema, repoint)
		for i := range sch.Items.Schemas {
			rewriteSchemaRefs(&sch.Items.Schemas[i], repoint)
		}
	}
	if sch.AdditionalProperties != nil {
		rewriteSchemaRefs(sch.AdditionalProperties.Schema, repoint)
	}
	if sch.AdditionalItems != nil {
		rewriteSchemaRefs(sch.AdditionalItems.Schema, repoint)
	}
	for k, v := range sch.Definitions {
		rewriteSchemaRefs(&v, repoint)
		sch.Definitions[k] = v
	}
}
