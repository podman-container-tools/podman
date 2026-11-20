// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"

	"github.com/go-openapi/spec"
)

const (
	swaggerName                = "name"
	swaggerTags                = "tags"
	swaggerSecurityDefinitions = "securityDefinitions"
	securitySchemeAPIKey       = "apiKey"
)

// Kinds of names, as warnings give them.
const (
	nameKindTag                = "tag"
	nameKindSecurityDefinition = "security definition"
	nameKindAPIKeyName         = "api key name"
	nameKindParameterKey       = "parameter key"
	nameKindParameter          = "parameter"
	nameKindResponseKey        = "response key"
	nameKindHeader             = "header"
	nameKindPath               = "path"
	nameKindOperationID        = "operationId"
	nameKindDefinition         = "definition"
	nameKindDiscriminator      = "discriminator"
	nameKindProperty           = "property"
)

// validateDubiousNames emits warnings (never errors) for names declared by the spec that contain a
// line break (\n or \r).
//
// No name in Swagger 2.0 needs a line break. Code generators print names into the generated
// source, in comments and in string literals: a line feed ends a // comment there, and the rest of
// the name becomes code. A carriage return is flagged too, because other languages and many editors
// end a line on it.
//
// The names checked are:
//
//   - path templates, the keys of paths;
//   - operationId and the tags of each operation, and the names of the top-level tags;
//   - parameter names, and the keys of the top-level parameters;
//   - the keys of the top-level responses, and the response header names;
//   - definition keys, property names and discriminators, in every schema [analysis.Spec] finds;
//   - security definition keys, and the name of an apiKey scheme.
//
// Descriptions, examples, enum and default values are data that may span lines, and are not
// checked.
//
// All findings are warnings: they do not affect validity (see [Result.IsValid]).
func (s *SpecValidator) validateDubiousNames() *Result {
	res := validatorPools.results.Borrow()
	sp := s.spec.Spec()
	if sp == nil {
		return res
	}

	for _, key := range sortedKeys(sp.Parameters) {
		at := newPathSegments(swaggerParameters, key)
		checkName(at, nameKindParameterKey, key, res)
		param := sp.Parameters[key]
		checkName(at.child(swaggerName), nameKindParameter, param.Name, res)
	}

	for _, key := range sortedKeys(sp.Responses) {
		at := newPathSegments(swaggerResponses, key)
		checkName(at, nameKindResponseKey, key, res)
		checkHeaderNames(at, sp.Responses[key].Headers, res)
	}

	for i, tag := range sp.Tags {
		checkName(newPathSegments(swaggerTags).item(i).child(swaggerName), nameKindTag, tag.Name, res)
	}

	for _, key := range sortedKeys(sp.SecurityDefinitions) {
		at := newPathSegments(swaggerSecurityDefinitions, key)
		checkName(at, nameKindSecurityDefinition, key, res)
		if scheme := sp.SecurityDefinitions[key]; scheme != nil && scheme.Type == securitySchemeAPIKey {
			checkName(at.child(swaggerName), nameKindAPIKeyName, scheme.Name, res)
		}
	}

	if sp.Paths != nil {
		for _, path := range sortedKeys(sp.Paths.Paths) {
			at := newPathSegments(swaggerPaths, path)
			checkName(at, nameKindPath, path, res)

			pathItem := sp.Paths.Paths[path]
			checkParamNames(at, pathItem.Parameters, res)

			operations := operationsOf(&pathItem)
			for _, method := range sortedKeys(operations) {
				if op := operations[method]; op != nil {
					checkOperationNames(at.child(method), op, res)
				}
			}
		}
	}

	s.checkSchemaNames(res)

	return res
}

func checkOperationNames(at pathSegments, op *spec.Operation, res *Result) {
	checkName(at.child(swaggerOperationID), nameKindOperationID, op.ID, res)

	for i, tag := range op.Tags {
		checkName(at.child(swaggerTags).item(i), nameKindTag, tag, res)
	}

	checkParamNames(at, op.Parameters, res)

	for _, response := range responsesOf(op) {
		checkHeaderNames(at.children(swaggerResponses, response.code), response.resp.Headers, res)
	}
}

// checkParamNames checks the parameters declared at at. A parameter given by $ref has no name of
// its own: its name is checked where the document declares it.
func checkParamNames(at pathSegments, params []spec.Parameter, res *Result) {
	for i := range params {
		checkName(at.child(swaggerParameters).item(i).child(swaggerName), nameKindParameter, params[i].Name, res)
	}
}

func checkHeaderNames(at pathSegments, headers map[string]spec.Header, res *Result) {
	for _, name := range sortedKeys(headers) {
		checkName(at.children(swaggerHeaders, name), nameKindHeader, name, res)
	}
}

// checkSchemaNames checks every schema the analyzer found, inline ones included: the name of a
// definition or a property, and the discriminator.
//
// The analyzer files each schema under the pointer that locates it, so the member that holds the
// schema tells a definition from a property. The analyzer also files items, allOf members and the
// like, which carry an index or a keyword: those have no name to check.
//
// The pointer is read from the decoded fragment of the schema's reference: the reference itself is
// a URL, which escapes a line break as %0A.
func (s *SpecValidator) checkSchemaNames(res *Result) {
	schemas := s.analyzer.AllDefinitions()
	pointers := make(map[string]int, len(schemas))
	for i, schema := range schemas {
		u := schema.Ref.GetURL()
		if u == nil { // Safeguard: the analyzer always builds a parseable reference
			continue
		}
		pointers[u.Fragment] = i
	}

	for _, pointer := range sortedKeys(pointers) {
		schema := schemas[pointers[pointer]]
		at := localRefPath("#" + pointer)

		switch at.beforeLast() {
		case swaggerDefinitions:
			checkName(at, nameKindDefinition, schema.Name, res)
		case jsonProperties:
			checkName(at, nameKindProperty, schema.Name, res)
		}

		if schema.Schema != nil {
			checkName(at.child(jsonDiscriminator), nameKindDiscriminator, schema.Schema.Discriminator, res)
		}
	}
}

func checkName(at pathSegments, kind, name string, res *Result) {
	if !strings.ContainsAny(name, "\n\r") {
		return
	}

	res.addWarningsAt(at, dubiousNameMsg(kind, name))
}
