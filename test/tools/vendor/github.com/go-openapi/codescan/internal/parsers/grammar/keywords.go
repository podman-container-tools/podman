// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"slices"
	"strings"
)

// ValueShape names the lexical shape of a keyword's value, mapping directly onto the value
// terminals:
//
//   - ShapeNumber       → NUMBER_VALUE
//   - ShapeInt          → INT_VALUE
//   - ShapeBool         → BOOL_VALUE
//   - ShapeString       → STRING_VALUE
//   - ShapeCommaList    → COMMA_LIST_VALUE
//   - ShapeEnumOption   → ENUM_OPTION_VALUE  (Values lists the closed set)
//   - ShapeRawBlock     → RAW_BLOCK_<KW>     (multi-line body terminal)
//   - ShapeRawValue     → RAW_VALUE_<KW>     (multi-line OR single-line body terminal)
//
// See README §keyword-table for the per-shape callback dispatched by Walker.
type ValueShape int

const (
	ShapeNone ValueShape = iota
	ShapeNumber
	ShapeInt
	ShapeBool
	ShapeString
	ShapeCommaList
	ShapeEnumOption
	ShapeRawBlock
	ShapeRawValue
)

// String renders a ValueShape for diagnostics.
func (v ValueShape) String() string {
	switch v {
	case ShapeNumber:
		return "number"
	case ShapeInt:
		return "integer"
	case ShapeBool:
		return "boolean"
	case ShapeString:
		return "string"
	case ShapeCommaList:
		return "comma-list"
	case ShapeEnumOption:
		return "enum-option"
	case ShapeRawBlock:
		return "raw-block"
	case ShapeRawValue:
		return "raw-value"
	case ShapeNone:
		fallthrough
	default:
		return "none"
	}
}

// IsBody reports whether the keyword's value is a multi-line body terminal (RAW_BLOCK_* or
// RAW_VALUE_*).
//
// Body keywords trigger the lexer's body accumulator.
func (v ValueShape) IsBody() bool {
	return v == ShapeRawBlock || v == ShapeRawValue
}

// Keyword describes one recognisable keyword: <value> form.
type Keyword struct {
	Name    string
	Aliases []string
	Shape   ValueShape
	Values  []string // populated when Shape == ShapeEnumOption
	// Contexts is the set of family contexts the keyword is legal in.
	//
	// Used by the parser layer for non-fatal context-invalid warnings.
	Contexts []KeywordContext
}

// KeywordContext is a family-level context where a keyword is legal.
//
// Distinct from AnnotationKind because validations are legal across several annotation kinds (model
// + parameters + response, etc.).
type KeywordContext int

const (
	CtxParam KeywordContext = iota
	CtxHeader
	CtxSchema
	CtxItems
	CtxRoute
	CtxOperation
	CtxMeta
	CtxResponse
)

// String renders a KeywordContext for diagnostics.
func (c KeywordContext) String() string {
	switch c {
	case CtxParam:
		return "param"
	case CtxHeader:
		return "header"
	case CtxSchema:
		return "schema"
	case CtxItems:
		return "items"
	case CtxRoute:
		return "route"
	case CtxOperation:
		return "operation"
	case CtxMeta:
		return "meta"
	case CtxResponse:
		return "response"
	default:
		return "?"
	}
}

// keyword constructs a Keyword via functional options.
func keyword(name string, opts ...keywordOpt) Keyword {
	kw := Keyword{Name: name}
	for _, o := range opts {
		o(&kw)
	}
	return kw
}

type keywordOpt func(*Keyword)

func aka(names ...string) keywordOpt {
	return func(kw *Keyword) { kw.Aliases = append(kw.Aliases, names...) }
}

func asNumber() keywordOpt   { return func(kw *Keyword) { kw.Shape = ShapeNumber } }
func asInt() keywordOpt      { return func(kw *Keyword) { kw.Shape = ShapeInt } }
func asBool() keywordOpt     { return func(kw *Keyword) { kw.Shape = ShapeBool } }
func asString() keywordOpt   { return func(kw *Keyword) { kw.Shape = ShapeString } }
func asRawBlock() keywordOpt { return func(kw *Keyword) { kw.Shape = ShapeRawBlock } }
func asRawValue() keywordOpt { return func(kw *Keyword) { kw.Shape = ShapeRawValue } }

func asEnumOption(values ...string) keywordOpt {
	return func(kw *Keyword) {
		kw.Shape = ShapeEnumOption
		kw.Values = values
	}
}

func ctx(ctxs ...KeywordContext) keywordOpt {
	return func(kw *Keyword) { kw.Contexts = append(kw.Contexts, ctxs...) }
}

// Canonical keyword names.
//
// These constants are the single source of truth for spelling: every Property's Keyword.Name
// compares equal to exactly one of them.
//
// Consumers that switch on Keyword.Name should reference these constants rather than re-declaring
// the strings — schema/walker.go and the bridge dispatchers in routes/parameters/
// responses/operations/items/spec all dispatch on these names.
//
// Sections (numeric validations / length validations / schema decorators / boolean markers /
// param-location / meta single-line / raw-block) follow the same order as the keywords table below.
const (
	KwMaximum              = "maximum"
	KwMinimum              = "minimum"
	KwMultipleOf           = "multipleOf"
	KwMaxLength            = "maxLength"
	KwMinLength            = "minLength"
	KwPattern              = "pattern"
	KwMaxItems             = "maxItems"
	KwMinItems             = "minItems"
	KwUnique               = "unique"
	KwCollectionFormat     = "collectionFormat"
	KwMaxProperties        = "maxProperties"
	KwMinProperties        = "minProperties"
	KwPatternProperties    = "patternProperties"
	KwAdditionalProperties = "additionalProperties"
	KwDefault              = "default"
	KwExample              = "example"
	KwExamples             = "examples"
	KwEnum                 = "enum"
	KwRequired             = "required"
	KwReadOnly             = "readOnly"
	KwDiscriminator        = "discriminator"
	KwDeprecated           = "deprecated"
	KwIn                   = "in"
	KwName                 = "name"
	KwSchemes              = "schemes"
	KwVersion              = "version"
	KwHost                 = "host"
	KwBasePath             = "basePath"
	KwLicense              = "license"
	KwContact              = "contact"
	KwConsumes             = "consumes"
	KwProduces             = "produces"
	KwSecurity             = "security"
	KwSecurityDefinitions  = "securityDefinitions"
	KwResponses            = "responses"
	KwParameters           = "parameters"
	KwExtensions           = "extensions"
	KwInfoExtensions       = "infoExtensions"
	KwTOS                  = "tos"
	KwExternalDocs         = "externalDocs"
	KwTags                 = "tags"
)

// keywords is the authoritative table. Additions land here.
//
//nolint:gochecknoglobals // canonical lookup table — see godoc on Lookup.
var keywords = []Keyword{
	// Numeric validations.
	keyword(KwMaximum, aka("max"), asNumber(), ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),
	keyword(KwMinimum, aka("min"), asNumber(), ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),
	keyword(KwMultipleOf,
		aka("multiple of", "multiple-of"),
		asNumber(),
		ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),

	// String-length validations.
	keyword(KwMaxLength,
		aka("max length", "max-length", "maxLen", "max len", "max-len",
			"maximum length", "maximum-length", "maximumLength",
			"maximum len", "maximum-len"),
		asInt(),
		ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),
	keyword(KwMinLength,
		aka("min length", "min-length", "minLen", "min len", "min-len",
			"minimum length", "minimum-length", "minimumLength",
			"minimum len", "minimum-len"),
		asInt(),
		ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),
	keyword(KwPattern,
		asString(),
		ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),

	// Array validations.
	keyword(KwMaxItems,
		aka("max items", "max-items", "max.items",
			"maximum items", "maximum-items", "maximumItems"),
		asInt(),
		ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),
	keyword(KwMinItems,
		aka("min items", "min-items", "min.items",
			"minimum items", "minimum-items", "minimumItems"),
		asInt(),
		ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),
	keyword(KwUnique,
		asBool(),
		ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),
	keyword(KwCollectionFormat,
		aka("collection format", "collection-format"),
		asEnumOption("csv", "ssv", "tsv", "pipes", "multi"),
		ctx(CtxParam, CtxHeader, CtxItems)),

	// Object validations.
	// Full-Schema-only (CtxSchema): object property-count bounds have no SimpleSchema
	// (param/header/items) equivalent in OAS v2. Shape-restricted to `object` schemas by
	// validations.keywordTypeRules.
	keyword(KwMaxProperties,
		aka("max properties", "max-properties",
			"maximum properties", "maximum-properties", "maximumProperties"),
		asInt(),
		ctx(CtxSchema)),
	keyword(KwMinProperties,
		aka("min properties", "min-properties",
			"minimum properties", "minimum-properties", "minimumProperties"),
		asInt(),
		ctx(CtxSchema)),
	keyword(KwPatternProperties,
		aka("pattern properties", "pattern-properties"),
		asString(),
		ctx(CtxSchema)),
	// additionalProperties: <spec> field keyword (true | false | a swagger:type-style spec).
	// Full-Schema-only; the schema builder resolves the spec and applies the lowest-priority
	// precedence.
	// See schema/additional_properties.go.
	keyword(KwAdditionalProperties,
		aka("additional properties", "additional-properties"),
		asString(),
		ctx(CtxSchema)),

	// Schema decorators with body-accepting bodies (default/example/enum).
	keyword(KwDefault,
		asRawValue(),
		ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),
	keyword(KwExample,
		asRawValue(),
		ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),
	keyword(KwEnum,
		asRawValue(),
		ctx(CtxParam, CtxHeader, CtxSchema, CtxItems)),

	// Field-level boolean markers.
	keyword(KwRequired, asBool(), ctx(CtxParam, CtxSchema)),
	keyword(KwReadOnly,
		aka("read only", "read-only"),
		asBool(),
		ctx(CtxSchema)),
	keyword(KwDiscriminator, asBool(), ctx(CtxSchema)),
	keyword(KwDeprecated, asBool(), ctx(CtxOperation, CtxRoute, CtxSchema)),

	// Parameter-location directive.
	// Not part of the formal schema- body grammar.
	// Recognised here so the lexer can hand a typed token to the parameters dispatch path; the schema
	// parser treats it as a context-invalid warning when seen outside that path.
	//
	// See README §keyword-table ("`in:` is a parameter-location directive").
	keyword(KwIn,
		asEnumOption("query", "path", "header", "body", "formData"),
		ctx(CtxParam)),

	// Name directive — the canonical field-naming keyword.
	//
	// Like `in:`, this is a structural keyword rather than part of the schema-body grammar: it renames
	// the published name of a field at every field site — a swagger:parameters field (the JSON
	// parameter name), a swagger:response field (the response header / Headers map key) and a
	// swagger:model property or interface method (the JSON property name) — always overriding the
	// json-tag / Go-field derivation.
	//
	// Legal in CtxParam, CtxHeader and CtxSchema so it is removed from the description prose and
	// silently ignored by the (Simple)Schema walker (isFullSchemaOnly stays false; no shape rule, so
	// checkShape passes) at all three; the parameters / responses / schema builders read the value via
	// GetString(KwName).
	//
	// The older `swagger:name` annotation remains honoured as a legacy form (and is the only form on
	// interface methods historically), but `name:` takes precedence when both appear.
	// See README §keyword-table. (doc-quirk G2.)
	keyword(KwName, asString(), ctx(CtxParam, CtxHeader, CtxSchema)),

	// List-shaped keywords.
	// KwSchemes uses asRawBlock() so multi-line bodies (`Schemes:\n - http\n - https`) populate the
	// same way they do for Consumes/Produces; the inline comma-list form (`Schemes: http, https`)
	// still works via the inline-value capture in collectRawBlock.
	//
	// Block.GetList unifies both surfaces.
	// See README §keyword-table.
	keyword(KwSchemes,
		asRawBlock(),
		ctx(CtxMeta, CtxRoute, CtxOperation)),
	keyword(KwVersion, asString(), ctx(CtxMeta)),
	keyword(KwHost, asString(), ctx(CtxMeta)),
	keyword(KwBasePath,
		aka("base path", "base-path"),
		asString(),
		ctx(CtxMeta)),
	keyword(KwLicense, asString(), ctx(CtxMeta)),
	keyword(KwContact,
		aka("contact info", "contact-info"),
		asString(),
		ctx(CtxMeta)),

	// Multi-line raw-block keywords.
	keyword(KwConsumes, asRawBlock(), ctx(CtxMeta, CtxRoute, CtxOperation)),
	keyword(KwProduces, asRawBlock(), ctx(CtxMeta, CtxRoute, CtxOperation)),
	keyword(KwSecurity, asRawBlock(), ctx(CtxMeta, CtxRoute, CtxOperation)),
	keyword(KwSecurityDefinitions,
		aka("security definitions", "security-definitions"),
		asRawBlock(),
		ctx(CtxMeta)),
	keyword(KwResponses, asRawBlock(), ctx(CtxRoute, CtxOperation)),
	keyword(KwParameters, asRawBlock(), ctx(CtxRoute, CtxOperation)),
	// `examples:` is a YAML map keyed by mime type (`examples:` then an indented `application/json:
	// {…}`) populating spec.Response.examples on a `swagger:response`.
	// OAS2 reserves the PLURAL `examples` for responses; the singular `example` above is the schema /
	// param / header / items decorator.
	//
	// Only the struct- based `swagger:response` path needs this keyword — the `swagger:operation`
	// YAML path carries examples for free via the spec unmarshal.
	// The body is YAML-parsed downstream, so it joins the yamlBody set in the lexer.
	// See features/response-examples-by-mime.
	keyword(KwExamples, asRawBlock(), ctx(CtxResponse)),
	keyword(KwExtensions,
		asRawBlock(),
		ctx(CtxMeta, CtxRoute, CtxOperation, CtxSchema, CtxParam, CtxHeader)),
	keyword(KwInfoExtensions,
		aka("info extensions", "info-extensions"),
		asRawBlock(),
		ctx(CtxMeta)),
	keyword(KwTOS,
		aka("terms of service", "terms-of-service", "termsOfService"),
		asRawBlock(),
		ctx(CtxMeta)),
	keyword(KwExternalDocs,
		aka("external docs", "external-docs"),
		asRawBlock(),
		ctx(CtxMeta, CtxRoute, CtxOperation, CtxSchema)),
	// `Tags:` block.
	// On swagger:meta it is a YAML list of tag objects ({name, description, externalDocs}) populating
	// spec.Swagger.Tags.
	//
	// On swagger:route/operation it is a plain string list of tag names, unioned onto the operation's
	// tags (the same names may also appear on the swagger:route header line).
	// The single keyword carries two shapes; the consuming builder decides which (the meta walker
	// unmarshals objects, the route walker reads AsList).
	//
	// See go-swagger/go-swagger#2655.
	keyword(KwTags, asRawBlock(), ctx(CtxMeta, CtxRoute, CtxOperation)),
}

// specPointer records where a keyword's value renders in the produced spec: the JSON-pointer
// segments (relative to the node the keyword decorates, or the document root for the meta-only
// keywords) and, optionally, the contexts where that node is actually produced when they differ
// from the keyword's lexical Contexts.
type specPointer struct {
	segs []string
	// ctxs overrides the render contexts; nil means "use the keyword's Contexts".
	//
	// Set only where lexical legality and node production diverge — e.g. `deprecated` is legal on a
	// schema but only renders a node on operations.
	ctxs []KeywordContext
}

// specPointers is the single source of truth mapping each keyword that produces
// an addressable spec node to its pointer segments. It keeps the keyword→spec-
// location knowledge in the grammar (beside Name/Shape/Contexts) instead of
// re-encoded in every builder walker; cross-ref provenance reads it via
// [PointerPath]. Keywords absent here produce no directly-addressable node
// (required → parent array, in/collectionFormat → simple-schema, prose, …) and
// resolve to their enclosing node's anchor.
//
//nolint:gochecknoglobals // canonical lookup table, mirrors keywords above
var specPointers = map[string]specPointer{
	// Schema / param / header / items validations (segment == keyword name).
	KwMaximum:    {segs: []string{KwMaximum}},
	KwMinimum:    {segs: []string{KwMinimum}},
	KwMultipleOf: {segs: []string{KwMultipleOf}},
	KwMaxLength:  {segs: []string{KwMaxLength}},
	KwMinLength:  {segs: []string{KwMinLength}},
	KwPattern:    {segs: []string{KwPattern}},
	KwMaxItems:   {segs: []string{KwMaxItems}},
	KwMinItems:   {segs: []string{KwMinItems}},
	KwUnique:     {segs: []string{"uniqueItems"}}, // the lone name≠segment case
	KwDefault:    {segs: []string{KwDefault}},
	KwExample:    {segs: []string{KwExample}},
	KwEnum:       {segs: []string{KwEnum}},
	KwReadOnly:   {segs: []string{KwReadOnly}},
	// Operation / route header keywords (child of the operation node).
	KwConsumes:   {segs: []string{KwConsumes}},
	KwProduces:   {segs: []string{KwProduces}},
	KwSchemes:    {segs: []string{KwSchemes}},
	KwDeprecated: {segs: []string{KwDeprecated}, ctxs: []KeywordContext{CtxOperation, CtxRoute}},
	// Meta keywords.
	// Info.* nest under /info; the rest land at the document root.
	KwVersion:  {segs: []string{"info", KwVersion}},
	KwTOS:      {segs: []string{"info", "termsOfService"}},
	KwContact:  {segs: []string{"info", KwContact}},
	KwLicense:  {segs: []string{"info", KwLicense}},
	KwHost:     {segs: []string{KwHost}},
	KwBasePath: {segs: []string{KwBasePath}},
}

// PointerPath returns the JSON-pointer segments where kw's value renders, when kw produces an
// addressable node legal in ctx.
//
// Consumers (the builder walkers) prepend the decorated node's base pointer and any items depth.
// The ctx gate reuses the keyword's lexical Contexts unless the entry overrides them, so a keyword
// is never anchored in a context where its node does not exist.
func PointerPath(kw Keyword, ctx KeywordContext) ([]string, bool) {
	sp, ok := specPointers[kw.Name]
	if !ok {
		return nil, false
	}
	ctxs := sp.ctxs
	if ctxs == nil {
		ctxs = kw.Contexts
	}
	if slices.Contains(ctxs, ctx) {
		return sp.segs, true
	}
	return nil, false
}

// Lookup returns the Keyword matching name (canonical or alias), case-insensitive on the
// canonical/alias spellings.
//
// The second return value reports whether a match was found.
func Lookup(name string) (Keyword, bool) {
	needle := strings.ToLower(strings.TrimSpace(name))
	if needle == "" {
		return Keyword{}, false
	}
	for _, kw := range keywords {
		if strings.EqualFold(kw.Name, needle) {
			return kw, true
		}
		for _, alias := range kw.Aliases {
			if strings.EqualFold(alias, needle) {
				return kw, true
			}
		}
	}
	return Keyword{}, false
}

// Keywords returns a copy of the authoritative table.
func Keywords() []Keyword {
	out := make([]Keyword, len(keywords))
	copy(out, keywords)
	return out
}
