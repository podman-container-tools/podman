// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package spec

import (
	"encoding/json"
	"strings"

	"github.com/go-openapi/codescan/internal/parsers/grammar"
	yamlparser "github.com/go-openapi/codescan/internal/parsers/yaml"
	"github.com/go-openapi/spec"
)

// applyMetaBlock dispatches one parsed swagger:meta block into the matching *spec.Swagger fields.
//
// Title and Description come from grammar's prose classifier; level-0 Property entries are routed
// by keyword name to the appropriate setter.
//
// swspec may have a nil Info field on entry; the helper allocates one before writing the first
// Info.* value.
func applyMetaBlock(swspec *spec.Swagger, block grammar.Block, clean func(string) string) error {
	if swspec.Info == nil {
		swspec.Info = new(spec.Info)
	}

	// Title / Description are godoc-derived (swagger:meta has no override path), so they pass through
	// the CleanGoDoc filter; the remaining meta fields are structured values, not prose, and are left
	// untouched.
	swspec.Info.Title = clean(stripPackagePrefix(block.Title()))
	swspec.Info.Description = clean(block.Description())

	for p := range block.Properties() {
		if p.ItemsDepth != 0 {
			continue
		}
		if err := dispatchMetaKeyword(p, swspec); err != nil {
			return err
		}
	}

	if reqs := block.SecurityRequirements(); reqs != nil {
		swspec.Security = reqs
	}
	for ext := range block.Extensions() {
		switch ext.Source {
		case grammar.KwInfoExtensions:
			if swspec.Info.Extensions == nil {
				swspec.Info.Extensions = spec.Extensions{}
			}
			swspec.Info.Extensions.Add(ext.Name, ext.Value)
		default:
			if swspec.Extensions == nil {
				swspec.Extensions = spec.Extensions{}
			}
			swspec.Extensions.Add(ext.Name, ext.Value)
		}
	}
	c, err := block.Contact()
	if err != nil {
		return err
	}
	if c != (grammar.Contact{}) {
		swspec.Info.Contact = &spec.ContactInfo{
			ContactInfoProps: spec.ContactInfoProps{Name: c.Name, Email: c.Email, URL: c.URL},
		}
	}
	if l, ok := block.License(); ok {
		swspec.Info.License = &spec.License{
			LicenseProps: spec.LicenseProps{Name: l.Name, URL: l.URL},
		}
	}

	return nil
}

// dispatchMetaKeyword routes one Property to the matching meta-side setter.
//
// Inline-value keywords (schemes, version, host, basePath, license, contact) read Property.Value;
// raw-block keywords (tos, consumes, produces, security, securityDefinitions, infoExtensions,
// extensions) split Property.Body and feed the body parsers.
func dispatchMetaKeyword(p grammar.Property, swspec *spec.Swagger) error {
	if dispatchMetaSimple(p, swspec) {
		return nil
	}
	return dispatchMetaYAMLBlock(p, swspec)
}

// dispatchMetaSimple handles the keywords whose setters cannot fail.
func dispatchMetaSimple(p grammar.Property, swspec *spec.Swagger) bool {
	switch p.Keyword.Name {
	case grammar.KwTOS:
		swspec.Info.TermsOfService = joinNonBlank(bodyLines(p.Body))
	case grammar.KwConsumes:
		swspec.Consumes = p.AsList()
	case grammar.KwProduces:
		swspec.Produces = p.AsList()
	case grammar.KwSchemes:
		swspec.Schemes = p.AsList()
	case grammar.KwVersion:
		swspec.Info.Version = strings.TrimSpace(p.Value)
	case grammar.KwHost:
		host := strings.TrimSpace(p.Value)
		if host == "" {
			host = "localhost"
		}
		swspec.Host = host
	case grammar.KwBasePath:
		swspec.BasePath = strings.TrimSpace(p.Value)
	default:
		return false
	}
	return true
}

// dispatchMetaYAMLBlock handles the keywords whose bodies are structurally YAML and not amenable to
// the flex-list union: securityDefinitions and externalDocs. extensions / infoExtensions ride
// grammar's typed Extensions surface (see applyMetaBlock — the block.Extensions() loop routes
// each entry by ext.Source).
//
// The KwExternalDocs arm here sets the top-level spec.ExternalDocs for swagger:meta.
// The same keyword on route/operation/schema is emitted by their own builders (routes/walker.go,
// handlers.schemaRawHandler), and per-tag externalDocs rides the KwTags []spec.Tag unmarshal below.
func dispatchMetaYAMLBlock(p grammar.Property, swspec *spec.Swagger) error {
	switch p.Keyword.Name {
	case grammar.KwSecurityDefinitions:
		return yamlparser.UnmarshalBody(p.Body, func(data []byte) error {
			var d spec.SecurityDefinitions
			if err := json.Unmarshal(data, &d); err != nil {
				return err
			}
			swspec.SecurityDefinitions = d
			return nil
		})
	case grammar.KwExternalDocs:
		return yamlparser.UnmarshalBody(p.Body, func(data []byte) error {
			var d spec.ExternalDocumentation
			if err := json.Unmarshal(data, &d); err != nil {
				return err
			}
			// Skip an empty/blank block so we don't emit a useless `externalDocs: {}` (the OAS object
			// requires `url`).
			if d != (spec.ExternalDocumentation{}) {
				swspec.ExternalDocs = &d
			}
			return nil
		})
	case grammar.KwTags:
		// `Tags:` is a YAML list of tag objects ({name, description, externalDocs, x-*}) →
		// spec.Swagger.Tags (go-swagger#2655).
		return yamlparser.UnmarshalListBody(p.Body, func(data []byte) error {
			var tags []spec.Tag
			if err := json.Unmarshal(data, &tags); err != nil {
				return err
			}
			swspec.Tags = append(swspec.Tags, tags...)
			return nil
		})
	}
	return nil
}

// bodyLines splits a grammar raw-block body into the []string shape the meta body parsers expect.
func bodyLines(body string) []string {
	if body == "" {
		return nil
	}
	lines := strings.Split(body, "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines
}

// joinNonBlank joins lines with "\n" after dropping whitespace-only entries.
//
// Used for the `Terms Of Service:` body — author free-form prose that should land as a single
// multi-line string on Info.TermsOfService.
func joinNonBlank(lines []string) string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// stripPackagePrefix shaves a leading `Package <ident>` prefix off a meta title.
//
// Go's `// Package <name>` doc-comment convention puts the package marker on the first prose line;
// the emitted Info.Title should carry only the rest.
// Returns the input unchanged when the pattern is not present.
//
// Match shape: optional leading whitespace, then `Package` (capital P, the canonical godoc spelling
// — `package` lowercase rejected so authors writing prose like "package this carefully" don't get
// silently chopped), one or more spaces, the package identifier (any non-space run), then optional
// trailing whitespace.
func stripPackagePrefix(s string) string {
	rest, ok := strings.CutPrefix(strings.TrimLeft(s, " \t"), "Package ")
	if !ok {
		return s
	}
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" {
		return s
	}
	idx := strings.IndexAny(rest, " \t")
	if idx < 0 {
		// Title is exactly `Package <ident>` with nothing after — preserve the original so the spec
		// doesn't end up with an empty Title.
		return s
	}
	return strings.TrimLeft(rest[idx:], " \t")
}
