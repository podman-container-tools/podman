// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package routes

import (
	"github.com/go-openapi/codescan/internal/parsers"
	oaispec "github.com/go-openapi/spec"
)

func (r *Builder) routeTaggers(op *oaispec.Operation) []parsers.TagParser {
	return []parsers.TagParser{
		parsers.NewMultiLineTagParser("Consumes", parsers.NewConsumesDropEmptyParser(opConsumesSetter(op)), false),
		parsers.NewMultiLineTagParser("Produces", parsers.NewProducesDropEmptyParser(opProducesSetter(op)), false),
		parsers.NewSingleLineTagParser("Schemes", parsers.NewSetSchemes(opSchemeSetter(op))),
		parsers.NewMultiLineTagParser("Security", parsers.NewSetSecurityScheme(opSecurityDefsSetter(op)), false),
		parsers.NewMultiLineTagParser("Parameters", parsers.NewSetParams(r.parameters, opParamSetter(op)), false),
		parsers.NewMultiLineTagParser("Responses", parsers.NewSetResponses(r.definitions, r.responses, opResponsesSetter(op)), false),
		parsers.NewSingleLineTagParser("Deprecated", parsers.NewSetDeprecatedOp(op)),
		parsers.NewMultiLineTagParser("Extensions", parsers.NewSetExtensions(opExtensionsSetter(op), r.ctx.Debug()), true),
	}
}
