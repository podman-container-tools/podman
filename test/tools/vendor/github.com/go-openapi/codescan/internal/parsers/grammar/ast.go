// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"go/token"
	"iter"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers/security"
)

// Block is the interface every typed AST node implements. One Block
// corresponds to one Go comment group's parsed content.
//
// The typed Block hierarchy matches the family productions:
//
//   - SchemaBlock variants:    ModelBlock, ResponseBlock, ParametersBlock, NameBlock
//   - OperationFamilyBlock:    RouteBlock, InlineOperationBlock
//   - MetaBlock
//   - ClassifierBlock variants for the classifier annotations
//   - UnboundBlock for the no-annotation case
//
// # Details
//
// See README §parser-contract and §block-shapes for the
// per-Block contracts.
//
//nolint:interfacebloat // single consumer contract for builders + LSP.
type Block interface {
	Pos() token.Position
	Title() string
	Description() string
	Diagnostics() []Diagnostic
	AnnotationKind() AnnotationKind

	Properties() iter.Seq[Property]
	YAMLBlocks() iter.Seq[RawYAML]
	Extensions() iter.Seq[Extension]
	SecurityRequirements() []security.Requirement
	Contact() (Contact, error)
	License() (License, bool)

	// IsDeprecated reports whether the block marks its subject deprecated — via the explicit
	// `deprecated: true` keyword or a godoc-style "Deprecated:" paragraph.
	//
	// See deprecated.go.
	IsDeprecated() bool

	// Walk dispatches properties / prose / extensions / diagnostics through the callbacks set on w.
	// See walker.go for the contract.
	Walk(w Walker)

	// ProseLines returns the cleaned prose lines (TITLE + DESC) in source order, with blank lines
	// preserved as empty strings.
	ProseLines() []string

	// PreambleLines returns prose lines that appear BEFORE the block's annotation.
	//
	// For UnboundBlock (no annotation), returns the same as ProseLines.
	// Schema and meta builders consume only pre-annotation prose for title/description;
	// routes/operations consult ProseLines() and observe post-annotation text too.
	PreambleLines() []string

	// PreambleTitle / PreambleDescription return the title/description computed from PreambleLines
	// (pre-annotation prose only).
	//
	// Schema's top-level model builder uses these so post-annotation prose reads as body content
	// rather than title/description.
	PreambleTitle() string
	PreambleDescription() string

	// Prose returns the entire prose surface (TITLE + DESC tokens in source order) joined with "\n",
	// with internal blanks preserved as paragraph breaks and a single trailing blank dropped.
	//
	// Used by field-level callers (struct field / interface method docs) where the entire prose is the
	// description and there's no separate title concept.
	Prose() string

	Has(name string) bool
	GetFloat(name string) (float64, bool)
	GetInt(name string) (int64, bool)
	GetBool(name string) (bool, bool)
	GetString(name string) (string, bool)
	GetList(name string) ([]string, bool)

	// AnnotationArg returns the first positional identifier argument of the block's primary annotation
	// (e.g. "Pet" for `swagger:model Pet`, "date-time" for `swagger:strfmt date-time`, the IDENT_NAME
	// for `swagger:name fooBar`).
	//
	// Returns ("", false) when the annotation has no arg or carries only an empty/whitespace one.
	// Bare annotations (e.g. `swagger:model` without a name) return ("", false); callers distinguish
	// "annotation present" via AnnotationKind().
	//
	// Convergence accessor — replaces type-asserting on each typed Block kind to read its Name /
	// Args[0] field.
	// Used by Walker callbacks that don't care which classifier flavour they're looking at, only what
	// its IDENT_NAME-style argument is.
	//
	// See README §block-shapes.
	AnnotationArg() (string, bool)
}

// Property is one keyword:value (or keyword body) attached to a Block.
//
// For inline-value keywords (Number / Integer / Bool / String / EnumOption / CommaList shapes),
// Value is the raw string and Typed carries the lexically-typed form.
//
// For body keywords (ShapeRawBlock / ShapeRawValue), Body holds the accumulated body content, Raw
// holds the verbatim source content, and Typed.Type indicates the body shape.
//
// ItemsDepth records the leading items.* depth from the keyword head.
//
// # Details
//
// See README §property-shape for the field-population matrix and the AsList unification rule.
type Property struct {
	Keyword    Keyword
	Pos        token.Position
	Value      string
	Body       string
	Raw        string
	Typed      TypedValue
	ItemsDepth int
}

// IsTyped reports whether the property carries a primitive-typed value the caller can consume
// directly without further coercion.
//
// True when Typed.Type is one of the primitive shapes — Number, Integer, Bool, EnumOption —
// meaning the matching Typed.* field is populated and authoritative.
// False otherwise:
//
//   - ShapeNone — typing was not applied (ShapeString keywords like
//     `pattern` keep the raw value in Property.Value), or typing
//     failed and a diagnostic was emitted.
//   - ShapeRawBlock / ShapeRawValue / ShapeCommaList / ShapeString —
//     the value's interpretation depends on the resolved spec type
//     (e.g. `default: 1.5` against a float-typed schema), so the
//     consumer must coerce against schemaType+schemaFormat.
//
// Canonical use site:
//
//	if p.IsTyped() {
//	    // read p.Typed.<field> matching p.Keyword.Shape
//	} else {
//	    // coerce p.Value against the resolved schema type
//	}
//
// Replaces the explicit `switch p.Typed.Type` boilerplate consumers would otherwise need at every
// call site.
func (p Property) IsTyped() bool {
	switch p.Typed.Type {
	case ShapeNumber, ShapeInt, ShapeBool, ShapeEnumOption:
		return true
	case ShapeNone, ShapeString, ShapeCommaList, ShapeRawBlock, ShapeRawValue:
		return false
	default:
		return false
	}
}

// TypedValue carries the lexer-recognised value shape.
//
// Op is the leading comparison operator stripped from a NumberValue ("<", "<=", ">", ">=", "=").
// Empty for non-Number values or when no operator was present.
// Accepts e.g. `maximum: <5`; the analyzer interprets Op + Number to decide inclusive vs exclusive
// semantics.
type TypedValue struct {
	Type    ValueShape
	Op      string
	Number  float64
	Integer int64
	Boolean bool
	String  string
}

// RawYAML is one captured `--- … ---` body.
//
// The parser does not parse the YAML — it isolates the body so the analyzer can hand it to
// internal/parsers/yaml/.
type RawYAML struct {
	Pos       token.Position
	Text      string
	Truncated bool
}

// Extension is one x-* vendor entry under an extensions: block.
//
// Value carries the YAML-typed nested value (`bool` / `float64` / `string` / `[]any` /
// `map[string]any`).
// The parser pipes the body through `internal/parsers/yaml.TypedExtensions`, so consumers can rely
// on JSON-normalised types — never `map[any]any`.
//
// Source carries the keyword that produced the entry: KwExtensions for `extensions:` blocks
// (top-level vendor extensions) or KwInfoExtensions for `infoExtensions:` blocks (Info-scoped
// vendor extensions, meta-only).
//
// Consumers that need to route entries to different targets — meta's swspec.Extensions vs
// swspec.Info.Extensions — switch on this field; consumers that treat extensions uniformly
// (routes / operations) can ignore it.
type Extension struct {
	Name   string
	Source string
	Pos    token.Position
	Value  any
}

// baseBlock provides the fields and accessors common to every typed Block.
//
// Typed kinds embed *baseBlock and add per-annotation fields.
type baseBlock struct {
	pos                 token.Position
	title               string
	description         string
	proseLines          []string
	preambleLines       []string
	preambleTitle       string
	preambleDescription string
	kind                AnnotationKind

	properties  []Property
	yamlBlocks  []RawYAML
	extensions  []Extension
	security    []security.Requirement
	diagnostics []Diagnostic
}

func (b *baseBlock) Pos() token.Position         { return b.pos }
func (b *baseBlock) Title() string               { return b.title }
func (b *baseBlock) Description() string         { return b.description }
func (b *baseBlock) ProseLines() []string        { return b.proseLines }
func (b *baseBlock) PreambleLines() []string     { return b.preambleLines }
func (b *baseBlock) PreambleTitle() string       { return b.preambleTitle }
func (b *baseBlock) PreambleDescription() string { return b.preambleDescription }

// Prose joins the cached proseLines (TITLE + DESC + internal blanks, in source order) with "\n" and
// drops a single whitespace-only trailing line.
//
// The trim and join happen on the cached slice; no extra parse work.
func (b *baseBlock) Prose() string {
	lines := b.proseLines
	if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == "" {
		lines = lines[:n-1]
	}
	return strings.Join(lines, "\n")
}
func (b *baseBlock) Diagnostics() []Diagnostic      { return b.diagnostics }
func (b *baseBlock) AnnotationKind() AnnotationKind { return b.kind }

// AnnotationArg default — Block kinds whose annotation takes no identifier argument
// (UnboundBlock, MetaBlock, RouteBlock, InlineOperationBlock, ParametersBlock — the latter has
// multiple args but conceptually it's a list, not a single IDENT_NAME).
//
// Typed kinds with a single IDENT_NAME override this method.
func (b *baseBlock) AnnotationArg() (string, bool) { return "", false }

func (b *baseBlock) Properties() iter.Seq[Property] {
	return func(yield func(Property) bool) {
		for _, p := range b.properties {
			if !yield(p) {
				return
			}
		}
	}
}

func (b *baseBlock) YAMLBlocks() iter.Seq[RawYAML] {
	return func(yield func(RawYAML) bool) {
		for _, y := range b.yamlBlocks {
			if !yield(y) {
				return
			}
		}
	}
}

func (b *baseBlock) Extensions() iter.Seq[Extension] {
	return func(yield func(Extension) bool) {
		for _, e := range b.extensions {
			if !yield(e) {
				return
			}
		}
	}
}

// SecurityRequirements returns the typed list of `Security:` requirements parsed at lex time from
// the block's `security:` raw body, or nil when no `security:` keyword appeared.
//
// Each entry is a single-key map from scheme name → scope list, mirroring the shape OAS v2
// expects on `spec.Operation.Security`.
func (b *baseBlock) SecurityRequirements() []security.Requirement {
	return b.security
}

// Contact returns the typed Contact value parsed from the block's `contact:` inline keyword.
//
// Returns (Contact{}, nil) when no `contact:` appeared, or when its value is empty.
// A non-nil error signals a malformed `Name <email>` head — the caller decides whether to fail or
// warn.
// Parses on call.
func (b *baseBlock) Contact() (Contact, error) {
	for _, p := range b.properties {
		if p.Keyword.Name == KwContact {
			return parseContact(p.Value)
		}
	}
	return Contact{}, nil
}

// License returns the typed License value parsed from the block's `license:` inline keyword, or
// (License{}, false) when no `license:` keyword appeared.
//
// Parses on call.
func (b *baseBlock) License() (License, bool) {
	for _, p := range b.properties {
		if p.Keyword.Name == KwLicense {
			return parseLicense(p.Value)
		}
	}
	return License{}, false
}

func (b *baseBlock) Has(name string) bool {
	_, ok := b.findProperty(name)
	return ok
}

func (b *baseBlock) GetFloat(name string) (float64, bool) {
	p, ok := b.findProperty(name)
	if !ok || p.Typed.Type != ShapeNumber {
		return 0, false
	}
	return p.Typed.Number, true
}

func (b *baseBlock) GetInt(name string) (int64, bool) {
	p, ok := b.findProperty(name)
	if !ok || p.Typed.Type != ShapeInt {
		return 0, false
	}
	return p.Typed.Integer, true
}

func (b *baseBlock) GetBool(name string) (bool, bool) {
	p, ok := b.findProperty(name)
	if !ok || p.Typed.Type != ShapeBool {
		return false, false
	}
	return p.Typed.Boolean, true
}

func (b *baseBlock) GetString(name string) (string, bool) {
	p, ok := b.findProperty(name)
	if !ok {
		return "", false
	}
	if p.Typed.Type == ShapeEnumOption {
		return p.Typed.String, true
	}
	return p.Value, true
}

// GetList returns the token list represented by the named keyword, unifying every list-shaped
// surface form the grammar accepts.
//
// Delegates to Property.AsList; see that method for the algorithm and accepted surface forms.
//
// Returns (nil, false) when the keyword is absent.
// Returns (nil, true) when the keyword is present but every line trims to empty — the bool
// reports presence, not non-emptiness.
func (b *baseBlock) GetList(name string) ([]string, bool) {
	p, ok := b.findProperty(name)
	if !ok {
		return nil, false
	}
	return p.AsList(), true
}

// AsList returns the token list represented by p, unifying every list-shaped surface form the
// grammar accepts (inline comma list, multi-line indented, YAML `- ` markers, or any combination).
//
// # Details
//
// See README §property-shape (`AsList — unified list extraction`) for the surface forms, the
// algorithm, and the explicit non-targets (enum values, routebody Parameters chunks, raw YAML
// bodies).
func (p Property) AsList() []string {
	var lines []string
	if p.Value != "" {
		lines = append(lines, p.Value)
	}
	if p.Body != "" {
		lines = append(lines, strings.Split(p.Body, "\n")...)
	}
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for tok := range strings.SplitSeq(line, ",") {
			if tok = strings.TrimSpace(tok); tok != "" {
				out = append(out, tok)
			}
		}
	}
	return out
}

func (b *baseBlock) findProperty(name string) (Property, bool) {
	for _, p := range b.properties {
		if strings.EqualFold(p.Keyword.Name, name) {
			return p, true
		}
		for _, alias := range p.Keyword.Aliases {
			if strings.EqualFold(alias, name) {
				return p, true
			}
		}
	}
	return Property{}, false
}

// --- Typed Block kinds -------------------------------------------------------

// ModelBlock is produced by `swagger:model [Name]`.
type ModelBlock struct {
	*baseBlock

	Name string
}

// AnnotationArg returns the IDENT_NAME arg (`Pet` for `swagger:model Pet`) or ("", false) for a
// bare `swagger:model`.
func (b *ModelBlock) AnnotationArg() (string, bool) {
	if b.Name == "" {
		return "", false
	}
	return b.Name, true
}

// ResponseBlock is produced by `swagger:response [Name]`.
type ResponseBlock struct {
	*baseBlock

	Name string
}

// AnnotationArg returns the IDENT_NAME arg or ("", false) for a bare `swagger:response`.
func (b *ResponseBlock) AnnotationArg() (string, bool) {
	if b.Name == "" {
		return "", false
	}
	return b.Name, true
}

// NameBlock is produced by `swagger:name <member-name>`.
//
// The annotation overrides a struct field's / interface method's JSON property name (or schema
// member name) without mutating the surrounding Go identifier.
// Name carries the IDENT_NAME argument.
type NameBlock struct {
	*baseBlock

	Name string
}

// AnnotationArg returns the IDENT_NAME arg.
//
// Bare `swagger:name` is rejected at parse time (CodeMissingRequiredArg); ("", false) only appears
// on the diagnostic path.
func (b *NameBlock) AnnotationArg() (string, bool) {
	if b.Name == "" {
		return "", false
	}
	return b.Name, true
}

// ParametersTarget classifies the first argument token of a `swagger:parameters` marker.
type ParametersTarget int

const (
	// ParamTargetOperations: the first token is an operation id.
	//
	// Covers the historical inline form (`swagger:parameters listPets …`) and the standalone
	// reference form (`swagger:parameters listPets X-Name …`); the two are disambiguated by the
	// builder using the host declaration (struct ⇒ definition/inline, otherwise reference).
	ParamTargetOperations ParametersTarget = iota
	// ParamTargetShared: the first token is the `*` shared-namespace marker (register the fields at
	// `#/parameters/{name}`).
	ParamTargetShared
	// ParamTargetPath: the first token is a `/path` (path-item parameters).
	ParamTargetPath
)

// ParametersBlock is produced by `swagger:parameters <target> [args…]`.
//
// The target is the shared-namespace wildcard `*`, a `/path`, or an operation id (see
// [ParametersTarget]).
// Args carries the remaining argument tokens, de-duplicated; the definition-vs-reference reading of
// those tokens is the builder's, since it depends on whether the marker is attached to a struct.
//
// §1.
type ParametersBlock struct {
	*baseBlock

	// Target is the kind of the first argument token.
	Target ParametersTarget
	// Path is the `/path` value when Target == ParamTargetPath.
	Path string
	// Args are the de-duplicated meaningful argument tokens in source order:
	//   - ParamTargetOperations: the operation ids (first token included)
	//   - ParamTargetShared:     the operation ids to reference into
	//   - ParamTargetPath:       the shared-parameter names to reference
	Args []string
	// Dups are argument tokens dropped as exact duplicates of an earlier one (the basis for a
	// duplicate-target / duplicate-ref warning).
	Dups []string
}

// OperationIDs returns the operation ids this block targets for inline application: Args when
// Target == ParamTargetOperations, else nil.
//
// The shared (`*`) and path (`/path`) targets are not operation ids.
func (b *ParametersBlock) OperationIDs() []string {
	if b.Target != ParamTargetOperations {
		return nil
	}
	return b.Args
}

// RouteBlock is produced by `swagger:route METHOD /path [tags] opID`. swagger:route's body is
// inline keyword raw-blocks and **never** an OPAQUE_YAML.
type RouteBlock struct {
	*baseBlock

	Method string
	Path   string
	Tags   []string
	OpID   string
}

// InlineOperationBlock is produced by `swagger:operation METHOD /path [tags] opID`.
//
// Body may carry an OPAQUE_YAML in addition to inline keyword raw-blocks.
type InlineOperationBlock struct {
	*baseBlock

	Method string
	Path   string
	Tags   []string
	OpID   string
}

// MetaBlock is produced by `swagger:meta`.
type MetaBlock struct {
	*baseBlock
}

// ClassifierBlock covers single-line classifier annotations: strfmt / allOf / ignore / alias / file
// / type / default.
//
// It also carries the schema-family override annotations swagger:title / swagger:description, whose
// free-text arg lives in Args[0] while any co-located validation keywords ride the embedded
// baseBlock.properties (those dispatch through the schema parser, not the classifier parser — see
// annotationFamily).
//
// The Args slice carries the lexer-tokenised positional arguments.
type ClassifierBlock struct {
	*baseBlock

	Args []Token
}

// AnnotationArg returns the first positional argument's text when non-empty (single-word capture).
//
// Bare classifier annotations (e.g. `swagger:ignore`, `swagger:alias`) return ("", false).
func (b *ClassifierBlock) AnnotationArg() (string, bool) {
	if len(b.Args) == 0 {
		return "", false
	}
	if b.Args[0].Text == "" {
		return "", false
	}
	return b.Args[0].Text, true
}

// EnumDeclBlock is produced by `swagger:enum [name] [values…]`.
//
// The optional multi-line value-list is carried in BodyValues when present (lexer accumulates the
// body as RAW_VALUE_ENUM).
type EnumDeclBlock struct {
	*baseBlock

	Name       string // "" when absent
	InlineForm enumArgsForm
	InlineArgs []Token // values fragment as a typed token, when inline values were given
	BodyValues string  // populated when a multi-line RAW_VALUE_ENUM body was present
}

// AnnotationArg returns the IDENT_NAME arg (`status` for `swagger:enum status`) or ("", false) when
// the enum was declared inline-only (`swagger:enum red green blue`) or as a pure body block.
func (b *EnumDeclBlock) AnnotationArg() (string, bool) {
	if b.Name == "" {
		return "", false
	}
	return b.Name, true
}

// UnboundBlock represents a comment group with no annotation line — e.g. a struct field's
// docstring.
type UnboundBlock struct {
	*baseBlock
}

// newBaseBlock initialises a baseBlock for the given annotation kind.
func newBaseBlock(kind AnnotationKind, pos token.Position) *baseBlock {
	return &baseBlock{pos: pos, kind: kind}
}
