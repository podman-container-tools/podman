// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import "go/token"

// TokenKind classifies a lexer-emitted token.
//
// The kinds map onto the grammar's terminal vocabulary:
//
//   - ANN_*  → TokenAnnotation,    Name carries the annotation label
//   - KW_*   → TokenKeyword,       Name carries the keyword label
//   - IDENT_NAME, JSON_VALUE, RAW_VALUE, TYPE_REF, HTTP_METHOD,
//     URL_PATH → distinct kinds, payload in Text
//   - NUMBER_VALUE / INT_VALUE / BOOL_VALUE / STRING_VALUE /
//     COMMA_LIST_VALUE / ENUM_OPTION_VALUE → distinct kinds
//   - RAW_BLOCK_<KW> → TokenRawBlockBody with Keyword = "consumes"/"produces"/…
//   - RAW_VALUE_<KW> → TokenRawValueBody with Keyword = "default"/"example"/"enum"
//   - OPAQUE_YAML → TokenOpaqueYaml
//   - TITLE / DESC / BLANK / EOF → TokenTitle / TokenDesc / TokenBlank / TokenEOF
//
// The lexer also emits a few internal kinds (tokenYAMLFence, tokenText, tokenKeywordPre,
// tokenRawLine, tokenDirective) that are consumed by intermediate stages and never appear in the
// final stream the parser consumes.
type TokenKind int

const (
	TokenEOF TokenKind = iota

	TokenBlank
	TokenTitle
	TokenDesc

	TokenAnnotation     // ANN_*  — Name = annotation label
	TokenKeyword        // KW_*   — Name = keyword label
	TokenIdentName      // IDENT_NAME
	TokenJSONValue      // JSON_VALUE
	TokenRawValue       // RAW_VALUE
	TokenTypeRef        // TYPE_REF
	TokenHTTPMethod     // HTTP_METHOD
	TokenURLPath        // URL_PATH
	TokenWildcard       // WILDCARD — the `*` shared-namespace target (swagger:parameters)
	TokenNumberValue    // NUMBER_VALUE
	TokenIntValue       // INT_VALUE
	TokenBoolValue      // BOOL_VALUE
	TokenStringValue    // STRING_VALUE
	TokenCommaListValue // COMMA_LIST_VALUE
	TokenEnumOption     // ENUM_OPTION_VALUE
	TokenRawBlockBody   // RAW_BLOCK_<KW>  — Name = parent keyword
	TokenRawValueBody   // RAW_VALUE_<KW>  — Name = parent keyword
	TokenOpaqueYaml     // OPAQUE_YAML

	// Internal kinds used between pipeline stages; never reach the parser.
	tokenYAMLFence  // "---" delimiter recognised by the line classifier
	tokenText       // free-form prose line (re-typed to TITLE/DESC by prose classifier)
	tokenKeywordPre // tentative keyword line (head + value-string), pre body-accumulation
	tokenRawLine    // raw line accumulated inside a YAML fence or raw block
	tokenDirective  // Go compiler/linter directive (//go:, //nolint:, …) — dropped before parsing
)

// String renders a TokenKind for diagnostics.
func (k TokenKind) String() string {
	switch k {
	case TokenEOF:
		return "EOF"
	case TokenBlank:
		return "BLANK"
	case TokenTitle:
		return "TITLE"
	case TokenDesc:
		return "DESC"
	case TokenAnnotation:
		return "ANN"
	case TokenKeyword:
		return "KW"
	case TokenIdentName:
		return "IDENT_NAME"
	case TokenJSONValue:
		return "JSON_VALUE"
	case TokenRawValue:
		return "RAW_VALUE"
	case TokenTypeRef:
		return "TYPE_REF"
	case TokenHTTPMethod:
		return "HTTP_METHOD"
	case TokenURLPath:
		return "URL_PATH"
	case TokenWildcard:
		return "WILDCARD"
	case TokenNumberValue:
		return "NUMBER_VALUE"
	case TokenIntValue:
		return "INT_VALUE"
	case TokenBoolValue:
		return "BOOL_VALUE"
	case TokenStringValue:
		return "STRING_VALUE"
	case TokenCommaListValue:
		return "COMMA_LIST_VALUE"
	case TokenEnumOption:
		return "ENUM_OPTION"
	case TokenRawBlockBody:
		return "RAW_BLOCK"
	case TokenRawValueBody:
		return "RAW_VALUE_BODY"
	case TokenOpaqueYaml:
		return "OPAQUE_YAML"
	case tokenYAMLFence:
		return "yaml-fence"
	case tokenText:
		return "text"
	case tokenKeywordPre:
		return "kw-pre"
	case tokenRawLine:
		return "raw-line"
	case tokenDirective:
		return "directive"
	default:
		return "?"
	}
}

// Token is one lexer-emitted item.
//
// Field population varies by kind:
//
//   - TokenAnnotation: Name = annotation label, Args are pre-tokenised
//     argument tokens already disambiguated by the lexer
//     (IDENT_NAME, JSON_VALUE, RAW_VALUE, TYPE_REF, HTTP_METHOD,
//     URL_PATH, etc. — see README §annotation-args).
//   - TokenKeyword: Name = canonical keyword label; SourceName = the
//     literal as it appeared (case / alias preserved); ItemsDepth =
//     number of leading "items." segments stripped.
//   - TokenRawBlockBody / TokenRawValueBody: Keyword = the parent
//     keyword; Body = body content joined by "\n"; Raw = verbatim
//     source-indented content; ItemsDepth carried from the head if
//     applicable.
//   - TokenOpaqueYaml: Body = body content (between fences).
//   - TokenIdentName / value-typed tokens: Text = payload string.
//   - TokenTitle / TokenDesc: Text = prose line content.
//
// Pos is the source position of the first meaningful payload character.
type Token struct {
	Kind TokenKind
	Pos  token.Position

	Name       string  // for TokenAnnotation / TokenKeyword: canonical label
	SourceName string  // keyword: literal as written (alias / case preserved)
	Text       string  // payload for value tokens, prose lines, idents
	Args       []Token // for TokenAnnotation: positional argument tokens
	ItemsDepth int     // leading "items." depth (keyword tokens only)
	Keyword    string  // for body tokens: parent keyword name
	Body       string  // for body tokens: body content joined by "\n"
	Raw        string  // verbatim source content (indentation preserved)
	Truncated  bool    // body lexed without a closer (e.g. unmatched ---)
}

// HasArg reports whether the annotation token has at least n positional arguments.
//
// Convenience used by the parser dispatchers.
func (t Token) HasArg(n int) bool { return len(t.Args) >= n }
