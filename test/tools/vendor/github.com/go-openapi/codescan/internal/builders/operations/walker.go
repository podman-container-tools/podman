// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package operations

import (
	"github.com/go-openapi/codescan/internal/parsers/grammar"
	"github.com/go-openapi/codescan/internal/parsers/yaml"
	oaispec "github.com/go-openapi/spec"
)

// applyBlockToOperation parses path.Remaining through grammar and writes Summary / Description /
// YAML body content onto op.
//
// # Details
//
// See [§walker](./README.md#walker) for the three-step bridge from the lexer-classified Block onto
// op and the single-fenced-body contract.
func (o *Builder) applyBlockToOperation(op *oaispec.Operation) error {
	block := grammar.NewParser(o.Ctx.FileSet(),
		grammar.WithSingleLineCommentAsDescription(o.Ctx.SingleLineCommentAsDescription())).Parse(o.path.Remaining)

	op.Summary = o.CleanGoDoc(block.Title())
	op.Description = o.CleanGoDoc(block.Description())

	for y := range block.YAMLBlocks() {
		// Parameters already bound to this operation by a swagger:parameters struct must survive the
		// inline YAML unmarshal as distinct entries. encoding/json reuses the existing slice elements
		// when decoding the inline `parameters:` array, which would weld a bound body's $ref schema onto
		// an inline parameter (go-swagger#2651).
		//
		// Decode the inline body into a fresh slice, then merge the bound parameters back.
		bound := op.Parameters
		op.Parameters = nil
		if err := yaml.UnmarshalBody(y.Text, op.UnmarshalJSON); err != nil {
			return err
		}
		op.Parameters = mergeBoundParameters(op.Parameters, bound)
		return nil
	}
	return nil
}

// mergeBoundParameters appends parameters bound by a swagger:parameters struct to the inline
// swagger:operation parameters, skipping any bound parameter already declared inline.
//
// Collisions are matched on (name, location); body parameters collide on location alone (an
// operation has at most one body).
// Inline declarations win on conflict.
func mergeBoundParameters(inline, bound []oaispec.Parameter) []oaispec.Parameter {
	if len(bound) == 0 {
		return inline
	}

	out := inline
	for _, b := range bound {
		duplicate := false
		for i := range inline {
			if parametersCollide(inline[i], b) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			out = append(out, b)
		}
	}
	return out
}

// parametersCollide reports whether two parameters occupy the same OAS v2 slot: same name and
// location, or both the (singleton) body parameter.
func parametersCollide(a, b oaispec.Parameter) bool {
	if a.In == "body" || b.In == "body" {
		return a.In == "body" && b.In == "body"
	}
	return a.In == b.In && a.Name == b.Name
}
