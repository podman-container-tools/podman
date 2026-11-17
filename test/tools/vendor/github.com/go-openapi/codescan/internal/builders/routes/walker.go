// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"fmt"
	"go/token"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-openapi/codescan/internal/builders/handlers"
	"github.com/go-openapi/codescan/internal/builders/validations"
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/parsers/routebody"
	"github.com/go-openapi/codescan/internal/scanner"
	oaispec "github.com/go-openapi/spec"
)

// primitiveTypes lists the OAS v2 primitive type spellings routebody
// accepts on `type:` for body parameters. A body type matching this
// set lands on param.Schema.Type as a typed schema; anything else is
// treated as a model reference and resolved via $ref.
//
//nolint:gochecknoglobals // immutable lookup table; read-only.
var primitiveTypes = map[string]struct{}{
	"string":  {},
	"integer": {},
	"number":  {},
	"boolean": {},
	"array":   {},
	"object":  {},
}

// responseBodyPrimitives are the OAS v2 scalar primitive type spellings
// accepted as a `body:` type on a swagger:route response line. Arrays
// are expressed with the `[]` prefix (`body:[]string`), so the bare
// word `array` is intentionally absent. `object` is absent too — a
// free-form/structured object body is declared by referencing a model
// (`body:MyType`), not by the bare keyword.
//
//nolint:gochecknoglobals // immutable lookup table; read-only.
var responseBodyPrimitives = map[string]struct{}{
	"string":  {},
	"number":  {},
	"integer": {},
	"boolean": {},
}

// responseBodyReservedTypes are OAS/JSON type keywords that look like a
// body type but are NOT valid as a `body:` response type: `array` /
// `object` (use `[]T` / a model name) and `file` / `null` (unsupported
// in this context — `file` would need extra produces/context checks).
// They draw a diagnostic instead of being resolved as a model $ref.
//
//nolint:gochecknoglobals // immutable lookup table; read-only.
var responseBodyReservedTypes = map[string]struct{}{
	"array":  {},
	"object": {},
	"file":   {},
	"null":   {},
}

// applyBlockToRoute parses route.Remaining through grammar and writes Summary / Description /
// per-keyword content onto op.
//
// Grammar's lexer classifies prose into TokenTitle / TokenDesc directly and isolates every
// route-level keyword into a Property — `schemes:`, `deprecated:`, `consumes:`, `produces:`,
// `security:`, `parameters:`, `responses:`, `extensions:`.
//
// Level-0 properties dispatch to dispatchRouteKeyword; items-depth properties are skipped (they
// belong to a nested schema, not the route header).
//
// route.Remaining is the *ast.CommentGroup AFTER the swagger:route header line has been stripped by
// parsers.ParseRoutePathAnnotation; grammar sees it as an UnboundBlock whose Title / Description /
// Properties behave identically to a properly-anchored block.
func (r *Builder) applyBlockToRoute(op *oaispec.Operation) error {
	block := grammar.NewParser(r.Ctx.FileSet(),
		grammar.WithSingleLineCommentAsDescription(r.Ctx.SingleLineCommentAsDescription())).Parse(r.route.Remaining)

	op.Summary = r.CleanGoDoc(block.Title())
	op.Description = r.CleanGoDoc(block.Description())

	for prop := range block.Properties() {
		if prop.ItemsDepth != 0 {
			continue
		}
		if err := r.dispatchRouteKeyword(prop, op); err != nil {
			return err
		}
		r.recordRouteKeywordOrigin(prop)
	}

	// Extensions and security are read straight off the block — grammar's lexer routes their raw
	// bodies through typed sub-parsers (yaml.TypedExtensions, security.Parse) at lex time, so the
	// dispatcher above skips them and we read the typed surface here.
	// See [§extensions](./README.md#extensions).
	for ext := range block.Extensions() {
		op.AddExtension(ext.Name, ext.Value)
	}
	if reqs := block.SecurityRequirements(); reqs != nil {
		op.Security = reqs
	}

	return nil
}

// recordRouteKeywordOrigin anchors one route-level keyword to its source line under
// /paths/{path}/{method}/{seg}, when a provenance sink is wired.
//
// The keyword→segment knowledge lives in the grammar ([grammar.PointerPath]); here we prepend the
// operation base. parameters/responses (containers) and security (consumed at lex time) are absent
// and resolve to the operation anchor.
func (r *Builder) recordRouteKeywordOrigin(p grammar.Property) {
	if !r.Ctx.OriginEnabled() {
		return
	}
	segs, ok := grammar.PointerPath(p.Keyword, grammar.CtxRoute)
	if !ok {
		return
	}
	base := scanner.JSONPointer("paths", r.route.Path, strings.ToLower(r.route.Method))
	r.Ctx.RecordOrigin(base+scanner.JSONPointer(segs...), p.Pos)
}

// dispatchRouteKeyword routes one grammar Property to the matching body parser.
//
// List-shaped keywords (schemes / consumes / produces) flow through Property.AsList, which unifies
// inline comma-lists, multi-line bare-line bodies, and YAML-style `- ` markers.
// Inline- keyword shapes (deprecated bool) read Property.Typed directly.
//
// Routebody body parsers (parameters / responses) own their own orchestration; extensions ride
// grammar's typed-extensions surface (see applyBlockToRoute above).
func (r *Builder) dispatchRouteKeyword(p grammar.Property, op *oaispec.Operation) error {
	switch p.Keyword.Name {
	case grammar.KwSchemes:
		if v := p.AsList(); len(v) > 0 {
			op.Schemes = v
		}
	case grammar.KwDeprecated:
		if p.IsTyped() {
			op.Deprecated = p.Typed.Boolean
		}
	case grammar.KwConsumes:
		op.Consumes = p.AsList()
	case grammar.KwProduces:
		op.Produces = p.AsList()
	case grammar.KwTags:
		// `Tags:` on a route is a plain string list of tag names, unioned onto any tags already parsed
		// off the swagger:route header line (go-swagger#2655).
		// Duplicates are dropped, source order preserved.
		// The meta `Tags:` object shape is handled by a different builder (the spec/meta walker).
		op.Tags = unionTags(op.Tags, p.AsList())
	case grammar.KwParameters:
		return r.dispatchParameters(p, op)
	case grammar.KwResponses:
		return r.dispatchResponses(p, op)
	case grammar.KwExternalDocs:
		ed, err := handlers.ParseExternalDocs(p.Body)
		if err != nil {
			r.RecordDiagnostic(grammar.Warnf(p.Pos, grammar.CodeInvalidAnnotation, "externalDocs: %v", err))
			return nil
		}
		if ed != nil {
			op.ExternalDocs = ed
		}
	}
	return nil
}

// unionTags appends src tag names to dst, dropping any already present, and returns the merged
// slice.
//
// Source order is preserved: header-line tags (in dst) come first, then any new `Tags:`-keyword
// names.
// Used to merge route-header and route-body tag sources.
func unionTags(dst, src []string) []string {
	if len(src) == 0 {
		return dst
	}
	seen := make(map[string]struct{}, len(dst)+len(src))
	for _, t := range dst {
		seen[t] = struct{}{}
	}
	for _, t := range src {
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		dst = append(dst, t)
	}
	return dst
}

// dispatchParameters lowers a `Parameters:` raw body via routebody then dispatches each ParamDecl
// through the standard handlers seam.
//
// Non-body params route through handlers.DispatchParamLevel0 (SimpleSchema); body params route
// through handlers.DispatchSchemaLevel0 onto a freshly-allocated param.Schema.
func (r *Builder) dispatchParameters(p grammar.Property, op *oaispec.Operation) error {
	decls := routebody.ParseParameters(p.Body, p.Pos, r.RecordDiagnostic)
	for i := range decls {
		decl := &decls[i]
		param := r.buildRouteParam(decl)
		if param == nil {
			continue
		}
		op.AddParam(param)
	}
	return nil
}

// buildRouteParam materialises one ParamDecl into a *spec.Parameter and dispatches its validation
// Block through the handlers seam.
//
// Returns nil when the decl is too thin to form a valid parameter (no name and no in — likely a
// fixture quirk routebody could not fully diagnose).
func (r *Builder) buildRouteParam(decl *routebody.ParamDecl) *oaispec.Parameter {
	if decl.Name == "" && decl.In == "" {
		return nil
	}
	param := &oaispec.Parameter{
		ParamProps: oaispec.ParamProps{
			Name:            decl.Name,
			In:              decl.In,
			Description:     decl.Description,
			Required:        decl.Required,
			AllowEmptyValue: decl.AllowEmpty,
		},
	}

	if decl.In == "body" {
		param.Schema = r.buildBodySchema(decl)
		// Body parameters' Description lives only on the parameter — the referenced model owns the
		// schema-level description.
		// Format is the only inline schema-level override routebody preserves on body.
		handlers.DispatchSchemaLevel0(decl.Block, nil, param.Schema, "", r.RecordDiagnostic, handlers.SchemaOptions{})
		return param
	}

	// SimpleSchema (path/query/header/formData) — populate type from head fields, then dispatch
	// validations through a type-gated Block.
	//
	// Format is applied AFTER dispatch on purpose: go-openapi's SimpleSchema.TypeName() returns Format
	// when non-empty (so validations.CoerceValue would key default/example coercion off the format
	// string instead of the type), which conflicts with the author's clear intent.
	// Setting Format post-dispatch keeps the scheme.Type stable through coercion.
	if decl.TypeRef != "" {
		param.Type = normaliseSimpleType(decl.TypeRef)
	}
	gated := r.typeGateBlock(decl.Block, param.Type, decl.Pos)
	if err := handlers.DispatchParamLevel0(gated, param, r.RecordDiagnostic); err != nil {
		// Surface the first coercion error as a diagnostic so the author sees the loss rather than
		// dropping it silently.
		r.RecordDiagnostic(grammar.Diagnostic{
			Pos:      decl.Pos,
			Severity: grammar.SeverityWarning,
			Code:     grammar.CodeInvalidAnnotation,
			Message:  err.Error(),
		})
	}
	param.Format = decl.Format
	return param
}

// buildBodySchema materialises a body parameter's Schema from a ParamDecl.
//
// Primitive types (string/integer/number/boolean/array/ object) become a typed schema; anything
// else is treated as a model reference and resolved via $ref (with optional `[]` array layer
// wrapping).
func (r *Builder) buildBodySchema(decl *routebody.ParamDecl) *oaispec.Schema {
	if decl.TypeRef == "" {
		return new(oaispec.Schema)
	}
	if _, prim := primitiveTypes[decl.TypeRef]; prim {
		schema := &oaispec.Schema{
			SchemaProps: oaispec.SchemaProps{
				Type: oaispec.StringOrArray{decl.TypeRef},
			},
		}
		if decl.Format != "" {
			schema.Format = decl.Format
		}
		return schema
	}
	schema := r.resolveBodySchema(decl.TypeRef, 0)
	if schema == nil {
		return new(oaispec.Schema)
	}
	if decl.Format != "" {
		schema.Format = decl.Format
	}
	return schema
}

// typeGateBlock returns a Block containing only the Properties from in whose Keyword applies to
// schemaType per validations.IsLegalForType.
//
// Dropped properties emit CodeShapeMismatch diagnostics so the author sees the loss (incompatible
// validations are dropped rather than left to produce a malformed spec).
//
// schemaType may be empty for params with no explicit `type:` head; IsLegalForType admits every
// keyword on the empty-type sentinel so nothing is gated.
func (r *Builder) typeGateBlock(in grammar.Block, schemaType string, pos token.Position) grammar.Block {
	if in == nil {
		return in
	}
	// No declared type means no meaningful validation surface: drop every property on a type-less
	// SimpleSchema param and surface a diagnostic per dropped keyword.
	if schemaType == "" {
		had := false
		for p := range in.Properties() {
			had = true
			r.RecordDiagnostic(grammar.Diagnostic{
				Pos:      p.Pos,
				Severity: grammar.SeverityWarning,
				Code:     grammar.CodeShapeMismatch,
				Message: fmt.Sprintf(
					"validation %q dropped: parameter has no declared `type:` to validate against",
					p.Keyword.Name,
				),
			})
		}
		if had {
			return grammar.NewSyntheticBlock(pos, in.Title(), in.Description(), nil)
		}
		return in
	}
	var filtered []grammar.Property
	for p := range in.Properties() {
		ok, hint := validations.IsLegalForType(p.Keyword, schemaType)
		if !ok {
			r.RecordDiagnostic(grammar.Diagnostic{
				Pos:      p.Pos,
				Severity: grammar.SeverityWarning,
				Code:     grammar.CodeShapeMismatch,
				Message:  hint,
			})
			continue
		}
		filtered = append(filtered, p)
	}
	return grammar.NewSyntheticBlock(pos, in.Title(), in.Description(), filtered)
}

// normaliseSimpleType maps short type spellings to their OAS v2 canonical forms.
//
// Only `bool` → `boolean` is meaningful at present; the other primitive names (`string`,
// `integer`, `number`, `boolean`, `array`) pass through unchanged.
func normaliseSimpleType(t string) string {
	if t == "bool" {
		return "boolean"
	}
	return t
}

// primitiveBodySchema returns a typed primitive Schema when name is an OAS v2 scalar primitive type
// spelling (per responseBodyPrimitives), wrapped in `arrays` nested array layers; it returns nil
// otherwise so the caller can diagnose a reserved keyword or fall back to model reference
// resolution.
//
// This gives swagger:route responses a path to a primitive body via the unambiguous `body:` tag —
// `200: body:string` → {schema: {type: string}}, `200: body:[]integer` → {schema: {type: array,
// items: {type: integer}}} — mirroring buildBodySchema's primitive handling for body parameters
// (go-swagger#2942).
//
// The bare/untagged `200: string` form is NOT promoted (an untagged token is a response name, not a
// type).
func primitiveBodySchema(name string, arrays int) *oaispec.Schema {
	if _, ok := responseBodyPrimitives[name]; !ok {
		return nil
	}
	leaf := &oaispec.Schema{SchemaProps: oaispec.SchemaProps{Type: oaispec.StringOrArray{name}}}
	for range arrays {
		leaf = &oaispec.Schema{SchemaProps: oaispec.SchemaProps{
			Type:  oaispec.StringOrArray{"array"},
			Items: &oaispec.SchemaOrArray{Schema: leaf},
		}}
	}
	return leaf
}

// resolveBodySchema builds a Schema for a body param/response's type reference. arrayLayer is the
// number of `[]` array wrappers already stripped from the ref; the function applies them as nested
// array Schemas around the final $ref.
//
// The resulting Schema is best-effort: the orchestrator does NOT gate on existence in r.definitions
// because the swagger:model pass may emit the definition independently.
// A dangling $ref preserves the author's spec-first intent (the "force-the-spec" reading) without
// silent loss.
//
// Response-side callers additionally check for unresolvable refs and emit CodeInvalidAnnotation
// diagnostics; on the parameter side we trust the author.
func (r *Builder) resolveBodySchema(ref string, arrayLayer int) *oaispec.Schema {
	if ref == "" {
		return nil
	}
	// Strip any remaining [] prefixes the caller didn't consume.
	for strings.HasPrefix(ref, "[]") {
		arrayLayer++
		ref = ref[2:]
	}
	if ref == "" {
		return nil
	}
	target, err := oaispec.NewRef("#/definitions/" + ref)
	if err != nil {
		return nil
	}
	// Innermost schema carries the $ref; nested arrays wrap from outside in.
	leaf := &oaispec.Schema{SchemaProps: oaispec.SchemaProps{Ref: target}}
	for range arrayLayer {
		leaf = &oaispec.Schema{
			SchemaProps: oaispec.SchemaProps{
				Type: oaispec.StringOrArray{"array"},
				Items: &oaispec.SchemaOrArray{
					Schema: leaf,
				},
			},
		}
	}
	return leaf
}

// resolveDefinitionByLeaf reports whether the definitions map holds a definition whose leaf name
// (the segment after the last '/') equals short, and whether more than one does.
//
// During build the definitions map is keyed by the fully-qualified identity (pkgpath/name — see
// scanner.EntityDecl.DefKey), while author annotations reference models by their short name; a leaf
// lookup bridges the two until the spec.Builder's reduce stage shortens unique leaves back to bare
// names.
//
// `ambiguous` means several cross-package definitions share the leaf — a real collision the short
// name cannot resolve (name-identity design §12.1).
func resolveDefinitionByLeaf(defs map[string]oaispec.Schema, short string) (key string, found, ambiguous bool) {
	for k := range defs {
		if leafOfKey(k) != short {
			continue
		}
		if found {
			return "", true, true
		}
		key, found = k, true
	}
	return key, found, false
}

// leafOfKey returns the segment of a definition key after the last '/', or the whole key when there
// is none.
func leafOfKey(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[i+1:]
	}
	return key
}

// dispatchResponses lowers a `Responses:` raw body via routebody then assembles each ResponseDecl
// into op.Responses.
//
// References to known swagger:response objects produce a `$ref: #/responses/<name>` directly on the
// Response; body refs produce a Schema with optional array wrapping.
// Untagged refs follow the definition-fallback rule: a name found in r.definitions but not in
// r.responses is silently promoted to a body ref.
//
// Unresolvable refs emit CodeInvalidAnnotation and the response is dropped.
func (r *Builder) dispatchResponses(p grammar.Property, op *oaispec.Operation) error {
	decls := routebody.ParseResponses(p.Body, p.Pos, r.RecordDiagnostic)
	if len(decls) == 0 {
		return nil
	}
	if op.Responses == nil {
		op.Responses = new(oaispec.Responses)
	}

	for i := range decls {
		decl := &decls[i]
		resp, ok := r.buildRouteResponse(decl)
		if !ok {
			continue
		}
		if strings.EqualFold(decl.Code, "default") {
			if op.Responses.Default == nil {
				cp := resp
				op.Responses.Default = &cp
			}
			continue
		}
		code, err := strconv.Atoi(decl.Code)
		if err != nil {
			r.RecordDiagnostic(grammar.Diagnostic{
				Pos:      decl.Pos,
				Severity: grammar.SeverityWarning,
				Code:     grammar.CodeInvalidAnnotation,
				Message:  "response code " + decl.Code + " is not a valid integer",
			})
			continue
		}
		if op.Responses.StatusCodeResponses == nil {
			op.Responses.StatusCodeResponses = make(map[int]oaispec.Response)
		}
		op.Responses.StatusCodeResponses[code] = resp
	}
	return nil
}

// defaultResponseDescription is the last-resort description for a `body:` response whose code has
// no standard HTTP reason phrase (the `default` catch-all, or a non-standard numeric code) and
// whose body type carries no godoc.
//
// OAS2 `default` covers any undeclared code — not necessarily an error — so the placeholder
// stays neutral rather than asserting "error".
// Capitalised to read as prose alongside the HTTP reason phrases ("Not Found", "Internal Server
// Error") it sits next to in a responses table.
const defaultResponseDescription = "Default response"

// bodyResponseDescription chooses a non-empty, human-meaningful description for a `body:`-form
// response that carries no trailing description text.
//
// OAS2 requires a non-empty description, but the bare Go type token (e.g. "Pet" or "string") leaks
// an implementation detail into the contract (doc-quirk G1).
// Preference, most to least specific:
//
//  1. the referenced model's own godoc (its Title, then Description) — this
//     mirrors how a named swagger:response derives its description from
//     prose, instead of echoing the type name;
//  2. the HTTP status reason phrase for a numeric code (200 → "OK",
//     404 → "Not Found");
//  3. a neutral placeholder for `default` / non-standard codes.
func (r *Builder) bodyResponseDescription(code, ref string) string {
	// The definitions map is keyed by the fully-qualified identity during build; the author wrote a
	// short model name, so resolve by leaf (a unique match only — an ambiguous one cannot pick a
	// godoc source).
	if key, ok, ambiguous := resolveDefinitionByLeaf(r.definitions, ref); ok && !ambiguous {
		def := r.definitions[key]
		if t := strings.TrimSpace(def.Title); t != "" {
			return t
		}
		if d := strings.TrimSpace(def.Description); d != "" {
			return d
		}
	}
	if n, err := strconv.Atoi(code); err == nil {
		if phrase := http.StatusText(n); phrase != "" {
			return phrase
		}
	}
	return defaultResponseDescription
}

// buildRouteResponse materialises one ResponseDecl into a spec.Response.
//
// Resolves the ref by consulting r.responses (named swagger:response objects) first, then
// r.definitions: untagged response names that happen to be model definitions are silently promoted
// to body refs.
// Unresolvable refs return (Response{}, false) with a CodeInvalidAnnotation diagnostic.
func (r *Builder) buildRouteResponse(decl *routebody.ResponseDecl) (oaispec.Response, bool) {
	switch {
	case decl.BodyTypeRef != "":
		schema := primitiveBodySchema(decl.BodyTypeRef, decl.Arrays)
		if schema == nil {
			if _, reserved := responseBodyReservedTypes[decl.BodyTypeRef]; reserved {
				r.RecordDiagnostic(grammar.Diagnostic{
					Pos:      decl.Pos,
					Severity: grammar.SeverityWarning,
					Code:     grammar.CodeInvalidAnnotation,
					Message: "response " + decl.Code + ": body:" + decl.BodyTypeRef + " — " +
						decl.BodyTypeRef + " is not a supported response body type; use a primitive " +
						"(string/number/integer/boolean), an array of those (body:[]string), or a model name",
				})
				return oaispec.Response{}, false
			}
			schema = r.resolveBodySchema(decl.BodyTypeRef, decl.Arrays)
		}
		if schema == nil {
			r.RecordDiagnostic(grammar.Diagnostic{
				Pos:      decl.Pos,
				Severity: grammar.SeverityWarning,
				Code:     grammar.CodeInvalidAnnotation,
				Message:  "response body ref " + decl.BodyTypeRef + " did not resolve",
			})
			return oaispec.Response{}, false
		}
		desc := decl.Description
		if desc == "" {
			desc = r.bodyResponseDescription(decl.Code, decl.BodyTypeRef)
		}
		return oaispec.Response{
			ResponseProps: oaispec.ResponseProps{
				Description: desc,
				Schema:      schema,
			},
		}, true

	case decl.ResponseRef != "":
		// Definition-fallback: if the ref name is NOT in r.responses but IS in r.definitions, silently
		// promote it to a body ref (intentional kindness for the common case where the author referenced
		// a model by name rather than a response).
		if _, ok := r.responses[decl.ResponseRef]; !ok {
			// The definitions map is keyed by the fully-qualified identity during build (pkgpath/name); the
			// author wrote a short model name, so resolve by leaf.
			// Exactly one match → promote; several → ambiguous cross-package collision the short name
			// cannot disambiguate (drop + diagnose, per name-identity D-8).
			//
			// See §12.1.
			_, found, ambiguous := resolveDefinitionByLeaf(r.definitions, decl.ResponseRef)
			switch {
			case ambiguous:
				r.RecordDiagnostic(grammar.Diagnostic{
					Pos:      decl.Pos,
					Severity: grammar.SeverityWarning,
					Code:     grammar.CodeInvalidAnnotation,
					Message: "response " + decl.Code + ": ref " + decl.ResponseRef +
						" is ambiguous — several models across packages share this name; " +
						"disambiguate with an explicit `swagger:model <name>`; dropped",
				})
				return oaispec.Response{}, false
			case found:
				schema := r.resolveBodySchema(decl.ResponseRef, decl.Arrays)
				desc := decl.Description
				if desc == "" {
					desc = r.bodyResponseDescription(decl.Code, decl.ResponseRef)
				}
				return oaispec.Response{
					ResponseProps: oaispec.ResponseProps{
						Description: desc,
						Schema:      schema,
					},
				}, true
			}
			// Dangling refs (not in responses, not in definitions) emit a diagnostic and are dropped rather
			// than silently emitting an invalid $ref.
			//
			// A bare primitive type spelling (`200: string`) is intentionally NOT promoted to a typed schema
			// — an untagged token is a response/model NAME, and reading it as a type would make the syntax
			// ambiguous.
			//
			// The unambiguous form is `body:<type>` (no Go type name carries a `:`), so point the author
			// there.
			msg := "response ref " + decl.ResponseRef + " not found in responses or definitions; dropped"
			if _, isPrim := responseBodyPrimitives[decl.ResponseRef]; isPrim {
				msg = "response " + decl.Code + ": " + decl.ResponseRef +
					" reads " + decl.ResponseRef + " as a response name, not a type; " +
					"for a primitive body write `" + decl.Code + ": body:" + decl.ResponseRef + "`"
			}
			r.RecordDiagnostic(grammar.Diagnostic{
				Pos:      decl.Pos,
				Severity: grammar.SeverityWarning,
				Code:     grammar.CodeInvalidAnnotation,
				Message:  msg,
			})
			return oaispec.Response{}, false
		}
		ref, err := oaispec.NewRef("#/responses/" + decl.ResponseRef)
		if err != nil {
			return oaispec.Response{}, false
		}
		return oaispec.Response{Refable: oaispec.Refable{Ref: ref}}, true

	default:
		// Description-only or empty-value response.
		// An empty description carries through as "" — callers see exactly what the author wrote, with
		// no implicit unset semantics.
		return oaispec.Response{
			ResponseProps: oaispec.ResponseProps{Description: decl.Description},
		}, true
	}
}
