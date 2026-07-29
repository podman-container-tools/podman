// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"strings"

	"github.com/go-openapi/codescan/internal/builders/godoclink"
	oaispec "github.com/go-openapi/spec"
)

// substituteGodocMarkers is the post-reduce half of CleanGoDoc idiom recomposition: it walks every
// godoc-derived title / description in the finished document and rewrites each godoclink marker to
// the exposed name of the schema it references. renames maps a pre-reduce definition key to its
// final user-facing name; a marker whose key is not an emitted definition (pruned / unresolved)
// collapses to its humanized fallback.
//
// §3.
func (s *Builder) substituteGodocMarkers(renames map[string]string) {
	finalName := func(defKey string) (string, bool) {
		if n, ok := renames[defKey]; ok {
			return n, true
		}
		// No rename entry: the definition kept its default short name (the leaf of the key).
		// Confirm it survived to the final document; otherwise the marker falls back.
		leaf := defKey[strings.LastIndex(defKey, "/")+1:]
		if _, ok := s.input.Definitions[leaf]; ok {
			return leaf, true
		}

		return "", false
	}

	subst := func(in string) string { return godoclink.SubstituteMarkers(in, finalName) }
	walkSpecProse(s.input, subst)
}

// walkSpecProse applies subst to every prose field (title / description / summary) reachable in sw
// — the info block, definitions, shared parameters and responses, and every operation under
// paths.
//
// It mirrors reduce.go's rewriteAllRefs traversal.
func walkSpecProse(sw *oaispec.Swagger, subst func(string) string) {
	if sw.Info != nil {
		sw.Info.Title = subst(sw.Info.Title)
		sw.Info.Description = subst(sw.Info.Description)
	}

	for k, v := range sw.Definitions {
		walkSchemaProse(&v, subst)
		sw.Definitions[k] = v
	}
	for k, p := range sw.Parameters {
		p.Description = subst(p.Description)
		walkSchemaProse(p.Schema, subst)
		sw.Parameters[k] = p
	}
	for k, r := range sw.Responses {
		walkResponseProse(&r, subst)
		sw.Responses[k] = r
	}

	if sw.Paths == nil {
		return
	}
	for k, pi := range sw.Paths.Paths {
		walkPathItemProse(&pi, subst)
		sw.Paths.Paths[k] = pi
	}
}

// walkPathItemProse applies subst to a path item's shared parameters and every operation's summary
// / description, parameters and responses.
func walkPathItemProse(pi *oaispec.PathItem, subst func(string) string) {
	for i := range pi.Parameters {
		pi.Parameters[i].Description = subst(pi.Parameters[i].Description)
		walkSchemaProse(pi.Parameters[i].Schema, subst)
	}

	for _, op := range operationsByMethod(pi) {
		if op == nil {
			continue
		}
		op.Summary = subst(op.Summary)
		op.Description = subst(op.Description)
		for i := range op.Parameters {
			op.Parameters[i].Description = subst(op.Parameters[i].Description)
			walkSchemaProse(op.Parameters[i].Schema, subst)
		}
		if op.Responses == nil {
			continue
		}
		if op.Responses.Default != nil {
			walkResponseProse(op.Responses.Default, subst)
		}
		for code, r := range op.Responses.StatusCodeResponses {
			walkResponseProse(&r, subst)
			op.Responses.StatusCodeResponses[code] = r
		}
	}
}

// walkResponseProse applies subst to a response's description, headers and schema.
func walkResponseProse(r *oaispec.Response, subst func(string) string) {
	r.Description = subst(r.Description)
	for k, h := range r.Headers {
		h.Description = subst(h.Description)
		r.Headers[k] = h
	}
	walkSchemaProse(r.Schema, subst)
}

// walkSchemaProse applies subst to a schema's title / description and recurses through every nested
// schema (properties, composition arms, items, maps).
func walkSchemaProse(sch *oaispec.Schema, subst func(string) string) {
	if sch == nil {
		return
	}

	sch.Title = subst(sch.Title)
	sch.Description = subst(sch.Description)

	for k, p := range sch.Properties {
		walkSchemaProse(&p, subst)
		sch.Properties[k] = p
	}
	for k, p := range sch.PatternProperties {
		walkSchemaProse(&p, subst)
		sch.PatternProperties[k] = p
	}
	for i := range sch.AllOf {
		walkSchemaProse(&sch.AllOf[i], subst)
	}
	for i := range sch.AnyOf {
		walkSchemaProse(&sch.AnyOf[i], subst)
	}
	for i := range sch.OneOf {
		walkSchemaProse(&sch.OneOf[i], subst)
	}
	if sch.Items != nil {
		walkSchemaProse(sch.Items.Schema, subst)
		for i := range sch.Items.Schemas {
			walkSchemaProse(&sch.Items.Schemas[i], subst)
		}
	}
	if sch.AdditionalProperties != nil {
		walkSchemaProse(sch.AdditionalProperties.Schema, subst)
	}
	for k, d := range sch.Definitions {
		walkSchemaProse(&d, subst)
		sch.Definitions[k] = d
	}
}
