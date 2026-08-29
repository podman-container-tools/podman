// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"go/token"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lex turns a preprocessed []Line into the token stream consumed by the grammar parser.
//
// Pipeline:
//
//  1. Line classifier (lexLine): per-line classification into raw
//     Tokens (annotation / keyword / fence / blank / text).
//  2. Body accumulator: folds multi-line bodies (OPAQUE_YAML, RAW_BLOCK,
//     RAW_VALUE) into single body tokens.
//  3. Prose classifier: re-types surviving text tokens as TITLE / DESC.
//
// The output stream ends in a single TokenEOF.
//
// # Details
//
// See README §lexer-contract for the per-stage rules, §raw-block-terminators for body-termination
// rules, and §prose-classification for the TITLE / DESC split heuristics.
func Lex(lines []Line) []Token {
	raw := classifyLines(lines)
	bodied := accumulateBodies(raw)
	return classifyProse(bodied)
}

// ----- Stage 1 — line classifier --------------------------------------------

// classifyLines emits one preliminary token per line.
//
// The state it carries between lines is whether the cursor sits between matching `---` fences (so
// YAML bodies survive verbatim); body accumulation happens later, in stage 2.
func classifyLines(lines []Line) []Token {
	out := make([]Token, 0, len(lines)+1)
	inFence := false
	inLiteralDesc := false
	for _, line := range lines {
		if inLiteralDesc {
			// Inside a `swagger:description |` literal block, every line is captured verbatim until the next
			// annotation or EOF — no `---` fence toggling, no blank-line or keyword termination.
			// This MUST happen here (stage 1) because a lone `---` in the body would otherwise flip inFence
			// and swallow a following annotation as raw YAML.
			//
			// See README §literal-description.
			tok := lexLine(line, false)
			if tok.Kind != TokenAnnotation {
				out = append(out, Token{Kind: tokenRawLine, Pos: line.Pos, Text: line.Raw, Raw: line.Raw})
				continue
			}
			// An annotation terminates the block; re-process it normally so a back-to-back
			// `swagger:description |` re-opens literal mode.
			inLiteralDesc = false
			out = append(out, tok)
			if isLiteralDescMarker(tok) {
				inLiteralDesc = true
			}
			continue
		}

		tok := lexLine(line, inFence)
		out = append(out, tok)
		if tok.Kind == tokenYAMLFence {
			inFence = !inFence
		}
		if !inFence && isLiteralDescMarker(tok) {
			inLiteralDesc = true
		}
	}
	return out
}

// isLiteralDescMarker reports whether tok is a `swagger:description |` annotation: a description
// override whose sole inline argument is the YAML literal block-scalar marker `|`.
//
// The marker opts the body into verbatim capture (blank lines and indentation preserved) instead of
// the default blank-terminated Option B fold.
func isLiteralDescMarker(tok Token) bool {
	return tok.Kind == TokenAnnotation &&
		tok.Name == labelDescription &&
		len(tok.Args) == 1 &&
		strings.TrimSpace(tok.Args[0].Text) == "|"
}

// lexLine classifies one line. Returns one of:
//   - TokenBlank
//   - tokenYAMLFence
//   - tokenRawLine        (verbatim line inside an active fence)
//   - TokenAnnotation     (with pre-classified Args)
//   - tokenKeywordPre     (head + value-string, body accumulator decides next)
//   - tokenText           (free-form prose; later re-typed as TITLE/DESC)
func lexLine(line Line, inFence bool) Token {
	text := strings.TrimRightFunc(line.Text, unicode.IsSpace)

	if strings.TrimSpace(text) == "---" {
		return Token{Kind: tokenYAMLFence, Pos: line.Pos}
	}
	if inFence {
		return Token{Kind: tokenRawLine, Pos: line.Pos, Text: line.Raw, Raw: line.Raw}
	}
	if text == "" {
		return Token{Kind: TokenBlank, Pos: line.Pos}
	}

	// First-character case insensitivity on swagger:<name>: only the leading character flips.
	if hasSwaggerPrefix(text) {
		return lexAnnotation(text, line.Pos)
	}
	if pfxLen, ok := matchGodocRoutePrefix(text); ok {
		pos := line.Pos
		pos.Column += pfxLen
		pos.Offset += pfxLen
		return lexAnnotation(text[pfxLen:], pos)
	}
	// Go compiler / linter directives (`//go:generate`, `//nolint:foo`, `//lint:ignore`, …) —
	// recognise on Raw (which preserves the post-`//` spacing) and drop from the prose surface so they
	// never land in TITLE / DESC.
	//
	// Must run after the swagger-prefix check so `//swagger:model` (legal but non-idiomatic, no
	// leading space) is not mistaken for a directive.
	if isGoDirective(line.Raw) {
		return Token{Kind: tokenDirective, Pos: line.Pos, Raw: line.Raw}
	}
	if tok, ok := lexKeyword(text, line.Raw, line.Pos); ok {
		return tok
	}
	return Token{Kind: tokenText, Pos: line.Pos, Text: text, Raw: line.Raw}
}

// isGoDirective reports whether raw is the body of a Go compiler / linter directive comment.
//
// A directive has the form `<lowercase-word>:<args>` where:
//
//   - the leading character is a lowercase ASCII letter (no leading
//     whitespace — distinguishes `//nolint:foo` from `// nolint:foo`);
//   - the leading word is lowercase ASCII letters only;
//   - the word is followed by `:` and **at least one non-whitespace
//     character** with no whitespace between the colon and the
//     argument.
//
// The "no whitespace after colon" rule separates directives from keyword lines: `maximum: 10`
// (space → keyword), `pattern:` (empty → block head), `nolint:foo` (immediate arg →
// directive).
//
// Note: `swagger:<name>` matches this shape; lexLine runs the swagger check before the directive
// check so swagger annotations are never dropped.
func isGoDirective(raw string) bool {
	if raw == "" || raw[0] < 'a' || raw[0] > 'z' {
		return false
	}
	i := 1
	for i < len(raw) && raw[i] >= 'a' && raw[i] <= 'z' {
		i++
	}
	if i >= len(raw) || raw[i] != ':' {
		return false
	}
	after := i + 1
	if after >= len(raw) {
		return false
	}
	if raw[after] == ' ' || raw[after] == '\t' {
		return false
	}
	return true
}

// isDirectiveMarker reports whether text is a Go "marker" comment of the kind emitted by Kubernetes
// code-generation tooling (kubebuilder, controller-gen, k8s deepcopy-gen, genclient): a line whose
// content begins with `+` immediately followed by an ASCII letter, e.g. `+genclient`,
// `+kubebuilder:validation:Required`, `+k8s:deepcopy-gen=…`.
//
// These markers are not part of the swagger annotation grammar and must not leak into model /
// property descriptions (go-swagger#2687, the residual of #3007); lexLine drops them from the prose
// surface exactly like Go directives.
//
// text is the godoc-stripped Line.Text, so both the common kubebuilder form `// +marker` (space
// after the comment marker) and the bare `//+marker` arrive here as `+marker`.
// Requiring a letter after the `+` avoids eating prose that merely opens with a sign (e.g. "+1 for
// …").
func isDirectiveMarker(text string) bool {
	if len(text) < 2 || text[0] != '+' {
		return false
	}
	c := text[1]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// hasSwaggerPrefix is the case-insensitive match on the first char of AnnotationPrefix — only the
// first character is permissive.
//
// AnnotationPrefix is fixed at "swagger:" so the case-insensitive fallback is tied to ASCII letter
// casing of its first byte.
// See README §quirks-open.
func hasSwaggerPrefix(s string) bool {
	if len(s) < len(AnnotationPrefix) {
		return false
	}
	first := AnnotationPrefix[0]
	if s[0] != first && s[0] != asciiUpper(first) {
		return false
	}
	return s[1:len(AnnotationPrefix)] == AnnotationPrefix[1:]
}

// asciiUpper returns the uppercase form of an ASCII letter, or the byte unchanged otherwise.
//
// Used for the first-character case- insensitive match on AnnotationPrefix.
func asciiUpper(b byte) byte {
	if b >= 'a' && b <= 'z' {
		return b - ('a' - 'A')
	}
	return b
}

// matchGodocRoutePrefix recognises a leading "GoIdent <ws>swagger:route".
//
// Returns the byte offset where "swagger:route" begins.
// Only "route" gets this exception.
// See README §lexer-contract ("Godoc-prefix exception for swagger:route").
func matchGodocRoutePrefix(s string) (int, bool) {
	identEnd := scanGoIdentifier(s)
	if identEnd == 0 {
		return 0, false
	}
	wsEnd := identEnd
	for wsEnd < len(s) && (s[wsEnd] == ' ' || s[wsEnd] == '\t') {
		wsEnd++
	}
	if wsEnd == identEnd {
		return 0, false
	}
	prefix := AnnotationPrefix + labelRoute
	if !strings.HasPrefix(s[wsEnd:], prefix) {
		return 0, false
	}
	after := wsEnd + len(prefix)
	if after < len(s) && s[after] != ' ' && s[after] != '\t' {
		return 0, false
	}
	return wsEnd, true
}

// scanGoIdentifier returns the byte length of a leading Go identifier: Letter followed by Letter |
// Digit | _ | -.
//
// Returns 0 if s does not start with a letter.
func scanGoIdentifier(s string) int {
	if s == "" {
		return 0
	}
	r, size := utf8.DecodeRuneInString(s)
	if !unicode.IsLetter(r) {
		return 0
	}
	i := size
	for i < len(s) {
		r, size = utf8.DecodeRuneInString(s[i:])
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			break
		}
		i += size
	}
	return i
}

// lexAnnotation parses "swagger:<name> [args...]".
//
// Empty name falls back to a text token so the parser can diagnose.
// Args are returned pre-classified via classifyAnnotationArgs.
func lexAnnotation(text string, pos token.Position) Token {
	rawRest := strings.TrimRightFunc(text[len(AnnotationPrefix):], unicode.IsSpace)
	// Classify the kind from a dot-stripped view so a bare `swagger:model.` (and likewise
	// `swagger:description.`) still resolves to its kind.
	name, _ := splitFirstField(stripTrailingDot(rawRest))
	if name == "" {
		return Token{Kind: tokenText, Pos: pos, Text: text}
	}
	kind := AnnotationKindFromName(name)
	// Free-text overrides (swagger:title / swagger:description) keep a trailing "." — it is sentence
	// content, not punctuation noise.
	// Every other annotation elides a single trailing dot (e.g. `swagger:model Pet.`).
	rest := rawRest
	if kind != AnnTitle && kind != AnnDescription {
		rest = stripTrailingDot(rawRest)
	}
	_, after := splitFirstField(rest)
	args := classifyAnnotationArgs(kind, after, pos, len(text)-len(after))
	return Token{Kind: TokenAnnotation, Pos: pos, Name: name, Args: args}
}

// stripTrailingDot elides a single trailing ".".
//
// Source preservation lives upstream on Line.Raw.
func stripTrailingDot(s string) string {
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	return strings.TrimSuffix(s, ".")
}

// splitFirstField returns the first whitespace-delimited token and the remainder (with leading
// whitespace stripped).
func splitFirstField(s string) (head, rest string) {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return "", ""
	}
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' {
		i++
	}
	head = s[:i]
	rest = strings.TrimLeft(s[i:], " \t")
	return head, rest
}

// classifyAnnotationArgs converts the post-name remainder of an annotation line into the typed
// argument tokens per annotation kind.
//
// See README §annotation-args.
//
// The byte-offset baseColumn is the column at which `rest` begins inside the source line; positions
// are computed relative to that.
func classifyAnnotationArgs(kind AnnotationKind, rest string, linePos token.Position, baseColumn int) []Token {
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" {
		return nil
	}
	pos := linePos
	pos.Column = linePos.Column + baseColumn
	pos.Offset = linePos.Offset + baseColumn

	switch kind {
	case AnnRoute, AnnOperation:
		return classifyOperationArgs(rest, pos)
	case AnnDefaultName:
		return []Token{argDefaultValue(rest, pos)}
	case AnnType, AnnAdditionalProperties:
		// Both take a swagger:type-style spec as a single ref token (true / false / primitive / []T /
		// type-name).
		// The builder does the semantic resolution.
		return []Token{argTypeRef(rest, pos)}
	case AnnPatternProperties:
		// The arg is a `"<re>": <spec>, …` pair list that may contain spaces/colons/commas inside
		// quoted regexes — capture the whole remainder verbatim; the builder parses the pairs.
		return []Token{{Kind: TokenRawValue, Pos: pos, Text: strings.TrimSpace(rest)}}
	case AnnTitle, AnnDescription:
		// Free-text override: the arg is the whole rest of the line, verbatim (a title/description
		// sentence with spaces).
		// Captured as one TokenRawValue so ClassifierBlock.AnnotationArg() returns it intact. (Multi-line
		// description bodies are folded in a later phase.)
		return []Token{{Kind: TokenRawValue, Pos: pos, Text: strings.TrimSpace(rest)}}
	case AnnEnum:
		return classifyEnumAnnotationArgs(rest, pos)
	case AnnParameters:
		return classifyParametersArgs(rest, pos)
	case AnnResponse:
		// `swagger:response *` is a synonym for the bare form (a shared response keyed by the type name).
		// The wildcard carries no trailing tokens — responses have no op-id injection (unlike
		// parameters) and no /path form.
		// Any other argument is the explicit response name.
		if head, _ := splitFirstField(rest); head == "*" {
			return []Token{{Kind: TokenWildcard, Pos: pos, Text: "*"}}
		}
		return []Token{firstIdent(rest, pos)}
	case AnnAllOf, AnnModel, AnnStrfmt, AnnName:
		return []Token{firstIdent(rest, pos)}
	case AnnAlias, AnnIgnore, AnnFile, AnnMeta, AnnUnknown:
		// No formal arguments.
		// Capture any trailing tokens as RAW so a downstream diagnostic can flag them.
		return classifyIdentList(rest, pos)
	default:
		return classifyIdentList(rest, pos)
	}
}

// classifyOperationArgs extracts METHOD, /path, [tags...], and the trailing operationID.
//
// The trailing IDENT_NAME is the OpID; everything between path and the trailing ident is treated as
// a (potentially space-separated) tag list.
// See README §annotation-args.
func classifyOperationArgs(rest string, basePos token.Position) []Token {
	fields := splitFields(rest, basePos)
	if len(fields) == 0 {
		return nil
	}
	out := make([]Token, 0, len(fields))

	// Field 0: HTTP_METHOD (if recognised).
	first := fields[0]
	if canon, ok := classifyHTTPMethod(first.text); ok {
		out = append(out, Token{Kind: TokenHTTPMethod, Pos: first.pos, Text: canon})
		fields = fields[1:]
	}

	// Field 0 (now): URL_PATH (if it looks like one).
	if len(fields) > 0 && looksLikeURLPath(fields[0].text) {
		out = append(out, Token{Kind: TokenURLPath, Pos: fields[0].pos, Text: fields[0].text})
		fields = fields[1:]
	}

	// Remaining fields: every IDENT_NAME — the parser layer marks the trailing one as the OpID;
	// everything before it is a tag.
	for _, f := range fields {
		out = append(out, Token{Kind: TokenIdentName, Pos: f.pos, Text: f.text})
	}
	return out
}

// argDefaultValue handles the JSON_VALUE | RAW_VALUE alternation for swagger:default.
//
// See README §disambiguation.
func argDefaultValue(rest string, pos token.Position) Token {
	kind := classifyDefaultValue(rest)
	return Token{Kind: kind, Pos: pos, Text: strings.TrimSpace(rest)}
}

// argTypeRef tags a well-formed `swagger:type` argument as TYPE_REF and leaves the semantic check
// (known keyword / scanned type, format compatibility) to the builder.
//
// A structurally malformed token falls back to TokenIdentName so the parser can flag it (see
// looksLikeTypeRef).
func argTypeRef(rest string, pos token.Position) Token {
	rest = strings.TrimSpace(rest)
	if looksLikeTypeRef(rest) {
		return Token{Kind: TokenTypeRef, Pos: pos, Text: rest}
	}
	return Token{Kind: TokenIdentName, Pos: pos, Text: rest}
}

// classifyEnumAnnotationArgs implements the four-step EnumArgs dispatch rule.
//
// The values fragment, when present, is emitted as a single token whose kind reflects the
// bracketed-vs-plain decision; downstream parsing of the list items lives in the parser/analyzer.
// See README §disambiguation.
func classifyEnumAnnotationArgs(rest string, pos token.Position) []Token {
	form, name, values := classifyEnumArgs(rest)
	switch form {
	case enumFormEmpty:
		return nil
	case enumFormBracketedOnly:
		return []Token{{Kind: TokenJSONValue, Pos: pos, Text: values}}
	case enumFormPlainOnly:
		return []Token{{Kind: TokenCommaListValue, Pos: pos, Text: values}}
	case enumFormNameOnly:
		return []Token{{Kind: TokenIdentName, Pos: pos, Text: name}}
	case enumFormNamePlusBracketed:
		valuesPos := pos
		valuesPos.Column += len(name) + 1
		valuesPos.Offset += len(name) + 1
		return []Token{
			{Kind: TokenIdentName, Pos: pos, Text: name},
			{Kind: TokenJSONValue, Pos: valuesPos, Text: values},
		}
	case enumFormNamePlusPlain:
		valuesPos := pos
		valuesPos.Column += len(name) + 1
		valuesPos.Offset += len(name) + 1
		return []Token{
			{Kind: TokenIdentName, Pos: pos, Text: name},
			{Kind: TokenCommaListValue, Pos: valuesPos, Text: values},
		}
	default:
		return nil
	}
}

// classifyParametersArgs tokenises `swagger:parameters` arguments.
//
// The FIRST token may be the shared-namespace wildcard `*` (TokenWildcard) or a `/path`
// (TokenURLPath); every other token (and any first token that is neither) is an IDENT_NAME — an
// operation id or a shared-parameter name, disambiguated downstream by the parser/builder.
//
// `*` and `/path` are only recognised in first position, so `*blah` / a mid-list `/x` fall through
// to IDENT_NAME.
// See README §annotation-args.
func classifyParametersArgs(rest string, basePos token.Position) []Token {
	fields := splitFields(rest, basePos)
	out := make([]Token, 0, len(fields))
	for i, f := range fields {
		switch {
		case i == 0 && f.text == "*":
			out = append(out, Token{Kind: TokenWildcard, Pos: f.pos, Text: f.text})
		case i == 0 && looksLikeURLPath(f.text):
			out = append(out, Token{Kind: TokenURLPath, Pos: f.pos, Text: f.text})
		default:
			out = append(out, Token{Kind: TokenIdentName, Pos: f.pos, Text: f.text})
		}
	}
	return out
}

// classifyIdentList tokenises a whitespace-separated list as IDENT_NAME tokens.
func classifyIdentList(rest string, basePos token.Position) []Token {
	fields := splitFields(rest, basePos)
	out := make([]Token, 0, len(fields))
	for _, f := range fields {
		out = append(out, Token{Kind: TokenIdentName, Pos: f.pos, Text: f.text})
	}
	return out
}

// firstIdent emits a single TokenIdentName for the first whitespace- separated token in rest.
//
// Used for single-arg classifier annotations.
func firstIdent(rest string, basePos token.Position) Token {
	fields := splitFields(rest, basePos)
	if len(fields) == 0 {
		return Token{Kind: TokenIdentName, Pos: basePos}
	}
	return Token{Kind: TokenIdentName, Pos: fields[0].pos, Text: fields[0].text}
}

// field is one whitespace-separated token with a position.
type field struct {
	text string
	pos  token.Position
}

// splitFields breaks s into whitespace-separated fields, advancing the position by byte offset for
// each field.
func splitFields(s string, basePos token.Position) []field {
	const sensibleAllocs = 4
	out := make([]field, 0, sensibleAllocs)
	i := 0
	for i < len(s) {
		// Skip whitespace.
		start := i
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i == len(s) {
			break
		}
		offset := i
		j := i
		for j < len(s) && s[j] != ' ' && s[j] != '\t' {
			j++
		}
		f := field{
			text: s[i:j],
			pos: token.Position{
				Filename: basePos.Filename,
				Offset:   basePos.Offset + offset,
				Line:     basePos.Line,
				Column:   basePos.Column + offset,
			},
		}
		out = append(out, f)
		i = j
		_ = start
	}
	return out
}

// lexKeyword tries to parse text as a "[items.]*<keyword>: [value]" form.
//
// Returns (token, true) on a match.
// Always returns a tokenKeywordPre — head + raw value string.
// Body accumulation (stage 2) decides whether to keep it as inline-value KW or expand into a body.
func lexKeyword(text, raw string, pos token.Position) (Token, bool) {
	rest, depth := stripItemsPrefix(text)

	before, after, found := strings.Cut(rest, ":")
	if !found {
		return Token{}, false
	}

	name := strings.TrimSpace(before)
	if name == "" {
		return Token{}, false
	}
	// First-character case insensitivity: lowercase only the first character before lookup.
	// See README §lexer-contract ("First-character case insensitivity on keywords").
	canonName := lowerFirst(name)

	kw, ok := Lookup(canonName)
	if !ok {
		return Token{}, false
	}

	consumed := len(text) - len(rest)
	kwPos := pos
	kwPos.Column += consumed
	kwPos.Offset += consumed

	value := strings.TrimSpace(after)
	value = stripTrailingDot(value)

	// A `deprecated:` line whose argument is not a bool is the godoc "Deprecated: <reason>"
	// convention, not the bool keyword.
	// Leave it as prose (Block.IsDeprecated detects it via the godoc regexp) instead of forcing a bool
	// parse that would spuriously error and strip the reason from the description.
	//
	// The bool form keeps being a keyword (and drives the native operation `deprecated` field).
	// See go-swagger/go-swagger#3138.
	if kw.Name == KwDeprecated {
		if _, isBool := parseBool(value); !isBool {
			return Token{}, false
		}
	}

	return Token{
		Kind:       tokenKeywordPre,
		Pos:        kwPos,
		Name:       kw.Name,
		SourceName: name,
		Text:       value, // raw post-":" payload
		Raw:        raw,
		ItemsDepth: depth,
		// Keyword field reused to carry the table entry shape downstream.
		Keyword: kw.Name,
	}, true
}

// lowerFirst applies first-character lowercase; only the first character is case-permissive on
// keyword recognition.
//
// See README §lexer-contract ("First-character case insensitivity on keywords").
func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if unicode.IsUpper(r) {
		lower := unicode.ToLower(r)
		var buf [4]byte
		n := utf8.EncodeRune(buf[:], lower)
		return string(buf[:n]) + s[size:]
	}
	return s
}

// stripItemsPrefix peels leading "items." (or "items ") segments, counting depth.
//
// Bare "items:" (no separator) is preserved.
func stripItemsPrefix(s string) (string, int) {
	depth := 0
	for {
		stripped, ok := stripOneItemsPrefix(s)
		if !ok {
			return s, depth
		}
		s = stripped
		depth++
	}
}

func stripOneItemsPrefix(s string) (string, bool) {
	const itemsLen = 5
	if len(s) < itemsLen {
		return s, false
	}
	if !strings.EqualFold(s[:itemsLen], "items") {
		return s, false
	}
	rest := s[itemsLen:]
	trimmed := strings.TrimLeft(rest, ". \t")
	if len(trimmed) == len(rest) {
		return s, false
	}
	return trimmed, true
}

// ----- Stage 2 — body accumulator -------------------------------------------

// accumulateBodies folds multi-line bodies into single body tokens and finalises inline-value
// keywords by typing the value per the keyword's declared shape.
//
// The output stream contains only tokens the parser actually consumes (no internal kinds).
func accumulateBodies(in []Token) []Token {
	out := make([]Token, 0, len(in)+1)
	i := 0
	for i < len(in) {
		t := in[i]
		switch t.Kind {
		case tokenYAMLFence:
			i = collectFencedYAML(in, i, &out)
		case tokenKeywordPre:
			kw, _ := Lookup(t.Name)
			switch kw.Shape {
			case ShapeRawBlock:
				i = collectRawBlock(in, i, kw, &out)
			case ShapeRawValue:
				i = collectRawValue(in, i, kw, &out)
			case ShapeNone, ShapeNumber, ShapeInt, ShapeBool,
				ShapeString, ShapeCommaList, ShapeEnumOption:
				out = append(out, finaliseInlineKeyword(t, kw))
				i++
			default:
				out = append(out, finaliseInlineKeyword(t, kw))
				i++
			}
		case tokenRawLine:
			// Stale raw line outside a fence — should not happen given classifyLines' state machine.
			// Drop silently.
			i++
		case tokenDirective:
			// Go directives (//go:, //nolint:, …) are dropped from the stream — they have no role in the
			// swagger annotation grammar and must not contaminate TITLE / DESC.
			i++
		case TokenAnnotation:
			switch {
			case t.Name == labelDescription && isLiteralDescMarker(t):
				i = collectDescriptionLiteral(in, i, &out)
			case t.Name == labelDescription:
				i = collectDescriptionBody(in, i, &out)
			default:
				out = append(out, t)
				i++
			}
		case TokenBlank, tokenText, TokenEOF:
			out = append(out, t)
			i++
		default:
			out = append(out, t)
			i++
		}
	}
	out = append(out, Token{Kind: TokenEOF})
	return out
}

// collectDescriptionBody folds the contiguous prose lines following a `swagger:description`
// annotation into its argument (Option B, blank-line terminator): the body runs to the first blank
// line, keyword, annotation, or EOF — anything that is not a plain prose line ends it (so a
// following `maximum:` keyword or `swagger:*` annotation is never swallowed).
//
// The combined inline-plus-body text becomes the annotation's single raw arg, so AnnotationArg()
// returns the whole multi-line description and the folded lines never reach the prose (TITLE/DESC)
// surface.
// Returns the index past the folded lines.
//
// `swagger:title` is single-line and is not handled here.
func collectDescriptionBody(in []Token, i int, out *[]Token) int {
	ann := in[i]
	j := i + 1
	var body []string
	for j < len(in) && in[j].Kind == tokenText {
		body = append(body, strings.TrimSpace(in[j].Text))
		j++
	}
	if len(body) > 0 {
		ann.Args = []Token{combineDescriptionArg(ann, body)}
	}
	*out = append(*out, ann)
	return j
}

// collectDescriptionLiteral folds the verbatim body of a `swagger:description |` literal block into
// the annotation's single raw argument. classifyLines' literal mode has already emitted the body as
// a contiguous run of tokenRawLine (every source line after the marker, until the next annotation
// or EOF), so the body is preserved exactly — indentation, blank lines, table pipes, and `---`
// all intact.
//
// Trailing blank lines are clipped (bare-`|` semantics) and the `|` marker itself is dropped.
// Returns the index past the folded body.
func collectDescriptionLiteral(in []Token, i int, out *[]Token) int {
	ann := in[i]
	j := i + 1
	var body []string
	for j < len(in) && in[j].Kind == tokenRawLine {
		// Drop the single godoc convention space after `//` (gofmt-canonical `// text`); it is comment
		// decoration, not body.
		// Author indentation beyond it, trailing whitespace (markdown hard breaks), pipes, and blank
		// lines are all preserved verbatim.
		body = append(body, strings.TrimPrefix(in[j].Raw, " "))
		j++
	}
	for len(body) > 0 && strings.TrimSpace(body[len(body)-1]) == "" {
		body = body[:len(body)-1]
	}
	ann.Args = []Token{{Kind: TokenRawValue, Pos: ann.Pos, Text: strings.Join(body, "\n")}}
	*out = append(*out, ann)
	return j
}

// combineDescriptionArg joins the inline argument (if any) with the folded body lines using "\n"
// — the same join the prose Description() accumulator uses — preserving the inline arg's
// position when present.
func combineDescriptionArg(ann Token, body []string) Token {
	pos := ann.Pos
	var parts []string
	if len(ann.Args) > 0 && ann.Args[0].Text != "" {
		parts = append(parts, ann.Args[0].Text)
		pos = ann.Args[0].Pos
	}
	parts = append(parts, body...)
	return Token{Kind: TokenRawValue, Pos: pos, Text: strings.Join(parts, "\n")}
}

// collectFencedYAML scans from a `---` opener at index i and emits one OPAQUE_YAML token.
//
// The body is stored in Body (joined with "\n") and in Raw (verbatim, including indentation).
// Truncated is set on EOF without a closer.
// Returns the index past the closing fence (or the EOF position).
func collectFencedYAML(in []Token, i int, out *[]Token) int {
	openerPos := in[i].Pos
	i++
	var body, raw []string
	for i < len(in) {
		switch in[i].Kind {
		case tokenYAMLFence:
			*out = append(*out, Token{
				Kind:    TokenOpaqueYaml,
				Pos:     openerPos,
				Body:    strings.Join(body, "\n"),
				Raw:     strings.Join(raw, "\n"),
				Keyword: "",
			})
			return i + 1
		case tokenRawLine:
			body = append(body, in[i].Text)
			raw = append(raw, in[i].Raw)
			i++
		default:
			// Non-raw token shouldn't appear inside a fence — defensive.
			i++
		}
	}
	// EOF before closing fence — truncated body.
	*out = append(*out, Token{
		Kind:      TokenOpaqueYaml,
		Pos:       openerPos,
		Body:      strings.Join(body, "\n"),
		Raw:       strings.Join(raw, "\n"),
		Truncated: true,
	})
	return i
}

// collectRawBlock accumulates the body of a RAW_BLOCK_<KW> keyword (consumes / produces / responses
// / parameters / extensions / …).
//
// Stops at the next sibling structural item or EOF; blank lines do not terminate.
//
// # Details
//
// See README §raw-block-terminators for the sibling-terminator rule, the inline-value capture on
// the head, and the per-body indentation handling.
func collectRawBlock(in []Token, i int, kw Keyword, out *[]Token) int {
	head := in[i]
	headPos := head.Pos
	i++
	var bodyText, bodyRaw []string
	pendingBlanks := 0

	// Inline-value capture.
	// `Consumes: application/json` on a single line carries the value on head.Text; prepending it as
	// the first body line keeps the inline-plus-indented-continuation form working uniformly.
	// Without the prepend the post-colon payload would be silently lost.
	if head.Text != "" {
		bodyText = append(bodyText, head.Text)
		bodyRaw = append(bodyRaw, head.Text)
	}

	// extensions / infoExtensions / securityDefinitions / Tags / security bodies are YAML-parsed
	// downstream (yaml.TypedExtensions, yaml.UnmarshalBody via the meta walker, or security.Parse), so
	// every body line MUST preserve its original indentation — Tags in particular is a sequence of
	// mappings whose nesting collapses if the per-item indent is dropped, and a `Security:`
	// requirement with block-style scopes (`- name:` then indented `- scope`) needs the same.
	//
	// Flat raw blocks (consumes / produces / …) use the Text view (leading whitespace dropped,
	// recognised keywords reformatted).
	// Both branches converge on the same bodyText slice.
	yamlBody := kw.Name == "extensions" ||
		kw.Name == "infoExtensions" ||
		kw.Name == "securityDefinitions" ||
		kw.Name == KwTags ||
		kw.Name == KwSecurity ||
		kw.Name == KwExamples
	bodyLine := func(t Token) string {
		if yamlBody {
			return strings.TrimRightFunc(t.Raw, unicode.IsSpace)
		}
		return t.Text
	}

	consumed := func() {
		// flush any pending blanks into the body so visual separators inside list-shaped bodies survive.
		for range pendingBlanks {
			bodyText = append(bodyText, "")
			bodyRaw = append(bodyRaw, "")
		}
		pendingBlanks = 0
	}

	for i < len(in) {
		next := in[i]
		switch next.Kind {
		case TokenAnnotation:
			emitRawBlock(out, headPos, head, kw, bodyText, bodyRaw)
			return i
		case tokenKeywordPre:
			// Sibling structural keyword? — terminate.
			// Keywords that could legitimately appear inside the body (e.g. nested `default:` under a
			// `Parameters:` block) are absorbed as body text.
			//
			// Rule: same family / a sub-context keyword is body; another route/operation/meta-context
			// keyword is a sibling.
			//
			// Indentation override (YAML-bodied blocks only): inside a YAML body — Tags /
			// securityDefinitions / extensions — a same-family keyword indented strictly deeper than the
			// head is a nested YAML key, not a sibling (e.g. `externalDocs:` under a `Tags:` list item, both
			// meta-family).
			//
			// Absorb it so the YAML structure survives.
			// Flat raw blocks (TOS / consumes / …) do NOT apply this: their keyword indentation is
			// cosmetic — the petstore meta indents Schemes/Host deeper than a column-0 `Terms Of
			// Service:`, yet they are siblings.
			sibling := isSiblingTerminatorFor(kw, next.Name)
			if sibling && yamlBody &&
				leadingIndentWidth(next.Raw) > leadingIndentWidth(head.Raw) {
				sibling = false
			}
			if sibling {
				emitRawBlock(out, headPos, head, kw, bodyText, bodyRaw)
				return i
			}
			consumed()
			if yamlBody {
				bodyText = append(bodyText, strings.TrimRightFunc(next.Raw, unicode.IsSpace))
			} else {
				bodyText = append(bodyText, formatKeywordLine(next))
			}
			bodyRaw = append(bodyRaw, next.Raw)
			i++
		case tokenYAMLFence:
			// extensions blocks may decorate the body with a `---` fence; absorb its contents and drop the
			// fence markers.
			// See README §yaml-fence-handling.
			if kw.Name == "extensions" {
				i = absorbDecorativeFenceInto(in, i+1, &bodyText, &bodyRaw)
				continue
			}
			emitRawBlock(out, headPos, head, kw, bodyText, bodyRaw)
			return i
		case tokenText:
			consumed()
			bodyText = append(bodyText, bodyLine(next))
			bodyRaw = append(bodyRaw, next.Raw)
			i++
		case tokenRawLine:
			consumed()
			bodyText = append(bodyText, bodyLine(next))
			bodyRaw = append(bodyRaw, next.Raw)
			i++
		case tokenDirective:
			// Directives never contribute to body text.
			i++
		case TokenBlank:
			pendingBlanks++
			i++
		default:
			emitRawBlock(out, headPos, head, kw, bodyText, bodyRaw)
			return i
		}
	}

	emitRawBlock(out, headPos, head, kw, bodyText, bodyRaw)
	return i
}

// absorbDecorativeFenceInto consumes raw lines until the matching closing fence and appends them
// into the active body.
//
// Fences themselves are dropped.
// Returns the index past the closing fence (or len(in) on truncation).
func absorbDecorativeFenceInto(in []Token, i int, bodyText, bodyRaw *[]string) int {
	for i < len(in) {
		switch in[i].Kind {
		case tokenYAMLFence:
			return i + 1
		case tokenRawLine:
			*bodyText = append(*bodyText, in[i].Text)
			*bodyRaw = append(*bodyRaw, in[i].Raw)
		default:
			// ignored kind
		}
		i++
	}
	return i
}

// emitRawBlock writes one TokenRawBlockBody to out. headPos/head carry items-depth and source-name
// details forwarded onto the body token.
//
// A RAW_BLOCK has no closing delimiter — its body ends at the next sibling structural keyword or
// EOF — so there is no truncation condition (unlike OPAQUE_YAML, where a missing closing `---` is
// a real failure mode).
func emitRawBlock(out *[]Token, headPos token.Position, head Token, kw Keyword, body, raw []string) {
	*out = append(*out, Token{
		Kind:       TokenRawBlockBody,
		Pos:        headPos,
		Name:       kw.Name,
		SourceName: head.SourceName,
		Keyword:    kw.Name,
		Body:       strings.Join(body, "\n"),
		Raw:        strings.Join(raw, "\n"),
		ItemsDepth: head.ItemsDepth,
	})
}

// collectRawValue handles RAW_VALUE_<KW> body keywords (default / example / enum).
//
// Single-line case (head with non-empty inline value) emits one body token immediately; multi-line
// case scans subsequent lines until a sibling terminator.
func collectRawValue(in []Token, i int, kw Keyword, out *[]Token) int {
	head := in[i]
	headPos := head.Pos
	i++

	// Single-line trivial path.
	if head.Text != "" {
		*out = append(*out, Token{
			Kind:       TokenRawValueBody,
			Pos:        headPos,
			Name:       kw.Name,
			SourceName: head.SourceName,
			Keyword:    kw.Name,
			Body:       head.Text,
			Raw:        head.Raw,
			ItemsDepth: head.ItemsDepth,
		})
		return i
	}

	// Multi-line block-head path.
	var bodyText, bodyRaw []string
	pendingBlanks := 0
	consumed := func() {
		for range pendingBlanks {
			bodyText = append(bodyText, "")
			bodyRaw = append(bodyRaw, "")
		}
		pendingBlanks = 0
	}

	for i < len(in) {
		next := in[i]
		switch next.Kind {
		case TokenAnnotation:
			emitRawValue(out, headPos, head, kw, bodyText, bodyRaw)
			return i
		case tokenKeywordPre:
			if isSiblingTerminatorFor(kw, next.Name) {
				emitRawValue(out, headPos, head, kw, bodyText, bodyRaw)
				return i
			}
			consumed()
			bodyText = append(bodyText, formatKeywordLine(next))
			bodyRaw = append(bodyRaw, next.Raw)
			i++
		case tokenYAMLFence:
			emitRawValue(out, headPos, head, kw, bodyText, bodyRaw)
			return i
		case tokenText:
			consumed()
			bodyText = append(bodyText, next.Text)
			bodyRaw = append(bodyRaw, next.Raw)
			i++
		case tokenDirective:
			// Directives never contribute to body text.
			i++
		case TokenBlank:
			pendingBlanks++
			i++
		default:
			emitRawValue(out, headPos, head, kw, bodyText, bodyRaw)
			return i
		}
	}

	emitRawValue(out, headPos, head, kw, bodyText, bodyRaw)
	return i
}

func emitRawValue(out *[]Token, headPos token.Position, head Token, kw Keyword, body, raw []string) {
	*out = append(*out, Token{
		Kind:       TokenRawValueBody,
		Pos:        headPos,
		Name:       kw.Name,
		SourceName: head.SourceName,
		Keyword:    kw.Name,
		Body:       strings.Join(body, "\n"),
		Raw:        strings.Join(raw, "\n"),
		ItemsDepth: head.ItemsDepth,
	})
}

// formatKeywordLine recreates the textual `<name>: <value>` line for a keyword token absorbed into
// a raw body — line-preserving rendering for downstream consumers that read the body as text.
func formatKeywordLine(t Token) string {
	name := t.SourceName
	if name == "" {
		name = t.Name
	}
	if t.Text == "" {
		return name + ":"
	}
	return name + ": " + t.Text
}

// isSiblingTerminatorFor decides whether a keyword named `next`, encountered while accumulating a
// body opened by `kw`, is a sibling structural terminator (true) or a sub-context keyword that
// should be absorbed as body text (false).
//
// Rule:
//
//   - if kw is a meta/route/operation context block (consumes, produces,
//     security, securityDefinitions, responses, parameters, extensions,
//     externalDocs, infoExtensions, tos, schemes), terminate on any
//     sibling that is also a meta/route/operation-context keyword;
//   - if kw is a schema body keyword (default, example, enum),
//     terminate on any sibling that is a schema-context keyword.
//
// Look-up uses the keyword table's Contexts.
// See README §raw-block-terminators. tabStopWidth is the column width a tab advances to when
// measuring leading indentation — the conventional 8-column tab stop.
const tabStopWidth = 8

// leadingIndentWidth measures the visual width of raw's leading whitespace run, expanding tabs to
// 8-column tab stops and counting spaces as one column each.
//
// Used by the raw-block terminator to tell a nested YAML key (indented deeper than its block head)
// from a true sibling keyword at the same indentation.
// Non-whitespace ends the run.
func leadingIndentWidth(raw string) int {
	w := 0
	for _, r := range raw {
		switch r {
		case ' ':
			w++
		case '\t':
			w += tabStopWidth - (w % tabStopWidth)
		default:
			return w
		}
	}
	return w
}

func isSiblingTerminatorFor(kw Keyword, nextName string) bool {
	nextKw, ok := Lookup(nextName)
	if !ok {
		return false
	}
	headFamily := familyOf(kw)
	nextFamily := familyOf(nextKw)
	for _, hf := range headFamily {
		if slices.Contains(nextFamily, hf) {
			return true
		}
	}
	return false
}

// familyOf classifies a keyword into one or more "family" buckets per its declared contexts.
func familyOf(kw Keyword) []KeywordContext {
	out := make([]KeywordContext, 0, len(kw.Contexts))
	for _, c := range kw.Contexts {
		switch c {
		case CtxMeta, CtxRoute, CtxOperation:
			out = append(out, c)
		case CtxSchema, CtxItems, CtxParam, CtxHeader, CtxResponse:
			out = append(out, c)
		default:
			// ignored context
		}
	}
	return out
}

// finaliseInlineKeyword converts a tokenKeywordPre into a TokenKeyword carrying the lexer-typed
// value via its Args field.
//
// Emitting a single TokenKeyword (rather than two adjacent tokens) keeps the body accumulator's
// output atomic — exactly one token per keyword regardless of how many sub-tokens the value
// carries.
// The parser unpacks Args to read the typed value.
func finaliseInlineKeyword(t Token, kw Keyword) Token {
	value := t.Text
	valuePos := t.Pos
	valuePos.Column += len(t.SourceName) + 1
	valuePos.Offset += len(t.SourceName) + 1

	var argTok Token
	switch kw.Shape {
	case ShapeNumber:
		argTok = Token{Kind: TokenNumberValue, Pos: valuePos, Text: value}
	case ShapeInt:
		argTok = Token{Kind: TokenIntValue, Pos: valuePos, Text: value}
	case ShapeBool:
		argTok = Token{Kind: TokenBoolValue, Pos: valuePos, Text: value}
	case ShapeString:
		argTok = Token{Kind: TokenStringValue, Pos: valuePos, Text: value}
	case ShapeCommaList:
		argTok = Token{Kind: TokenCommaListValue, Pos: valuePos, Text: value}
	case ShapeEnumOption:
		argTok = Token{Kind: TokenEnumOption, Pos: valuePos, Text: value}
	case ShapeNone, ShapeRawBlock, ShapeRawValue:
		// Body keywords reach finaliseInlineKeyword only on the pathological "head with no inline value
		// but no following body" case.
		// Treat the value, if any, as a string token.
		argTok = Token{Kind: TokenStringValue, Pos: valuePos, Text: value}
	default:
		// ignored shape
	}

	return Token{
		Kind:       TokenKeyword,
		Pos:        t.Pos,
		Name:       kw.Name,
		SourceName: t.SourceName,
		Text:       value,
		Raw:        t.Raw,
		ItemsDepth: t.ItemsDepth,
		Args:       []Token{argTok},
	}
}

// ----- Stage 3 — prose classifier -------------------------------------------

// classifyProse re-types tokenText tokens as TITLE / DESC.
//
// The function preserves all non-text tokens and the relative order of text tokens.
// Blank tokens within a prose run are preserved so downstream consumers can reproduce paragraph
// structure.
//
// # Details
//
// See README §prose-classification for the four heuristics and the rationale for applying them to
// unbound (no-annotation) comments as well as annotated ones.
func classifyProse(in []Token) []Token {
	hasAnnotation := false
	for _, t := range in {
		if t.Kind == TokenAnnotation {
			hasAnnotation = true
			break
		}
	}

	out := make([]Token, 0, len(in))
	state := proseStart
	for _, t := range in {
		if t.Kind != tokenText && t.Kind != TokenBlank {
			out = append(out, t)
			if t.Kind == TokenAnnotation {
				state = proseAfterAnnotation
			} else {
				state = proseInBody
			}
			continue
		}
		// Drop Kubernetes-style marker comments (`+kubebuilder:…`, `+genclient`, `+k8s:…`) from the
		// prose surface so they never leak into model / property descriptions (go-swagger#2687, the
		// residual of #3007).
		//
		// Done here (Stage 3) rather than at line classification so annotation bodies are untouched —
		// the inline swagger:route parameters grammar uses `+name:` as a parameter separator
		// (go-swagger#3100), and by this stage that body has already been folded into its keyword token
		// by accumulateBodies.
		if t.Kind == tokenText && isDirectiveMarker(t.Text) {
			continue
		}
		// Buffer prose runs; the run-classifier runs at run-end.
		out = append(out, t)
		_ = state
	}

	// Always classify — UnboundBlock-style comments (no swagger annotation) still need title/desc
	// classification because the schema builder consumes their PreambleTitle/PreambleDescription when
	// an interface or alias is referenced indirectly.
	return classifyProseRunsInPlace(out, hasAnnotation)
}

type proseState int

const (
	proseStart proseState = iota
	proseAfterAnnotation
	proseInBody
)

// classifyProseRunsInPlace walks `out` and re-types contiguous runs of (tokenText / TokenBlank)
// into TITLE / DESC.
//
// The first prose run is split into title + description; later prose runs become DESC.
//
// The annotation flag is no longer consulted — heuristics fire on UnboundBlock-style comments (no
// swagger annotation) too, because such comments render as schemas through indirect references
// (e.g. a non-annotated interface embedded by a swagger:model parent) and the consumer still wants
// the title/description split.
func classifyProseRunsInPlace(out []Token, _ bool) []Token {
	firstRun := true
	for i := 0; i < len(out); {
		if out[i].Kind != tokenText && out[i].Kind != TokenBlank {
			i++
			continue
		}
		j := i
		for j < len(out) && (out[j].Kind == tokenText || out[j].Kind == TokenBlank) {
			j++
		}
		if firstRun {
			classifyTitleDescRun(out, i, j)
		} else {
			retypeRunAs(out, i, j, TokenDesc)
		}
		firstRun = false
		i = j
	}
	return out
}

// classifyTitleDescRun applies the four prose heuristics to a single contiguous prose run [start,
// end).
//
// See README §prose-classification.
func classifyTitleDescRun(out []Token, start, end int) {
	// Find the first text-line index inside the run.
	firstText := -1
	for k := start; k < end; k++ {
		if out[k].Kind == tokenText {
			firstText = k
			break
		}
	}
	if firstText == -1 {
		// Run is all blanks.
		retypeRunAs(out, start, end, TokenDesc)
		return
	}

	// Heuristic 1: blank inside the run splits title (before) / desc (after).
	// Only fires when the blank has text AFTER it — a trailing blank is a separator between the
	// prose run and the next non-prose token (annotation / EOF), not an internal title/desc divide.
	//
	// On a heuristic-1 split, also strip an ATX heading marker from the first title line so the
	// rendered title doesn't carry the `#`+ prefix.
	for k := firstText + 1; k < end; k++ {
		if out[k].Kind != TokenBlank {
			continue
		}
		hasTextAfter := false
		for m := k + 1; m < end; m++ {
			if out[m].Kind == tokenText {
				hasTextAfter = true
				break
			}
		}
		if !hasTextAfter {
			continue
		}
		if rest, ok := stripATXHeading(out[firstText].Text); ok {
			out[firstText].Text = rest
		}
		retypeRunAs(out, start, k, TokenTitle)
		retypeRunAs(out, k, end, TokenDesc)
		return
	}

	// Heuristic 2: first prose line ends with Unicode punctuation -> title is line 1.
	first := out[firstText].Text
	if endsWithPunct(first) {
		retypeRunAs(out, start, firstText+1, TokenTitle)
		retypeRunAs(out, firstText+1, end, TokenDesc)
		return
	}

	// Heuristic 3: first line matches a markdown ATX heading -> strip marker, title is line 1.
	if rest, ok := stripATXHeading(first); ok {
		out[firstText].Text = rest
		retypeRunAs(out, start, firstText+1, TokenTitle)
		retypeRunAs(out, firstText+1, end, TokenDesc)
		return
	}

	// Heuristic 4: entire run becomes description.
	retypeRunAs(out, start, end, TokenDesc)
}

// retypeRunAs re-types the (text, blank) tokens in [start, end) so that text becomes `kind`.
//
// Blanks are preserved (kept as TokenBlank) because consumers may want paragraph breaks intact
// between TITLE / DESC runs.
func retypeRunAs(out []Token, start, end int, kind TokenKind) {
	for k := start; k < end; k++ {
		if out[k].Kind == tokenText {
			out[k].Kind = kind
		}
	}
}

// stripATXHeading recognises a markdown ATX-style heading prefix — one or more leading `#`
// followed by at least one whitespace character — and returns the trimmed remainder.
//
// Reports false when the input doesn't open with `#`.
// Replaces a regexp.
func stripATXHeading(s string) (rest string, ok bool) {
	i := 0
	for i < len(s) && s[i] == '#' {
		i++
	}
	if i == 0 {
		return s, false
	}
	// Need at least one whitespace separator between the # run and the heading text.
	if i >= len(s) {
		return s, false
	}
	switch s[i] {
	case ' ', '\t', '\n', '\f', '\r', '\v':
	default:
		return s, false
	}
	return strings.TrimSpace(s[i+1:]), true
}

// endsWithPunct reports whether s ends in Unicode punctuation other than dash/connector —
// implementation looks for category Po ("punctuation, other") on the last rune.
func endsWithPunct(s string) bool {
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	if s == "" {
		return false
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	return unicode.Is(unicode.Po, r)
}

// FormatToken renders a token compactly for diagnostics and tests.
//
// Avoids leaking internal kinds in production output.
func FormatToken(t Token) string {
	switch t.Kind {
	case TokenAnnotation:
		return "ANN(" + t.Name + argSummary(t.Args) + ")"
	case TokenKeyword:
		return "KW(" + t.Name + argSummary(t.Args) + ")"
	case TokenRawBlockBody:
		return "RAW_BLOCK_" + strings.ToUpper(t.Keyword)
	case TokenRawValueBody:
		return "RAW_VALUE_" + strings.ToUpper(t.Keyword)
	case TokenOpaqueYaml:
		return "OPAQUE_YAML"
	default:
		if t.Text != "" {
			return t.Kind.String() + "(" + strconv.Quote(t.Text) + ")"
		}
		return t.Kind.String()
	}
}

func argSummary(args []Token) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = a.Kind.String()
	}
	return ":" + strings.Join(parts, ",")
}
