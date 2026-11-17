// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package resolvers

import oaispec "github.com/go-openapi/spec"

// ExtEnumDesc is the vendor-extension key the scanner uses to expose
// the per-enum-value documentation lines built from `swagger:enum`
// + const-block comments. It's a go-swagger concept (not part of
// the OpenAPI spec); the key lives in `resolvers` so every builder
// (schema, parameters, responses) reads and writes it through the
// same constant.
const ExtEnumDesc = "x-go-enum-desc"

// GetEnumDesc reads the x-go-enum-desc extension off a Swagger extensions map.
//
// Empty when absent.
//
// Consumers typically check this after a build pass to know whether they should append the
// per-value docs to a Description (parameters do this for the parameter description; the schema
// builder folds it in differently — see `handlers/dispatch_schema.go:clearStaleEnumDesc` for the
// override-cleanup dance).
//
// Response headers carry the mapping on x-go-enum-desc but never fold it into the header
// description.
func GetEnumDesc(extensions oaispec.Extensions) string {
	desc, _ := extensions.GetString(ExtEnumDesc)
	return desc
}

// AppendEnumDesc folds the x-go-enum-desc const-name mapping (if any) into description, returning
// the resulting description.
//
// A newline separates the authored prose from the appended mapping.
//
// When skip is true the description is returned unchanged — the mapping then rides x-go-enum-desc
// only.
//
// This is the single gate shared by the schema (model decl + struct field) and parameter builders
// so the SkipEnumDescriptions option behaves identically across every target that folds the mapping
// in.
//
// Response headers never fold the mapping into their description, so they don't call this — but
// they do carry it on x-go-enum-desc in both modes (spec >= v0.22.6 marshals header vendor
// extensions).
// See go-swagger/go-swagger#2922.
func AppendEnumDesc(description string, extensions oaispec.Extensions, skip bool) string {
	if skip {
		return description
	}
	enumDesc := GetEnumDesc(extensions)
	if enumDesc == "" {
		return description
	}
	if description != "" {
		description += "\n"
	}
	return description + enumDesc
}
