// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package routebody

import (
	"errors"
	"fmt"
	"go/token"
	"slices"
	"strconv"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
)

// errInvalidValue is the sentinel wrapped by buildProperty when a validation property's raw value
// fails to parse against the keyword's declared shape.
//
// The caller (applyParamLine) catches it and forwards the wrapping message as a
// CodeInvalidAnnotation diagnostic.
var errInvalidValue = errors.New("invalid value")

// ParamDecl is one parsed `+ name:`-delimited chunk from a swagger:route `Parameters:` body.
//
// Head fields are routebody-owned (the orchestrator reads them directly to populate the
// *spec.Parameter shell).
// Validation fields land on Block as grammar.Property entries that the orchestrator dispatches via
// the standard handlers seam — see package godoc for the field split.
type ParamDecl struct {
	Name        string
	In          string
	TypeRef     string
	Format      string
	Description string
	Required    bool
	AllowEmpty  bool
	Block       grammar.Block
	Pos         token.Position
}

// chunkParseState tracks the in-flight param chunk while iterating lines.
//
// The state machine: a `+` or `-` line opens a new chunk, subsequent lines fill its fields, the
// next `+`/`-` (or end-of-body) commits the current chunk.
type chunkParseState struct {
	cur     *ParamDecl
	props   []grammar.Property
	basePos token.Position
}

// ParseParameters lowers a Parameters: raw block body into typed param chunks.
//
// See package godoc for the grammar spec.
//
// basePos is the source position of the `parameters:` keyword head; each chunk's Pos is offset by
// the chunk's line number within body (1-indexed) so diagnostics point at the offending line in the
// original source.
//
// diag may be nil; when nil, diagnostics are dropped.
func ParseParameters(body string, basePos token.Position, diag func(grammar.Diagnostic)) []ParamDecl {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	var out []ParamDecl
	state := chunkParseState{basePos: basePos}
	lines := strings.Split(body, "\n")

	for i, raw := range lines {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		pos := offsetPos(basePos, lineNo)

		// Chunk-start sigil: `+ ` or `- ` (alias).
		// The sigil itself may be the entire line (bare `+` / `-`) or be followed by a `key: value` on
		// the same line.
		if isChunkSigil(line) {
			commitChunk(&state, &out, diag)
			state.cur = &ParamDecl{Pos: pos}
			state.props = nil
			line = strings.TrimSpace(line[1:])
			if line == "" {
				continue
			}
		}

		// Lines without a `:` are silently ignored, as are lines whose key trims to empty.
		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}

		if state.cur == nil {
			emitDiagf(diag, pos,
				"parameter property %q outside any chunk; expected `+ name:` chunk-start first", key)
			continue
		}

		applyParamLine(state.cur, &state.props, key, value, pos, diag)
	}

	commitChunk(&state, &out, diag)
	return out
}

// isChunkSigil reports whether line begins with the `+ ` (canonical) or `- ` (alias) chunk-start
// sigil.
//
// The sigil is the very first character — leading whitespace was trimmed by the caller.
func isChunkSigil(line string) bool {
	if line == "" {
		return false
	}
	return line[0] == '+' || line[0] == '-'
}

// splitKeyValue splits one `key: value` line on the first colon.
//
// Returns (key, value, true) when both halves are non-empty after trimming; (_, _, false)
// otherwise.
func splitKeyValue(line string) (key, value string, ok bool) {
	before, after, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	key = strings.TrimSpace(before)
	value = strings.TrimSpace(after)
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

// applyParamLine dispatches one `key: value` line onto the current chunk.
//
// Head keys land directly on cur; validation keys are lowered to grammar.Property entries on props
// (the eventual ParamDecl.Block).
func applyParamLine(cur *ParamDecl, props *[]grammar.Property, key, value string, pos token.Position, diag func(grammar.Diagnostic)) {
	switch strings.ToLower(key) {
	case "name":
		cur.Name = value
	case "in":
		canonical, ok := grammar.NormalizeIn(value, true)
		if !ok {
			emitDiagf(diag, pos,
				"in: %q is not one of path/query/header/body/formData", value)
			return
		}
		// allowFormAlias=true accepts the v1 routes affordance `form` and canonicalises to `formData`.
		// See Q27 — the alias is contained to this parser; no other capture site passes
		// allowFormAlias=true.
		cur.In = canonical
	case "type":
		cur.TypeRef = value
	case "format":
		cur.Format = value
	case "description":
		cur.Description = value
	case "required":
		v, err := strconv.ParseBool(value)
		if err != nil {
			emitDiagf(diag, pos,
				"required: %q is not a valid boolean (true/false)", value)
			return
		}
		cur.Required = v
	case "allowempty", "allowemptyvalue":
		v, err := strconv.ParseBool(value)
		if err != nil {
			emitDiagf(diag, pos,
				"allowempty: %q is not a valid boolean (true/false)", value)
			return
		}
		cur.AllowEmpty = v
	default:
		// Validation property: look up in the grammar keyword table.
		// Unknown keys emit CodeInvalidAnnotation rather than being dropped silently.
		kw, ok := grammar.Lookup(key)
		if !ok {
			emitDiagf(diag, pos,
				"unknown parameter keyword %q", key)
			return
		}
		p, err := buildProperty(kw, value, pos)
		if err != nil {
			emitDiagf(diag, pos, "%s", err.Error())
			return
		}
		*props = append(*props, p)
	}
}

// commitChunk finalises the in-flight chunk (if any), builds its validation Block, and appends to
// out.
//
// A bare `+`/`-` sigil with no key:value follow-up (no name, no other head fields, no properties)
// emits CodeInvalidAnnotation and is dropped rather than being committed as an empty parameter.
func commitChunk(state *chunkParseState, out *[]ParamDecl, diag func(grammar.Diagnostic)) {
	if state.cur == nil {
		return
	}
	cur := state.cur
	if isEmptyChunk(cur, state.props) {
		emitDiagf(diag, cur.Pos,
			"empty parameter chunk: `+` / `-` requires at least `name:` and `in:` follow-up")
		state.cur = nil
		state.props = nil
		return
	}
	cur.Block = grammar.NewSyntheticBlock(cur.Pos, cur.Name, cur.Description, state.props)
	*out = append(*out, *cur)
	state.cur = nil
	state.props = nil
}

// isEmptyChunk reports whether the in-flight chunk has nothing the orchestrator could use — no
// name, no in, no type, no description, no validations.
//
// A chunk with just `required: true` and nothing else is still considered empty (no name → no
// usable param).
func isEmptyChunk(cur *ParamDecl, props []grammar.Property) bool {
	return cur.Name == "" && cur.In == "" && cur.TypeRef == "" &&
		cur.Format == "" && cur.Description == "" && len(props) == 0
}

// buildProperty constructs a grammar.Property from one validation key/value pair, populating Typed
// when the keyword's Shape demands it.
//
// Returns an error when typing fails (caller surfaces it as a diagnostic with the keyword name
// attached).
func buildProperty(kw grammar.Keyword, raw string, pos token.Position) (grammar.Property, error) {
	p := grammar.Property{
		Keyword: kw,
		Pos:     pos,
		Value:   raw,
	}
	switch kw.Shape {
	case grammar.ShapeNumber:
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return p, fmt.Errorf("%w: %s: %q is not a valid number", errInvalidValue, kw.Name, raw)
		}
		p.Typed = grammar.TypedValue{Type: grammar.ShapeNumber, Number: v}
	case grammar.ShapeInt:
		v, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return p, fmt.Errorf("%w: %s: %q is not a valid integer", errInvalidValue, kw.Name, raw)
		}
		p.Typed = grammar.TypedValue{Type: grammar.ShapeInt, Integer: v}
	case grammar.ShapeBool:
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return p, fmt.Errorf("%w: %s: %q is not a valid boolean", errInvalidValue, kw.Name, raw)
		}
		p.Typed = grammar.TypedValue{Type: grammar.ShapeBool, Boolean: v}
	case grammar.ShapeEnumOption:
		// Closed-vocab string-enum (e.g. collectionFormat).
		// Set Typed only when raw is in the allowed set; otherwise leave ShapeNone so the dispatcher's
		// IsTyped() check drops the write while the handler-side string fallback recovers the raw value
		// where supported (handlers.CollectionFormatString reads pr.Value when Typed is empty).
		if slices.Contains(kw.Values, raw) {
			p.Typed = grammar.TypedValue{Type: grammar.ShapeEnumOption, String: raw}
		}
	case grammar.ShapeNone, grammar.ShapeString, grammar.ShapeCommaList,
		grammar.ShapeRawBlock, grammar.ShapeRawValue:
		// Raw / string / comma-list keywords carry their value through Property.Value unchanged; the
		// dispatcher's Raw and String callbacks read it from there and coerce per the resolved schema
		// type at write time.
	default:
		// Unknown shape — future grammar additions.
		// Keep the raw value on Property.Value; the dispatcher will treat it as untyped.
	}
	return p, nil
}
