// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

// AnnotationDoc is the reference entry for one annotation: enough to answer "what does this do and what may I write
// under it" without leaving the editor.
//
// Deliberately three short fields rather than prose. This is looked at mid-typing, in a popup a few lines tall, by
// someone who already knows roughly what they want — the long form lives on the documentation site.
type AnnotationDoc struct {
	// Usage is the syntax line, in the doc site's notation: UPPER for a required placeholder, [ ] around what is
	// optional, ( a | b ) for a choice.
	Usage string

	// Summary is the one-line statement of what the annotation does.
	Summary string

	// Keywords describes what may appear in the annotation's body, or is empty when it takes no body.
	//
	// Prose rather than a generated list: the keyword table knows which CONTEXTS a keyword is legal in, not which
	// annotations open those contexts, so generating this would mean inventing that mapping and getting it subtly wrong.
	Keywords string
}

// Doc returns the reference entry for an annotation, reporting ok=false for AnnUnknown.
//
// The summaries are the ones published on the documentation site, carried here by hand. That is a copy, on purpose:
// wiring the site's markdown into the build to save twenty short strings would make the parser depend on the docs
// tree, and the two are edited on entirely different rhythms.
//
//nolint:funlen // one case per annotation; a table would only move the same twenty entries elsewhere
func (a AnnotationKind) Doc() (AnnotationDoc, bool) {
	switch a {
	case AnnModel:
		return AnnotationDoc{
			Usage:   "swagger:model [NAME]",
			Summary: "Publishes a Go type as a definitions entry.",
			Keywords: "The type's fields carry the schema keywords: required, readOnly, discriminator, " +
				"minimum/maximum, minLength/maxLength, pattern, enum, default, example, and the rest.",
		}, true
	case AnnResponse:
		return AnnotationDoc{
			Usage:   "swagger:response [NAME]",
			Summary: "Declares a Go struct as a named response object.",
			Keywords: "Fields become headers or the body (`in:body`). Header fields take the SimpleSchema " +
				"keywords: type, format, default, example, collectionFormat and the validations.",
		}, true
	case AnnParameters:
		return AnnotationDoc{
			Usage:   "swagger:parameters OPERATION_ID [OPERATION_ID …]",
			Summary: "Declares a Go struct as the parameter set for one or more operations.",
			Keywords: "Per field: in (query|path|header|body|formData), name, required, collectionFormat, " +
				"default, example, plus the validations legal for its location.",
		}, true
	case AnnRoute:
		return AnnotationDoc{
			Usage:   "swagger:route METHOD PATH [TAG …] OPERATION_ID",
			Summary: "Declares an HTTP route + operation in one annotation.",
			Keywords: "Body: Consumes, Produces, Schemes, Security, Parameters, Responses, Deprecated. " +
				"The first line after the header is the summary; a blank line then starts the description.",
		}, true
	case AnnOperation:
		return AnnotationDoc{
			Usage:   "swagger:operation METHOD PATH [TAG …] OPERATION_ID",
			Summary: "Declares an HTTP route + operation with a YAML-document body.",
			Keywords: "The body is a YAML operation object: summary, description, tags, consumes, produces, " +
				"parameters, responses, security, deprecated.",
		}, true
	case AnnMeta:
		return AnnotationDoc{
			Usage:   "swagger:meta",
			Summary: "Declares the package as the top-level OpenAPI spec container.",
			Keywords: "Body: Version, Host, BasePath, Schemes, Consumes, Produces, Security, " +
				"SecurityDefinitions, Contact, License, TOS, Extensions, InfoExtensions.",
		}, true
	case AnnStrfmt:
		return AnnotationDoc{
			Usage:    "swagger:strfmt FORMAT_NAME",
			Summary:  "Marks a named type as a custom string format.",
			Keywords: "None. The format name is the whole argument.",
		}, true
	case AnnAlias:
		return AnnotationDoc{
			Usage:    "swagger:alias [NAME]",
			Summary:  "Deprecated no-op — alias rendering is controlled by Go aliases plus the scanner options.",
			Keywords: "None.",
		}, true
	case AnnName:
		return AnnotationDoc{
			Usage:   "swagger:name IDENT_NAME",
			Summary: "Overrides the emitted property name of a struct field or interface method.",
			Keywords: "None of its own, but it shares its comment group with the field's schema keywords " +
				"(required, minLength, pattern, …), which still apply.",
		}, true
	case AnnAllOf:
		return AnnotationDoc{
			Usage:    "swagger:allOf",
			Summary:  "Marks a struct as participating in an allOf composition.",
			Keywords: "None. Goes on the embedded field, not the enclosing type.",
		}, true
	case AnnEnum:
		return AnnotationDoc{
			Usage:   "swagger:enum [IDENT_NAME]",
			Summary: "Marks a named type as an enum and collects its const values.",
			Keywords: "None. Members come from the type-checker, so iota, expressions and rune literals all " +
				"resolve; type and format come from the declared Go type.",
		}, true
	case AnnIgnore:
		return AnnotationDoc{
			Usage:    "swagger:ignore",
			Summary:  "Excludes the surrounding declaration (or one field) from the generated spec.",
			Keywords: "None.",
		}, true
	case AnnOmit:
		return AnnotationDoc{
			Usage:   "swagger:omit FIELD[,FIELD…]",
			Summary: "Drops named fields from what an embed promotes into the enclosing schema.",
			Keywords: "None. Names resolve against the embedded TYPE, not against the emitted property names; " +
				"dotted paths are allowed when placed on the declaration.",
		}, true
	case AnnDefaultName:
		return AnnotationDoc{
			Usage:    "swagger:default [VALUE]",
			Summary:  "Deprecated no-op — defaults come from the default: keyword, or a default response code.",
			Keywords: "None.",
		}, true
	case AnnType:
		return AnnotationDoc{
			Usage:    "swagger:type TYPE",
			Summary:  "Replaces a field's or named type's inferred Swagger type with an inlined type.",
			Keywords: "None. TYPE is a scalar, []T, an inline object, or a known type name.",
		}, true
	case AnnFile:
		return AnnotationDoc{
			Usage:    "swagger:file",
			Summary:  "Marks a parameter or response body as a binary file ({type: file}).",
			Keywords: "None.",
		}, true
	case AnnAdditionalProperties:
		return AnnotationDoc{
			Usage:    "swagger:additionalProperties ( true | false | TYPE )",
			Summary:  "Sets a schema's additionalProperties policy for keys beyond the named properties.",
			Keywords: "None. TYPE takes the same forms as swagger:type.",
		}, true
	case AnnPatternProperties:
		return AnnotationDoc{
			Usage:    `swagger:patternProperties "REGEX": TYPE [, "REGEX": TYPE …]`,
			Summary:  "Adds typed patternProperties entries mapping a name regex to a value schema.",
			Keywords: "None. Each TYPE takes the same forms as swagger:type.",
		}, true
	case AnnTitle:
		return AnnotationDoc{
			Usage:    "swagger:title TEXT",
			Summary:  "Overrides the godoc-derived title on a model or field.",
			Keywords: "None. TEXT is the rest of the line.",
		}, true
	case AnnDescription:
		return AnnotationDoc{
			Usage:   "swagger:description TEXT   |   swagger:description |",
			Summary: "Overrides the godoc-derived description on a model, field, response, or header.",
			Keywords: "None. The bare form runs to a blank line; the trailing pipe opens a verbatim markdown " +
				"block that keeps blank lines and indentation until the next annotation.",
		}, true
	case AnnUnknown:
		fallthrough
	default:
		return AnnotationDoc{}, false
	}
}
