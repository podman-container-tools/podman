// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"go/types"

	"github.com/go-openapi/codescan/internal/ifaces"
	oaispec "github.com/go-openapi/spec"
)

// Option to build a schema.
type Option func(*options)

type options struct {
	definitions  map[string]oaispec.Schema
	inputType    types.Type
	target       ifaces.SwaggerTypable
	simpleSchema bool
	paramIn      string
	path         string // base JSON pointer for cross-ref provenance (empty = off)
}

// WithPath sets the base RFC 6901 pointer this build emits provenance under (cross-ref linkage).
//
// The caller initiates it for the placement context (e.g. "/definitions/User" or
// "/paths/~1pets/get/responses/200/schema"); the builder path-joins each member it produces.
// Empty (the default) records nothing.
func WithPath(base string) Option {
	return func(o *options) {
		o.path = base
	}
}

// WithDefinitions selects the "definitions" Build mode.
//
// The builder emits the top-level schema for the bound EntityDecl into the supplied map keyed by
// `s.Name`.
func WithDefinitions(definitions map[string]oaispec.Schema) Option {
	return func(o *options) {
		o.definitions = definitions
	}
}

// WithType selects the "typed target" Build mode (full Schema).
//
// The builder writes the schema for tpe into the caller-owned target.
// Used for body parameters, response bodies, and any other site that produces a full OAS v2 Schema.
func WithType(tpe types.Type, tgt ifaces.SwaggerTypable) Option {
	return func(o *options) {
		o.inputType = tpe
		o.target = tgt
	}
}

// WithSimpleSchema selects the SimpleSchema Build mode for OAS v2 parameter / response-header sites
// where `in` is not `body`. tpe is the Go type; tgt is the caller-owned SimpleSchema-shaped target
// (typically paramTypable or headerTypable); in carries the parameter location string ("query" /
// "path" / "header" / "formData", or empty for response headers).
//
// # Details
//
// See [§simple-schema-mode](./README.md#simple-schema-mode) — the allowed keyword surface, the
// catch-at-exit contract, the SimpleSchemaProbe interface, and the rules that drive the
// file/allowEmptyValue special cases.
func WithSimpleSchema(tpe types.Type, tgt ifaces.SwaggerTypable, in string) Option {
	return func(o *options) {
		o.inputType = tpe
		o.target = tgt
		o.simpleSchema = true
		o.paramIn = in
	}
}

// OptionFor picks the right Build mode based on the typable's location: WithType when the target is
// a body schema, WithSimpleSchema otherwise.
//
// The parameters and responses builders both call this at every field-build site; the body /
// non-body split is the single discriminator they rely on.
//
// Centralised here so the dispatch is uniform — adding a third mode or refining the gate becomes
// a one-place edit.
func OptionFor(tpe types.Type, tgt ifaces.SwaggerTypable) Option {
	if tgt.In() == "body" {
		return WithType(tpe, tgt)
	}
	return WithSimpleSchema(tpe, tgt, tgt.In())
}

func optionsWithDefaults(opts []Option) options {
	var o options

	for _, apply := range opts {
		apply(&o)
	}

	return o
}
