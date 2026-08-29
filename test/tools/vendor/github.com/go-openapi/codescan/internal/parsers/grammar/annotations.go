// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

// AnnotationPrefix is the literal that introduces every codescan annotation header.
//
// Centralised so callers and tests reference the single source of truth rather than the bare
// literal.
const AnnotationPrefix = "swagger:"

// AnnotationKind identifies the top-level swagger:<name> directive.
type AnnotationKind int

const (
	AnnUnknown AnnotationKind = iota

	AnnModel       // swagger:model
	AnnResponse    // swagger:response
	AnnParameters  // swagger:parameters
	AnnRoute       // swagger:route
	AnnOperation   // swagger:operation
	AnnMeta        // swagger:meta
	AnnStrfmt      // swagger:strfmt
	AnnAlias       // swagger:alias
	AnnName        // swagger:name
	AnnAllOf       // swagger:allOf
	AnnEnum        // swagger:enum
	AnnIgnore      // swagger:ignore
	AnnDefaultName // swagger:default — value-only classifier annotation
	AnnType        // swagger:type
	AnnFile        // swagger:file
	// AnnAdditionalProperties — swagger:additionalProperties <spec>.
	//
	// A type/model-level classifier whose arg is true | false | a swagger:type-style spec.
	// See the schema builder's classifierAdditionalProperties.
	AnnAdditionalProperties
	// AnnPatternProperties — swagger:patternProperties "<re>": <spec>, … A type/model-level
	// classifier whose arg is a comma-separated list of quoted-regex → swagger:type-style-spec
	// pairs.
	//
	// The whole remainder is captured as one raw arg token; the schema builder parses the pairs.
	// See classifierPatternProperties.
	AnnPatternProperties
	// AnnTitle — swagger:title <text>.
	//
	// A schema-level override that replaces the godoc-derived title on a model / field.
	// Single-line: the arg is the whole rest of the line.
	AnnTitle
	// AnnDescription — swagger:description <text> [+ body].
	//
	// A schema / response / header override that replaces the godoc-derived description.
	// The arg is the rest of the head line; under Option B a blank-terminated body may extend it (P4).
	// See the design doc above.
	AnnDescription
)

const (
	labelModel                = "model"
	labelResponse             = "response"
	labelParameters           = "parameters"
	labelRoute                = "route"
	labelOperation            = "operation"
	labelMeta                 = "meta"
	labelStrfmt               = "strfmt"
	labelAlias                = "alias"
	labelName                 = "name"
	labelAllOf                = "allOf"
	labelEnum                 = "enum"
	labelIgnore               = "ignore"
	labelDefault              = "default"
	labelType                 = "type"
	labelFile                 = "file"
	labelAdditionalProperties = "additionalProperties"
	labelPatternProperties    = "patternProperties"
	labelTitle                = "title"
	labelDescription          = "description"
	labelUnknown              = "unknown"
)

// String renders an AnnotationKind as its source label.
func (a AnnotationKind) String() string {
	switch a {
	case AnnModel:
		return labelModel
	case AnnResponse:
		return labelResponse
	case AnnParameters:
		return labelParameters
	case AnnRoute:
		return labelRoute
	case AnnOperation:
		return labelOperation
	case AnnMeta:
		return labelMeta
	case AnnStrfmt:
		return labelStrfmt
	case AnnAlias:
		return labelAlias
	case AnnName:
		return labelName
	case AnnAllOf:
		return labelAllOf
	case AnnEnum:
		return labelEnum
	case AnnIgnore:
		return labelIgnore
	case AnnDefaultName:
		return labelDefault
	case AnnType:
		return labelType
	case AnnFile:
		return labelFile
	case AnnAdditionalProperties:
		return labelAdditionalProperties
	case AnnPatternProperties:
		return labelPatternProperties
	case AnnTitle:
		return labelTitle
	case AnnDescription:
		return labelDescription
	case AnnUnknown:
		fallthrough
	default:
		return labelUnknown
	}
}

// AnnotationKindFromName resolves the swagger:<name> label to its kind.
//
// Returns AnnUnknown for labels outside the recognised set.
func AnnotationKindFromName(name string) AnnotationKind {
	switch name {
	case labelModel:
		return AnnModel
	case labelResponse:
		return AnnResponse
	case labelParameters:
		return AnnParameters
	case labelRoute:
		return AnnRoute
	case labelOperation:
		return AnnOperation
	case labelMeta:
		return AnnMeta
	case labelStrfmt:
		return AnnStrfmt
	case labelAlias:
		return AnnAlias
	case labelName:
		return AnnName
	case labelAllOf:
		return AnnAllOf
	case labelEnum:
		return AnnEnum
	case labelIgnore:
		return AnnIgnore
	case labelDefault:
		return AnnDefaultName
	case labelType:
		return AnnType
	case labelFile:
		return AnnFile
	case labelAdditionalProperties:
		return AnnAdditionalProperties
	case labelPatternProperties:
		return AnnPatternProperties
	case labelTitle:
		return AnnTitle
	case labelDescription:
		return AnnDescription
	default:
		return AnnUnknown
	}
}

// annotationFamily classifies an AnnotationKind into one of the four family sub-grammars.
//
// Used by the parser dispatcher.
type annotationFamily int

const (
	familyUnknown annotationFamily = iota
	familySchema
	familyOperation
	familyMeta
	familyClassifier
)

func (a AnnotationKind) family() annotationFamily {
	switch a {
	case AnnModel, AnnResponse, AnnParameters,
		// swagger:name is a field-level rename that accepts the same validation-keyword body as a schema
		// field (min length, pattern, required, etc.).
		// It dispatches through the schema parser so the body keywords surface as Properties rather than
		// being rejected as context-invalid under a classifier block.
		//
		// See README §parser-contract.
		AnnName,
		// swagger:title / swagger:description are field/decl overrides that, like swagger:name, must
		// coexist with the field's own validation keywords on the same comment group — so they dispatch
		// through the schema parser too (familySchema), not the classifier parser which would reject
		// those co-located keywords.
		AnnTitle, AnnDescription:
		return familySchema
	case AnnRoute, AnnOperation:
		return familyOperation
	case AnnMeta:
		return familyMeta
	case AnnStrfmt, AnnAlias, AnnAllOf, AnnEnum,
		AnnIgnore, AnnDefaultName, AnnType, AnnFile,
		AnnAdditionalProperties, AnnPatternProperties:
		return familyClassifier
	case AnnUnknown:
		fallthrough
	default:
		return familyUnknown
	}
}
