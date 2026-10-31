// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"

	"github.com/go-openapi/errors"
)

// Error messages related to schema validation and returned as results.
const (
	// ArrayDoesNotAllowAdditionalItemsError when an additionalItems construct is not verified by the array values provided.
	//
	// Proposal for enhancement: should move to package go-openapi/errors.
	ArrayDoesNotAllowAdditionalItemsError = "array doesn't allow for additional items"

	// HasDependencyError indicates that a dependencies construct was not verified.
	HasDependencyError = "%q has a dependency on %s"

	// InvalidSchemaProvidedError indicates that the schema provided to validate a value cannot be properly compiled.
	InvalidSchemaProvidedError = "Invalid schema provided to SchemaValidator: %v"

	// InvalidTypeConversionError indicates that a numerical conversion for the given type could not be carried on.
	InvalidTypeConversionError = "invalid type conversion in %s: %v "

	// MustValidateAtLeastOneSchemaError indicates that in a AnyOf construct, none of the schema constraints specified were verified.
	MustValidateAtLeastOneSchemaError = "%q must validate at least one schema (anyOf)"

	// MustValidateOnlyOneSchemaError indicates that in a OneOf construct, either none of the schema constraints specified were verified, or several were.
	MustValidateOnlyOneSchemaError = "%q must validate one and only one schema (oneOf). %s"

	// MustValidateAllSchemasError indicates that in a AllOf construct, at least one of the schema constraints specified were not verified.
	//
	// NOTE: punctuation in message.
	MustValidateAllSchemasError = "%q must validate all the schemas (allOf)%s"

	// TooManyCompositionValidatorsError indicates that allOf, anyOf, oneOf and not, followed through
	// their $ref, nest too deep or need too many validators. A circular $ref through these
	// keywords is the usual cause.
	TooManyCompositionValidatorsError = "%q nests allOf, anyOf, oneOf and not deeper than %d levels or into more than %d validators: look for a circular $ref"

	// MustNotValidateSchemaError indicates that in a Not construct, the schema constraint specified was verified.
	MustNotValidateSchemaError = "%q must not validate the schema (not)"
)

// invalidSchemaProvidedPrefix starts the panic message of a schema validator that cannot be built.
var invalidSchemaProvidedPrefix = strings.TrimSuffix(InvalidSchemaProvidedError, "%v")

// Warning messages related to schema validation and returned as results.
const ()

func invalidSchemaProvidedMsg(err error) errors.Error {
	return errors.New(InternalErrorCode, InvalidSchemaProvidedError, err)
}

func tooManyCompositionValidatorsMsg(path string, maxDepth, maxValidators int) errors.Error {
	return errors.New(InternalErrorCode, TooManyCompositionValidatorsError, path, maxDepth, maxValidators)
}

func invalidTypeConversionMsg(path string, err error) errors.Error {
	return errors.New(errors.CompositeErrorCode, InvalidTypeConversionError, path, err)
}

func mustValidateOnlyOneSchemaMsg(path, additionalMsg string) errors.Error {
	return errors.New(errors.CompositeErrorCode, MustValidateOnlyOneSchemaError, path, additionalMsg)
}

func mustValidateAtLeastOneSchemaMsg(path string) errors.Error {
	return errors.New(errors.CompositeErrorCode, MustValidateAtLeastOneSchemaError, path)
}

func mustValidateAllSchemasMsg(path, additionalMsg string) errors.Error {
	return errors.New(errors.CompositeErrorCode, MustValidateAllSchemasError, path, additionalMsg)
}

func mustNotValidatechemaMsg(path string) errors.Error {
	return errors.New(errors.CompositeErrorCode, MustNotValidateSchemaError, path)
}

func hasADependencyMsg(path, depkey string) errors.Error {
	return errors.New(errors.CompositeErrorCode, HasDependencyError, path, depkey)
}

func arrayDoesNotAllowAdditionalItemsMsg() errors.Error {
	return errors.New(errors.CompositeErrorCode, ArrayDoesNotAllowAdditionalItemsError)
}
