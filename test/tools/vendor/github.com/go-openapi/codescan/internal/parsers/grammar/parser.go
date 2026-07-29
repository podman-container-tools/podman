// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers/security"
	"github.com/go-openapi/codescan/internal/parsers/yaml"
)

// Parser is the consumer contract for the grammar parser.
//
// The package ships *DefaultParser; the interface exists so tests can substitute a mock that
// fabricates Block values without running the full lex pipeline.
//
// # Details
//
// See README §parser-contract for the family dispatch table and the body-token consumption rules.
type Parser interface {
	Parse(cg *ast.CommentGroup) Block
	ParseAll(cg *ast.CommentGroup) []Block
	ParseText(text string, pos token.Position) Block
	ParseAs(kind AnnotationKind, text string, pos token.Position) Block
}

// DefaultParser is the concrete Parser implementation.
type DefaultParser struct {
	fset             *token.FileSet
	sink             func(Diagnostic)
	singleLineAsDesc bool
}

// NewParser constructs a DefaultParser bound to a FileSet (needed to map *ast.CommentGroup
// positions to absolute source positions).
func NewParser(fset *token.FileSet, opts ...Option) *DefaultParser {
	p := &DefaultParser{fset: fset}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Option configures a DefaultParser.
type Option func(*DefaultParser)

// WithDiagnosticSink streams diagnostics to a callback in addition to accumulating them on the
// returned Block.
func WithDiagnosticSink(sink func(Diagnostic)) Option {
	return func(p *DefaultParser) { p.sink = sink }
}

// WithSingleLineCommentAsDescription makes a single-line prose comment resolve to the block
// Description instead of the Title, regardless of trailing punctuation.
//
// Multi-line comments keep the normal title/description split.
// See go-swagger/go-swagger#2626.
func WithSingleLineCommentAsDescription(on bool) Option {
	return func(p *DefaultParser) { p.singleLineAsDesc = on }
}

//nolint:ireturn // stable seam.
func (p *DefaultParser) Parse(cg *ast.CommentGroup) Block {
	lines := Preprocess(cg, p.fset)
	tokens := Lex(lines)
	return p.parseTokens(tokens)
}

// ParseAll returns one Block per annotation in cg, in source order.
//
// A comment group with no annotation yields a single-element slice holding an UnboundBlock; a
// single-annotation comment yields the same Block as Parse, wrapped in a slice; multi-annotation
// comments yield one Block per annotation.
//
// Token partition: each annotation owns the slice of tokens from its index up to (but excluding)
// the next annotation.
//
// The first annotation also owns the pre-annotation prose, so its PreambleTitle /
// PreambleDescription match Parse(cg)'s. Body tokens between annotations attach to the *preceding*
// annotation.
//
// Multi-annotation comments like.
//
//	// swagger:model
//	// swagger:strfmt date-time
//
// pair a schema-family annotation with a classifier; the schema builder's Walker dispatches on each
// Block's AnnotationKind() without further partitioning.
func (p *DefaultParser) ParseAll(cg *ast.CommentGroup) []Block {
	lines := Preprocess(cg, p.fset)
	tokens := Lex(lines)
	return p.parseAllTokens(tokens)
}

//nolint:ireturn // stable seam.
func (p *DefaultParser) ParseText(text string, pos token.Position) Block {
	lines := preprocessText(text, pos)
	return p.parseTokens(Lex(lines))
}

//nolint:ireturn // stable seam.
func (p *DefaultParser) ParseAs(kind AnnotationKind, text string, pos token.Position) Block {
	injected := AnnotationPrefix + kind.String() + "\n" + text
	return p.ParseText(injected, pos)
}

// Parse is the convenience wrapper around NewParser(fset).Parse(cg).
//
//nolint:ireturn // stable seam.
func Parse(cg *ast.CommentGroup, fset *token.FileSet) Block {
	return NewParser(fset).Parse(cg)
}

// ParseAll is the convenience wrapper around NewParser(fset).ParseAll(cg).
func ParseAll(cg *ast.CommentGroup, fset *token.FileSet) []Block {
	return NewParser(fset).ParseAll(cg)
}

// ParseTokens runs the parser on a pre-lexed token stream. Useful for
// tests and LSP scenarios.
//
//nolint:ireturn // stable seam.
func ParseTokens(tokens []Token) Block {
	p := &DefaultParser{}
	return p.parseTokens(tokens)
}

// parseAllTokens implements the multi-annotation slicing rule documented on ParseAll.
func (p *DefaultParser) parseAllTokens(tokens []Token) []Block {
	var annIndices []int
	for i, t := range tokens {
		if t.Kind == TokenAnnotation {
			annIndices = append(annIndices, i)
		}
	}
	if len(annIndices) <= 1 {
		// Zero annotations → UnboundBlock; one annotation → equivalent to Parse.
		// Either way, the existing single- block path is correct — wrap in a slice.
		return []Block{p.parseTokens(tokens)}
	}
	out := make([]Block, 0, len(annIndices))
	start := 0
	for i := range annIndices {
		end := len(tokens)
		if i+1 < len(annIndices) {
			end = annIndices[i+1]
		}
		out = append(out, p.parseTokens(tokens[start:end]))
		start = end
	}
	return out
}

// parseState holds per-block parsing state.
//
// Today's parsers walk s.tokens via range loops because the token classifier serialises the body
// — order between annotation header and body items is flat, so a cursor adds no value.
//
// The `pos` field, `peek`, and `advance` below are scaffolding for future order-sensitive
// productions (strict positional checks on EnumDeclBlock's annotation header → RAW_VALUE_ENUM
// body, or LSP partial-parse resumption from a cursor).
// See README §parser-contract.
//
// To find every unused-on-purpose site:
//
//	golangci-lint run --enable-only unused ./internal/parsers/grammar/...
type parseState struct {
	tokens           []Token
	pos              int //nolint:unused // see godoc — reserved for recursive-descent cursor
	diags            []Diagnostic
	sink             func(Diagnostic)
	singleLineAsDesc bool // see WithSingleLineCommentAsDescription
}

func (s *parseState) emit(d Diagnostic) {
	if s.sink != nil {
		s.sink(d)
	}
	s.diags = append(s.diags, d)
}

// peek returns the token at s.pos without consuming it. Reserved for
// future order-sensitive productions — see parseState godoc.
//
//nolint:unused // prepare full recursive-descent for order-sensitive grammar productions
func (s *parseState) peek() Token {
	if s.pos >= len(s.tokens) {
		return Token{Kind: TokenEOF}
	}
	return s.tokens[s.pos]
}

// advance returns the token at s.pos and moves the cursor one forward.
// Reserved for future order-sensitive productions — see parseState
// godoc.
//
//nolint:unused // prepare full recursive-descent for order-sensitive grammar productions
func (s *parseState) advance() Token {
	t := s.peek()
	if s.pos < len(s.tokens) {
		s.pos++
	}
	return t
}

// parseTokens dispatches the stream to the family-specific parser.
//
//nolint:ireturn // stable seam.
func (p *DefaultParser) parseTokens(tokens []Token) Block {
	s := &parseState{tokens: tokens, sink: p.sink, singleLineAsDesc: p.singleLineAsDesc}
	annIdx := findAnnotation(tokens)
	if annIdx < 0 {
		return s.parseUnboundBlock()
	}

	annTok := tokens[annIdx]
	kind := AnnotationKindFromName(annTok.Name)
	switch kind.family() {
	case familySchema:
		return s.parseSchemaBlock(annIdx, annTok, kind)
	case familyOperation:
		return s.parseOperationBlock(annIdx, annTok, kind)
	case familyMeta:
		return s.parseMetaBlock(annIdx, annTok)
	case familyClassifier:
		return s.parseClassifierBlock(annIdx, annTok, kind)
	case familyUnknown:
		fallthrough
	default:
		return s.parseUnboundBlock()
	}
}

// findAnnotation returns the index of the first TokenAnnotation, or -1.
func findAnnotation(tokens []Token) int {
	for i, t := range tokens {
		if t.Kind == TokenAnnotation {
			return i
		}
	}
	return -1
}

// extractTitleDesc walks the prose tokens of a block (or a sub-slice such as the pre-annotation
// preamble) and returns:
//
//   - title — TokenTitle texts joined with "\n"
//   - desc  — TokenDesc texts joined with "\n", with internal blank
//     lines preserved as "" entries (paragraph breaks)
//   - lines — interleaved title/desc/blank entries in source order,
//     blanks rendered as "" — the ProseLines / PreambleLines shape
//     consumers use
//
// Join semantics: a single trailing blank is dropped from each side, internal blanks are kept
// verbatim so paragraph breaks survive.
func extractTitleDesc(tokens []Token) (title, desc string, lines []string) {
	var titleLines, descLines []string
	const (
		stateBeforeTitle = iota
		stateInTitle
		stateInDesc
	)
	state := stateBeforeTitle

	for _, t := range tokens {
		switch t.Kind {
		case TokenTitle:
			titleLines = append(titleLines, t.Text)
			lines = append(lines, t.Text)
			state = stateInTitle
		case TokenDesc:
			descLines = append(descLines, t.Text)
			lines = append(lines, t.Text)
			state = stateInDesc
		case TokenBlank:
			lines = append(lines, "")
			switch state {
			case stateInTitle:
				// Either an internal blank in a multi-paragraph title or a separator before the desc run
				// starts.
				// The trailing-blank trim below resolves the latter.
				titleLines = append(titleLines, "")
			case stateInDesc:
				descLines = append(descLines, "")
			}
		default:
			// ignored token
		}
	}

	lines = dropTrailingBlankLines(lines)
	titleLines = dropTrailingBlankLines(titleLines)
	descLines = dropTrailingBlankLines(descLines)
	title = strings.Join(titleLines, "\n")
	desc = strings.Join(descLines, "\n")
	return title, desc, lines
}

// dropTrailingBlankLines drops every trailing whitespace-only line from ls.
//
// The state-machine in extractTitleDesc over-appends separator blanks (e.g. a TITLE → BLANK+BLANK
// → DESC sequence pushes both blanks onto titleLines); trimming the whole tail lands on the
// desired shape.
func dropTrailingBlankLines(ls []string) []string {
	for len(ls) > 0 && strings.TrimSpace(ls[len(ls)-1]) == "" {
		ls = ls[:len(ls)-1]
	}
	return ls
}

// finaliseBase populates Title/Description/ProseLines/PreambleLines and copies accumulated
// diagnostics onto the base block.
//
// PreambleLines / PreambleTitle / PreambleDescription hold the subset of prose that appears BEFORE
// the block's annotation.
// Schema's top-level model builder consumes only pre-annotation prose so post-annotation text reads
// as body content.
//
// Routes / operations / meta consult Title() and Description(), which span the whole block.
func (s *parseState) finaliseBase(base *baseBlock) {
	t, d, lines := extractTitleDesc(s.tokens)
	t, d = s.demoteSingleLineTitle(t, d)
	base.title = t
	base.description = d
	base.proseLines = lines

	preTokens := s.tokens
	if annIdx := findAnnotation(s.tokens); annIdx >= 0 {
		preTokens = s.tokens[:annIdx]
	}
	pt, pd, preLines := extractTitleDesc(preTokens)
	pt, pd = s.demoteSingleLineTitle(pt, pd)
	base.preambleTitle = pt
	base.preambleDescription = pd
	base.preambleLines = preLines

	base.diagnostics = append(base.diagnostics, s.diags...)
}

// demoteSingleLineTitle implements the SingleLineCommentAsDescription option (go-swagger#2626):
// when the prose resolved to a one-line title with no description, move that single line to the
// description so a lone doc comment never becomes a title / summary.
//
// A title spanning multiple lines, or any prose that already produced a description (i.e. a
// multi-line comment), is left untouched.
func (s *parseState) demoteSingleLineTitle(title, desc string) (string, string) {
	if !s.singleLineAsDesc {
		return title, desc
	}
	if title == "" || desc != "" || strings.Contains(title, "\n") {
		return title, desc
	}
	return "", title
}

// --- UnboundBlock ------------------------------------------------------------

//nolint:ireturn // stable seam.
func (s *parseState) parseUnboundBlock() Block {
	base := newBaseBlock(AnnUnknown, firstMeaningfulPos(s.tokens))
	for _, t := range s.tokens {
		s.consumeBodyToken(base, t, AnnUnknown)
	}
	s.finaliseBase(base)
	return &UnboundBlock{baseBlock: base}
}

// firstMeaningfulPos picks the first non-blank, non-EOF token's position so an UnboundBlock has a
// sensible Pos().
func firstMeaningfulPos(tokens []Token) token.Position {
	for _, t := range tokens {
		switch t.Kind {
		case TokenEOF, TokenBlank:
			continue
		default:
			return t.Pos
		}
	}
	return token.Position{}
}

// --- Schema family -----------------------------------------------------------

//nolint:ireturn // stable seam.
func (s *parseState) parseSchemaBlock(annIdx int, annTok Token, kind AnnotationKind) Block {
	base := newBaseBlock(kind, annTok.Pos)

	// Validate annotation arguments before walking body tokens so any emitted diagnostics land on the
	// block via finaliseBase.
	switch kind {
	case AnnParameters:
		if len(annTok.Args) == 0 {
			s.emit(Errorf(annTok.Pos, CodeMissingRequiredArg,
				"swagger:parameters requires a target (an operation id, `*`, or `/path`)"))
		}
	case AnnName:
		if firstIdentArg(annTok) == "" {
			s.emit(Errorf(annTok.Pos, CodeMissingRequiredArg,
				"swagger:name requires a member name override argument"))
		}
	default:
		// other schema-family annotations either accept no args (AnnModel) or accept an optional name
		// (AnnResponse).
	}

	// Walk pre + post tokens; pre-annotation prose contributes to Title/Description (already
	// classified by the lexer); pre-annotation body tokens (rare, godoc-style "annotation at the
	// bottom") are treated the same as post-annotation body tokens.
	for i, t := range s.tokens {
		if i == annIdx {
			continue
		}
		s.consumeBodyToken(base, t, kind)
	}
	s.finaliseBase(base)

	switch kind {
	case AnnModel:
		return &ModelBlock{baseBlock: base, Name: firstIdentArg(annTok)}
	case AnnResponse:
		return &ResponseBlock{baseBlock: base, Name: firstIdentArg(annTok)}
	case AnnParameters:
		target, path, args, dups := parseParametersArgs(annTok)
		return &ParametersBlock{baseBlock: base, Target: target, Path: path, Args: args, Dups: dups}
	case AnnName:
		return &NameBlock{baseBlock: base, Name: firstIdentArg(annTok)}
	case AnnTitle, AnnDescription:
		// Override annotations dispatch through the schema parser (so co-located validation keywords
		// surface as Properties) but carry no typed block of their own — a ClassifierBlock holds the
		// free-text arg (AnnotationArg) plus any harvested body keywords.
		return &ClassifierBlock{baseBlock: base, Args: annTok.Args}
	default:
		return &UnboundBlock{baseBlock: base}
	}
}

// --- Operation family --------------------------------------------------------

//nolint:ireturn // stable seam.
func (s *parseState) parseOperationBlock(annIdx int, annTok Token, kind AnnotationKind) Block {
	base := newBaseBlock(kind, annTok.Pos)

	method, path, tags, opID := s.parseOperationArgs(annTok)

	// swagger:route does not allow OPAQUE_YAML; flag if seen.
	allowYAML := kind == AnnOperation
	for i, t := range s.tokens {
		if i == annIdx {
			continue
		}
		if t.Kind == TokenOpaqueYaml && !allowYAML {
			s.emit(Errorf(t.Pos, CodeUnexpectedToken,
				"OPAQUE_YAML body is not legal under swagger:route"))
			continue
		}
		s.consumeBodyToken(base, t, kind)
	}
	s.finaliseBase(base)

	if kind == AnnRoute {
		return &RouteBlock{
			baseBlock: base,
			Method:    method,
			Path:      path,
			Tags:      tags,
			OpID:      opID,
		}
	}
	return &InlineOperationBlock{
		baseBlock: base,
		Method:    method,
		Path:      path,
		Tags:      tags,
		OpID:      opID,
	}
}

// parseOperationArgs extracts METHOD, /path, [tags…], OperationID.
//
// Trailing IDENT_NAME is the OpID; any preceding IDENT_NAMEs are tags.
// See README §annotation-args.
func (s *parseState) parseOperationArgs(annTok Token) (method, path string, tags []string, opID string) {
	args := annTok.Args
	// Method.
	if len(args) > 0 && args[0].Kind == TokenHTTPMethod {
		method = args[0].Text
		args = args[1:]
	} else {
		s.emit(Errorf(annTok.Pos, CodeMalformedOperation,
			"swagger:%s: missing or invalid HTTP method", annTok.Name))
	}
	// Path.
	if len(args) > 0 && args[0].Kind == TokenURLPath {
		path = args[0].Text
		args = args[1:]
	} else {
		s.emit(Errorf(annTok.Pos, CodeMalformedOperation,
			"swagger:%s: missing or invalid URL path", annTok.Name))
	}
	// Trailing ident is the OpID; any preceding idents are tags.
	if len(args) == 0 {
		s.emit(Errorf(annTok.Pos, CodeMalformedOperation,
			"swagger:%s: missing operation id", annTok.Name))
		return method, path, nil, ""
	}
	last := args[len(args)-1]
	if last.Kind != TokenIdentName {
		s.emit(Errorf(last.Pos, CodeMalformedOperation,
			"swagger:%s: trailing operation id must be an identifier", annTok.Name))
	}
	opID = last.Text
	args = args[:len(args)-1]
	for _, a := range args {
		if a.Kind == TokenIdentName {
			tags = append(tags, a.Text)
		}
	}
	return method, path, tags, opID
}

// --- Meta family -------------------------------------------------------------

//nolint:ireturn // stable seam.
func (s *parseState) parseMetaBlock(annIdx int, annTok Token) Block {
	base := newBaseBlock(AnnMeta, annTok.Pos)
	for i, t := range s.tokens {
		if i == annIdx {
			continue
		}
		s.consumeBodyToken(base, t, AnnMeta)
	}
	s.finaliseBase(base)
	return &MetaBlock{baseBlock: base}
}

// --- Classifier family -------------------------------------------------------

//nolint:ireturn // stable seam.
func (s *parseState) parseClassifierBlock(annIdx int, annTok Token, kind AnnotationKind) Block {
	base := newBaseBlock(kind, annTok.Pos)

	// Classifier bodies are prose-only.
	//
	// EnumDeclBlock additionally allows a multi-line RAW_VALUE_ENUM body.
	// Other body content surfaces as a context-invalid warning.
	var enumBody string
	for i, t := range s.tokens {
		if i == annIdx {
			continue
		}
		switch t.Kind {
		case TokenTitle, TokenDesc, TokenBlank, TokenEOF, TokenAnnotation:
			// Prose / structural — counted by extractTitleDesc.
		case TokenRawValueBody:
			if kind == AnnEnum && t.Keyword == "enum" {
				enumBody = t.Body
				continue
			}
			s.emit(Warnf(t.Pos, CodeContextInvalid,
				"keyword %q not valid under swagger:%s", t.Keyword, kind))
		case TokenRawBlockBody:
			s.emit(Warnf(t.Pos, CodeContextInvalid,
				"keyword %q not valid under swagger:%s", t.Keyword, kind))
		case TokenKeyword:
			s.emit(Warnf(t.Pos, CodeContextInvalid,
				"keyword %q not valid under swagger:%s", t.Name, kind))
		case TokenOpaqueYaml:
			s.emit(Warnf(t.Pos, CodeUnexpectedToken,
				"YAML body not legal under swagger:%s", kind))
		default:
			// ignored kind
		}
	}

	// Argument validation runs before finaliseBase so its diagnostics reach the returned Block.
	switch kind {
	case AnnEnum:
		// A bare `swagger:enum` (no name, no inline values, no body) is valid on a type declaration: the
		// builder infers the enum name from the declared type and collects its consts (F4b).
		// Only the grammar's structural shape is checked here, and the bare form is structurally fine —
		// semantic resolution (does a type with consts exist?) is the builder's job.
		form, name, _, valuesArgs := splitEnumArgs(annTok)
		s.finaliseBase(base)
		return &EnumDeclBlock{
			baseBlock:  base,
			Name:       name,
			InlineForm: form,
			InlineArgs: valuesArgs,
			BodyValues: enumBody,
		}
	case AnnStrfmt:
		if firstIdentArg(annTok) == "" {
			s.emit(Errorf(annTok.Pos, CodeMissingRequiredArg,
				"swagger:strfmt requires a name argument"))
		}
	case AnnDefaultName:
		if !annTok.HasArg(1) {
			s.emit(Errorf(annTok.Pos, CodeMissingRequiredArg,
				"swagger:default requires a value argument"))
		}
	case AnnType:
		// Only the STRUCTURAL shape is checked here: a missing arg, or a malformed token (embedded
		// spaces, bare `[]`, illegal chars).
		// Whether the (well-formed) name is a known keyword / scanned type is resolved by the builder,
		// which alone knows the scanned definitions and the annotated Go type (F3 reconciliation).
		if len(annTok.Args) == 0 {
			s.emit(Errorf(annTok.Pos, CodeMissingRequiredArg,
				"swagger:type requires a type-reference argument"))
		} else if annTok.Args[0].Kind != TokenTypeRef {
			s.emit(Errorf(annTok.Args[0].Pos, CodeInvalidTypeRef,
				"swagger:type: %q is not a well-formed type reference", annTok.Args[0].Text))
		}
	case AnnAllOf, AnnIgnore, AnnAlias, AnnFile:
	// Optional / no args.
	default:
		// ignored annotation
	}
	s.finaliseBase(base)
	return &ClassifierBlock{baseBlock: base, Args: annTok.Args}
}

// splitEnumArgs reconstructs the (form, name, name-pos, value-tokens) from a TokenAnnotation
// produced for swagger:enum.
func splitEnumArgs(annTok Token) (enumArgsForm, string, token.Position, []Token) {
	if len(annTok.Args) == 0 {
		return enumFormEmpty, "", annTok.Pos, nil
	}

	first := annTok.Args[0]

	// Bracketed-only — single JSON_VALUE arg, no name.
	if len(annTok.Args) == 1 && first.Kind == TokenJSONValue {
		return enumFormBracketedOnly, "", first.Pos, annTok.Args
	}
	// Plain-only — single COMMA_LIST_VALUE arg, no name.
	if len(annTok.Args) == 1 && first.Kind == TokenCommaListValue {
		return enumFormPlainOnly, "", first.Pos, annTok.Args
	}
	// Name-only — single IDENT_NAME arg.
	if len(annTok.Args) == 1 && first.Kind == TokenIdentName {
		return enumFormNameOnly, first.Text, first.Pos, nil
	}
	// Name + bracketed.
	if len(annTok.Args) == 2 && first.Kind == TokenIdentName && annTok.Args[1].Kind == TokenJSONValue {
		return enumFormNamePlusBracketed, first.Text, first.Pos, annTok.Args[1:]
	}
	// Name + plain.
	if len(annTok.Args) == 2 && first.Kind == TokenIdentName && annTok.Args[1].Kind == TokenCommaListValue {
		return enumFormNamePlusPlain, first.Text, first.Pos, annTok.Args[1:]
	}
	return enumFormEmpty, "", annTok.Pos, annTok.Args
}

// firstIdentArg returns the Text of the first IDENT_NAME-typed arg, or "".
func firstIdentArg(annTok Token) string {
	for _, a := range annTok.Args {
		if a.Kind == TokenIdentName {
			return a.Text
		}
	}
	return ""
}

// parseParametersArgs classifies the arguments of a `swagger:parameters` marker into a target
// (operations / shared `*` / `/path`) plus the remaining argument tokens, de-duplicated.
//
// Dropped duplicates are returned separately so the builder can raise a duplicate-target /
// duplicate-ref warning.
// The definition-vs-reference reading of the args is the builder's, since it depends on the host
// declaration.
func parseParametersArgs(annTok Token) (target ParametersTarget, path string, args, dups []string) {
	target = ParamTargetOperations
	rest := annTok.Args
	if len(rest) > 0 {
		switch rest[0].Kind {
		case TokenWildcard:
			target = ParamTargetShared
			rest = rest[1:]
		case TokenURLPath:
			target = ParamTargetPath
			path = rest[0].Text
			rest = rest[1:]
		default:
			// First token is an operation id: keep it among the args.
		}
	}

	seen := make(map[string]struct{}, len(rest))
	for _, a := range rest {
		if a.Text == "" {
			continue
		}
		if _, dup := seen[a.Text]; dup {
			dups = append(dups, a.Text)
			continue
		}
		seen[a.Text] = struct{}{}
		args = append(args, a.Text)
	}
	return target, path, args, dups
}

// --- Body-token consumption (shared across families) -----------------------

// consumeBodyToken folds one token from the stream into the block's state.
//
// Prose, blank, EOF, and the annotation are no-ops here (they are consumed by extractTitleDesc /
// dispatch).
func (s *parseState) consumeBodyToken(base *baseBlock, t Token, kind AnnotationKind) {
	switch t.Kind {
	case TokenEOF, TokenBlank, TokenTitle, TokenDesc, TokenAnnotation:
		// Handled elsewhere.
	case TokenKeyword:
		s.emitInlineKeyword(base, t, kind)
	case TokenRawBlockBody:
		s.emitRawBlock(base, t, kind)
	case TokenRawValueBody:
		s.emitRawValue(base, t, kind)
	case TokenOpaqueYaml:
		base.yamlBlocks = append(base.yamlBlocks, RawYAML{
			Pos:       t.Pos,
			Text:      t.Body,
			Truncated: t.Truncated,
		})
		if t.Truncated {
			s.emit(Errorf(t.Pos, CodeUnterminatedYAML,
				"YAML body opened with --- but never closed"))
		}
	default:
		// Stray value-only tokens (e.g. trailing IDENT_NAME with no owning keyword) are not legal at the
		// body level.
		s.emit(Warnf(t.Pos, CodeUnexpectedToken,
			"unexpected %s token", t.Kind))
	}
}

// emitInlineKeyword stores an inline-value keyword as a Property.
func (s *parseState) emitInlineKeyword(base *baseBlock, t Token, kind AnnotationKind) {
	kw, ok := Lookup(t.Name)
	if !ok {
		// Lexer should never emit TokenKeyword for unknown keywords, but guard defensively.
		return
	}
	if !contextLegal(kw, kind) {
		s.emit(Warnf(t.Pos, CodeContextInvalid,
			"keyword %q not valid under swagger:%s (legal in: %s)",
			kw.Name, kind, formatContexts(kw)))
	}
	prop := Property{
		Keyword:    kw,
		Pos:        t.Pos,
		Value:      t.Text,
		ItemsDepth: t.ItemsDepth,
		Typed:      s.typeInlineValue(kw, t),
	}
	base.properties = append(base.properties, prop)
}

// typeInlineValue converts the typed argument token's payload into a TypedValue per the keyword's
// declared shape.
//
// Failures emit non-fatal diagnostics; on failure Typed.Type stays at ShapeNone so consumers can
// distinguish "no conversion" from "zero converted".
func (s *parseState) typeInlineValue(kw Keyword, t Token) TypedValue {
	if len(t.Args) == 0 {
		return TypedValue{}
	}
	arg := t.Args[0]
	switch kw.Shape {
	case ShapeNumber:
		op, rest := splitCmpOperator(arg.Text)
		n, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			s.emit(Errorf(arg.Pos, CodeInvalidNumber,
				"%s: %q is not a valid number", kw.Name, arg.Text))
			return TypedValue{}
		}
		return TypedValue{Type: ShapeNumber, Op: op, Number: n}
	case ShapeInt:
		i, err := strconv.ParseInt(strings.TrimSpace(arg.Text), 10, 64)
		if err != nil {
			s.emit(Errorf(arg.Pos, CodeInvalidInteger,
				"%s: %q is not a valid integer", kw.Name, arg.Text))
			return TypedValue{}
		}
		return TypedValue{Type: ShapeInt, Integer: i}
	case ShapeBool:
		b, ok := parseBool(arg.Text)
		if !ok {
			s.emit(Errorf(arg.Pos, CodeInvalidBoolean,
				"%s: %q is not a valid boolean (expected true or false)", kw.Name, arg.Text))
			return TypedValue{}
		}
		return TypedValue{Type: ShapeBool, Boolean: b}
	case ShapeEnumOption:
		for _, allowed := range kw.Values {
			if strings.EqualFold(arg.Text, allowed) {
				return TypedValue{Type: ShapeEnumOption, String: allowed}
			}
		}
		s.emit(Errorf(arg.Pos, CodeInvalidEnumOption,
			"%s: %q is not one of {%s}",
			kw.Name, arg.Text, strings.Join(kw.Values, ", ")))
		return TypedValue{}
	case ShapeNone, ShapeString, ShapeCommaList, ShapeRawBlock, ShapeRawValue:
		return TypedValue{}
	default:
		// ignored shape
	}
	return TypedValue{}
}

// emitRawBlock stores a multi-line raw-block body and, for extensions blocks, parses out the x-*
// entries.
func (s *parseState) emitRawBlock(base *baseBlock, t Token, kind AnnotationKind) {
	kw, ok := Lookup(t.Keyword)
	if !ok {
		return
	}
	if !contextLegal(kw, kind) {
		s.emit(Warnf(t.Pos, CodeContextInvalid,
			"keyword %q not valid under swagger:%s (legal in: %s)",
			kw.Name, kind, formatContexts(kw)))
	}
	prop := Property{
		Keyword:    kw,
		Pos:        t.Pos,
		Body:       t.Body,
		Raw:        t.Raw,
		Typed:      TypedValue{Type: ShapeRawBlock},
		ItemsDepth: t.ItemsDepth,
	}
	base.properties = append(base.properties, prop)

	if kw.Name == "extensions" || kw.Name == "infoExtensions" {
		s.collectExtensionsFromBody(base, t, kw.Name)
	}
	if kw.Name == KwSecurity {
		base.security = security.Parse(t.Body)
	}
}

// emitRawValue stores a raw-value body keyword.
func (s *parseState) emitRawValue(base *baseBlock, t Token, kind AnnotationKind) {
	kw, ok := Lookup(t.Keyword)
	if !ok {
		return
	}
	if !contextLegal(kw, kind) {
		s.emit(Warnf(t.Pos, CodeContextInvalid,
			"keyword %q not valid under swagger:%s (legal in: %s)",
			kw.Name, kind, formatContexts(kw)))
	}
	prop := Property{
		Keyword:    kw,
		Pos:        t.Pos,
		Value:      t.Body,
		Body:       t.Body,
		Raw:        t.Raw,
		Typed:      TypedValue{Type: ShapeRawValue},
		ItemsDepth: t.ItemsDepth,
	}
	base.properties = append(base.properties, prop)
}

// collectExtensionsFromBody parses the body of an `extensions:` raw block via the typed-extensions
// service and registers one Extension per top-level x-* entry on the block, carrying its YAML-typed
// value (`bool` / `float64` / `string` / `[]any` / `map[string]any`).
//
// Non-x-* keys emit a CodeInvalidAnnotation warning and are dropped.
// A YAML parse failure emits CodeInvalidYAMLExtensions and the block is skipped (no Extension
// entries are registered).
//
// Position is currently coarse — every Extension shares t.Pos (the `extensions:` keyword's
// position).
// See README §typed-extensions.
func (s *parseState) collectExtensionsFromBody(base *baseBlock, t Token, source string) {
	data, err := yaml.TypedExtensions(t.Body)
	if err != nil {
		s.emit(Warnf(t.Pos, CodeInvalidYAMLExtensions,
			"extensions block: %v", err))
		return
	}
	for name, value := range data {
		if !isExtensionName(name) {
			// Non-x-* key — diagnose then drop.
			// Authors who typo a vendor-extension key (e.g. `invalid-key:` under `Extensions:`) get a
			// CodeInvalidAnnotation warning rather than silent loss.
			//
			// Builders that consume block.Extensions() never see the rejected entry; they observe the
			// diagnostic via the Diagnostic Walker callback.
			s.emit(Warnf(t.Pos, CodeInvalidAnnotation,
				"extensions block: %q is not a valid vendor-extension name (must start with x- or X-); dropped",
				name))
			continue
		}
		base.extensions = append(base.extensions, Extension{
			Name:   name,
			Source: source,
			Pos:    t.Pos,
			Value:  value,
		})
	}
}

// isExtensionName reports whether s is a well-formed x-* / X-* name.
func isExtensionName(s string) bool {
	const minExtNameLen = 3
	if len(s) < minExtNameLen {
		return false
	}
	if (s[0] != 'x' && s[0] != 'X') || s[1] != '-' {
		return false
	}
	return true
}

// --- shared helpers ---------------------------------------------------------

// contextLegal reports whether kw may appear under the given annotation kind.
//
// Returns true when the keyword's contexts overlap with the kind's allowed contexts; returns true
// unconditionally when the kind has no parser-layer policy (e.g. UnboundBlock).
func contextLegal(kw Keyword, kind AnnotationKind) bool {
	allowed := allowedContexts(kind)
	if allowed == nil {
		return true
	}
	for _, c := range kw.Contexts {
		if slices.Contains(allowed, c) {
			return true
		}
	}
	return false
}

// allowedContexts maps an annotation kind to the keyword contexts that are legal under it.
//
// Classifier kinds have no body keywords (return nil — no parser-layer policy).
// See README §context-legality.
func allowedContexts(kind AnnotationKind) []KeywordContext {
	switch kind {
	case AnnModel:
		return []KeywordContext{CtxSchema, CtxItems}
	case AnnParameters:
		return []KeywordContext{CtxParam, CtxSchema, CtxItems}
	case AnnResponse:
		return []KeywordContext{CtxResponse, CtxSchema, CtxHeader, CtxItems}
	case AnnOperation:
		return []KeywordContext{CtxOperation, CtxParam, CtxSchema, CtxHeader, CtxItems, CtxResponse}
	case AnnRoute:
		return []KeywordContext{CtxRoute, CtxParam, CtxSchema, CtxHeader, CtxItems, CtxResponse}
	case AnnMeta:
		return []KeywordContext{CtxMeta, CtxSchema}
	case AnnUnknown,
		AnnStrfmt, AnnAlias, AnnName, AnnAllOf, AnnEnum,
		AnnIgnore, AnnDefaultName, AnnType, AnnFile:
		return nil
	default:
		return nil
	}
}

// formatContexts renders a keyword's legal contexts for diagnostics.
func formatContexts(kw Keyword) string {
	parts := make([]string, len(kw.Contexts))
	for i, c := range kw.Contexts {
		parts[i] = c.String()
	}
	return strings.Join(parts, ", ")
}

// splitCmpOperator strips a leading comparison operator from a number value.
//
// Supports the `maximum: <5` form.
func splitCmpOperator(s string) (op, rest string) {
	s = strings.TrimLeft(s, " \t")
	for _, c := range []string{"<=", ">=", "<", ">", "="} {
		if strings.HasPrefix(s, c) {
			return c, s[len(c):]
		}
	}
	return "", s
}

// parseBool accepts only "true" or "false" (case-insensitive). stdlib strconv.ParseBool is too
// lenient for the annotation grammar.
func parseBool(s string) (bool, bool) {
	s = strings.TrimSpace(s)
	switch {
	case strings.EqualFold(s, "true"):
		return true, true
	case strings.EqualFold(s, "false"):
		return false, true
	default:
		return false, false
	}
}
