// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
)

// SpecChangeCode enumerates the various types of diffs from one spec to another.
type SpecChangeCode int

const (
	// NoChangeDetected - the specs have no changes.
	NoChangeDetected SpecChangeCode = iota
	// DeletedProperty - A message property has been deleted in the new spec.
	DeletedProperty
	// AddedProperty - A message property has been added in the new spec.
	AddedProperty
	// AddedRequiredProperty - A required message property has been added in the new spec.
	AddedRequiredProperty
	// DeletedOptionalParam - An endpoint parameter has been deleted in the new spec.
	DeletedOptionalParam
	// ChangedDescription - Changed a description.
	ChangedDescription
	// AddedDescription - Added a description.
	AddedDescription
	// DeletedDescription - Deleted a description.
	DeletedDescription
	// ChangedTag - Changed a tag.
	ChangedTag
	// AddedTag - Added a tag.
	AddedTag
	// DeletedTag - Deleted a tag.
	DeletedTag
	// DeletedResponse - An endpoint response has been deleted in the new spec.
	DeletedResponse
	// DeletedEndpoint - An endpoint has been deleted in the new spec.
	DeletedEndpoint
	// DeletedDeprecatedEndpoint - A deprecated endpoint has been deleted in the new spec.
	DeletedDeprecatedEndpoint
	// AddedRequiredParam - A required parameter has been added in the new spec.
	AddedRequiredParam
	// DeletedRequiredParam - A required parameter has been deleted in the new spec.
	DeletedRequiredParam
	// AddedEndpoint - An endpoint has been added in the new spec.
	AddedEndpoint
	// WidenedType - An type has been changed to a more permissive type eg int->string.
	WidenedType
	// NarrowedType - An type has been changed to a less permissive type eg string->int.
	NarrowedType
	// ChangedToCompatibleType - An type has been changed to a compatible type eg password->string.
	ChangedToCompatibleType
	// ChangedType - An type has been changed to a type whose relative compatibility cannot be determined.
	ChangedType
	// AddedEnumValue - An enum type has had a new potential value added to it.
	AddedEnumValue
	// DeletedEnumValue - An enum type has had a existing value removed from it.
	DeletedEnumValue
	// AddedOptionalParam - A new optional parameter has been added to the new spec.
	AddedOptionalParam
	// ChangedOptionalToRequired - An optional parameter is now required in the new spec.
	ChangedOptionalToRequired
	// ChangedRequiredToOptional - An required parameter is now optional in the new spec.
	ChangedRequiredToOptional
	// AddedResponse An endpoint has new response code in the new spec.
	AddedResponse
	// AddedConsumesFormat - a new consumes format (json/xml/yaml etc) has been added in the new spec.
	AddedConsumesFormat
	// DeletedConsumesFormat - an existing format has been removed in the new spec.
	DeletedConsumesFormat
	// AddedProducesFormat - a new produces format (json/xml/yaml etc) has been added in the new spec.
	AddedProducesFormat
	// DeletedProducesFormat - an existing produces format has been removed in the new spec.
	DeletedProducesFormat
	// AddedSchemes - a new scheme has been added to the new spec.
	AddedSchemes
	// DeletedSchemes - a scheme has been removed from the new spec.
	DeletedSchemes
	// ChangedHostURL - the host url has been changed. If this is used in the client generation, then clients will break.
	ChangedHostURL
	// ChangedBasePath - the host base path has been changed. If this is used in the client generation, then clients will break.
	ChangedBasePath
	// AddedResponseHeader Added a header Item.
	AddedResponseHeader
	// ChangedResponseHeader Added a header Item.
	ChangedResponseHeader
	// DeletedResponseHeader Added a header Item.
	DeletedResponseHeader
	// RefTargetChanged Changed a ref to point to a different object.
	RefTargetChanged
	// RefTargetRenamed Renamed a ref to point to the same object.
	RefTargetRenamed
	// DeletedConstraint Deleted a schema constraint.
	DeletedConstraint
	// AddedConstraint Added a schema constraint.
	AddedConstraint
	// DeletedDefinition removed one of the definitions.
	DeletedDefinition
	// AddedDefinition removed one of the definitions.
	AddedDefinition
	// ChangedDefault - Changed default value.
	ChangedDefault
	// AddedDefault - Added a default value.
	AddedDefault
	// DeletedDefault - Deleted a default value.
	DeletedDefault
	// ChangedExample - Changed an example value.
	ChangedExample
	// AddedExample - Added an example value.
	AddedExample
	// DeletedExample - Deleted an example value.
	DeletedExample
	// ChangedCollectionFormat - A collectionFormat has been changed to a collectionFormat whose relative compatibility cannot be determined.
	ChangedCollectionFormat
	// DeletedExtension deleted an extension.
	DeletedExtension
	// AddedExtension added an extension.
	AddedExtension
	// ChangedExtensionValue changed an extension value.
	ChangedExtensionValue
)

var toLongStringSpecChangeCode = map[SpecChangeCode]string{
	NoChangeDetected:          "No Change detected",
	AddedEndpoint:             "Added endpoint",
	DeletedEndpoint:           "Deleted endpoint",
	DeletedDeprecatedEndpoint: "Deleted a deprecated endpoint",
	AddedRequiredProperty:     "Added required property",
	DeletedProperty:           "Deleted property",
	ChangedDescription:        "Changed a description",
	AddedDescription:          "Added a description",
	DeletedDescription:        "Deleted a description",
	ChangedTag:                "Changed a tag",
	AddedTag:                  "Added a tag",
	DeletedTag:                "Deleted a tag",
	AddedProperty:             "Added property",
	AddedOptionalParam:        "Added optional param",
	AddedRequiredParam:        "Added required param",
	DeletedOptionalParam:      "Deleted optional param",
	DeletedRequiredParam:      "Deleted required param",
	DeletedResponse:           "Deleted response",
	AddedResponse:             "Added response",
	WidenedType:               "Widened type",
	NarrowedType:              "Narrowed type",
	ChangedType:               "Changed type",
	ChangedToCompatibleType:   "Changed type to equivalent type",
	ChangedOptionalToRequired: "Changed optional param to required",
	ChangedRequiredToOptional: "Changed required param to optional",
	AddedEnumValue:            "Added possible enumeration(s)",
	DeletedEnumValue:          "Deleted possible enumeration(s)",
	AddedConsumesFormat:       "Added a consumes format",
	DeletedConsumesFormat:     "Deleted a consumes format",
	AddedProducesFormat:       "Added produces format",
	DeletedProducesFormat:     "Deleted produces format",
	AddedSchemes:              "Added schemes",
	DeletedSchemes:            "Deleted schemes",
	ChangedHostURL:            "Changed host URL",
	ChangedBasePath:           "Changed base path",
	AddedResponseHeader:       "Added response header",
	ChangedResponseHeader:     "Changed response header",
	DeletedResponseHeader:     "Deleted response header",
	RefTargetChanged:          "Changed ref to different object",
	RefTargetRenamed:          "Changed ref to renamed object",
	DeletedConstraint:         "Deleted a schema constraint",
	AddedConstraint:           "Added a schema constraint",
	DeletedDefinition:         "Deleted a schema definition",
	AddedDefinition:           "Added a schema definition",
	ChangedDefault:            "Default value is changed",
	AddedDefault:              "Default value is added",
	DeletedDefault:            "Default value is removed",
	ChangedExample:            "Example value is changed",
	AddedExample:              "Example value is added",
	DeletedExample:            "Example value is removed",
	ChangedCollectionFormat:   "Changed collection format",
	DeletedExtension:          "Deleted Extension",
	AddedExtension:            "Added Extension",
	ChangedExtensionValue:     "Changed Extension Value",
}

var toStringSpecChangeCode = map[SpecChangeCode]string{
	AddedEndpoint:             "AddedEndpoint",
	NoChangeDetected:          "NoChangeDetected",
	DeletedEndpoint:           "DeletedEndpoint",
	DeletedDeprecatedEndpoint: "DeletedDeprecatedEndpoint",
	AddedRequiredProperty:     "AddedRequiredProperty",
	DeletedProperty:           "DeletedProperty",
	AddedProperty:             "AddedProperty",
	ChangedDescription:        "ChangedDescription",
	AddedDescription:          "AddedDescription",
	DeletedDescription:        "DeletedDescription",
	ChangedTag:                "ChangedTag",
	AddedTag:                  "AddedTag",
	DeletedTag:                "DeletedTag",
	AddedOptionalParam:        "AddedOptionalParam",
	AddedRequiredParam:        "AddedRequiredParam",
	DeletedOptionalParam:      "DeletedRequiredParam",
	DeletedRequiredParam:      "Deleted required param",
	DeletedResponse:           "DeletedResponse",
	AddedResponse:             "AddedResponse",
	WidenedType:               "WidenedType",
	NarrowedType:              "NarrowedType",
	ChangedType:               "ChangedType",
	ChangedToCompatibleType:   "ChangedToCompatibleType",
	ChangedOptionalToRequired: "ChangedOptionalToRequiredParam",
	ChangedRequiredToOptional: "ChangedRequiredToOptionalParam",
	AddedEnumValue:            "AddedEnumValue",
	DeletedEnumValue:          "DeletedEnumValue",
	AddedConsumesFormat:       "AddedConsumesFormat",
	DeletedConsumesFormat:     "DeletedConsumesFormat",
	AddedProducesFormat:       "AddedProducesFormat",
	DeletedProducesFormat:     "DeletedProducesFormat",
	AddedSchemes:              "AddedSchemes",
	DeletedSchemes:            "DeletedSchemes",
	ChangedHostURL:            "ChangedHostURL",
	ChangedBasePath:           "ChangedBasePath",
	AddedResponseHeader:       "AddedResponseHeader",
	ChangedResponseHeader:     "ChangedResponseHeader",
	DeletedResponseHeader:     "DeletedResponseHeader",
	RefTargetChanged:          "RefTargetChanged",
	RefTargetRenamed:          "RefTargetRenamed",
	DeletedConstraint:         "DeletedConstraint",
	AddedConstraint:           "AddedConstraint",
	DeletedDefinition:         "DeletedDefinition",
	AddedDefinition:           "AddedDefinition",
	ChangedDefault:            "ChangedDefault",
	AddedDefault:              "AddedDefault",
	DeletedDefault:            "DeletedDefault",
	ChangedExample:            "ChangedExample",
	AddedExample:              "AddedExample",
	DeletedExample:            "DeletedExample",
	ChangedCollectionFormat:   "ChangedCollectionFormat",
	DeletedExtension:          "DeletedExtension",
	AddedExtension:            "AddedExtension",
	ChangedExtensionValue:     "ChangedExtensionValue",
}

var toIDSpecChangeCode = map[string]SpecChangeCode{}

// Description returns an english version of this error.
func (s SpecChangeCode) Description() (result string) {
	result, ok := toLongStringSpecChangeCode[s]
	if !ok {
		log.Printf("warning: No description for %v", s)
		result = "UNDEFINED"
	}
	return result
}

// MarshalJSON marshals the enum as a quoted json string.
func (s SpecChangeCode) MarshalJSON() ([]byte, error) {
	return stringAsQuotedBytes(toStringSpecChangeCode[s])
}

// UnmarshalJSON unmashalls a quoted json string to the enum value.
func (s *SpecChangeCode) UnmarshalJSON(b []byte) error {
	str, err := readStringFromByteStream(b)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will return an error to the caller.
	val, ok := toIDSpecChangeCode[str]

	if ok {
		*s = val
	} else {
		return fmt.Errorf("unknown enum value. cannot unmarshal '%s'", str)
	}
	return nil
}

// Compatibility - whether this is a breaking or non-breaking change.
type Compatibility int

const (
	// Breaking this change could break existing clients.
	Breaking Compatibility = iota
	// NonBreaking This is a backwards-compatible API change.
	NonBreaking
	// Warning changes are technically non-breaking but can cause behavior changes in client and thus should be reported differently.
	Warning
)

func (s Compatibility) String() string {
	return toStringCompatibility[s]
}

var toStringCompatibility = map[Compatibility]string{
	Breaking:    "Breaking",
	NonBreaking: "NonBreaking",
	Warning:     "Warning",
}

var toIDCompatibility = map[string]Compatibility{}

// MarshalJSON marshals the enum as a quoted json string.
func (s Compatibility) MarshalJSON() ([]byte, error) {
	return stringAsQuotedBytes(toStringCompatibility[s])
}

// UnmarshalJSON unmashals a quoted json string to the enum value.
func (s *Compatibility) UnmarshalJSON(b []byte) error {
	str, err := readStringFromByteStream(b)
	if err != nil {
		return err
	}
	// Note that if the string cannot be found then it will return an error to the caller.
	val, ok := toIDCompatibility[str]

	if ok {
		*s = val
	} else {
		return fmt.Errorf("unknown enum value. cannot unmarshal '%s'", str)
	}
	return nil
}

func stringAsQuotedBytes(str string) ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(str)
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

func readStringFromByteStream(b []byte) (string, error) {
	var j string
	err := json.Unmarshal(b, &j)
	if err != nil {
		return "", err
	}
	return j, nil
}

func init() {
	for key, val := range toStringSpecChangeCode {
		toIDSpecChangeCode[val] = key
	}
	for key, val := range toStringCompatibility {
		toIDCompatibility[val] = key
	}
}
