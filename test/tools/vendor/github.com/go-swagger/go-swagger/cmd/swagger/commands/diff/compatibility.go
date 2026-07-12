// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package diff

// CompatibilityPolicy decides which changes are breaking and which are not.
type CompatibilityPolicy struct {
	ForResponse map[SpecChangeCode]Compatibility
	ForRequest  map[SpecChangeCode]Compatibility
	ForChange   map[SpecChangeCode]Compatibility
}

var compatibility CompatibilityPolicy

func init() {
	compatibility = CompatibilityPolicy{
		ForResponse: map[SpecChangeCode]Compatibility{
			AddedRequiredProperty:   Breaking,
			DeletedProperty:         Breaking,
			AddedProperty:           NonBreaking,
			DeletedResponse:         Breaking,
			AddedResponse:           NonBreaking,
			WidenedType:             NonBreaking,
			NarrowedType:            NonBreaking,
			ChangedType:             Breaking,
			ChangedToCompatibleType: NonBreaking,
			AddedEnumValue:          Breaking,
			DeletedEnumValue:        NonBreaking,
			AddedResponseHeader:     NonBreaking,
			ChangedResponseHeader:   Breaking,
			DeletedResponseHeader:   Breaking,
			ChangedDescription:      NonBreaking,
			AddedDescription:        NonBreaking,
			DeletedDescription:      NonBreaking,
			ChangedTag:              NonBreaking,
			AddedTag:                NonBreaking,
			DeletedTag:              NonBreaking,
			DeletedConstraint:       Breaking,
			AddedConstraint:         NonBreaking,
			DeletedExtension:        Warning,
			AddedExtension:          Warning,
			ChangedExtensionValue:   Warning,
		},
		ForRequest: map[SpecChangeCode]Compatibility{
			AddedRequiredProperty:     Breaking,
			DeletedProperty:           Breaking,
			AddedProperty:             NonBreaking,
			AddedOptionalParam:        NonBreaking,
			AddedRequiredParam:        Breaking,
			DeletedOptionalParam:      NonBreaking,
			DeletedRequiredParam:      NonBreaking,
			WidenedType:               NonBreaking,
			NarrowedType:              Breaking,
			ChangedType:               Breaking,
			ChangedToCompatibleType:   NonBreaking,
			ChangedOptionalToRequired: Breaking,
			ChangedRequiredToOptional: NonBreaking,
			AddedEnumValue:            NonBreaking,
			DeletedEnumValue:          Breaking,
			ChangedDescription:        NonBreaking,
			AddedDescription:          NonBreaking,
			DeletedDescription:        NonBreaking,
			ChangedTag:                NonBreaking,
			AddedTag:                  NonBreaking,
			DeletedTag:                NonBreaking,
			DeletedConstraint:         NonBreaking,
			AddedConstraint:           Breaking,
			ChangedDefault:            Warning,
			AddedDefault:              Warning,
			DeletedDefault:            Warning,
			ChangedExample:            NonBreaking,
			AddedExample:              NonBreaking,
			DeletedExample:            NonBreaking,
			ChangedCollectionFormat:   Breaking,
			DeletedExtension:          Warning,
			AddedExtension:            Warning,
			ChangedExtensionValue:     Warning,
		},
		ForChange: map[SpecChangeCode]Compatibility{
			NoChangeDetected:          NonBreaking,
			AddedEndpoint:             NonBreaking,
			DeletedEndpoint:           Breaking,
			DeletedDeprecatedEndpoint: NonBreaking,
			AddedConsumesFormat:       NonBreaking,
			DeletedConsumesFormat:     Breaking,
			AddedProducesFormat:       NonBreaking,
			DeletedProducesFormat:     Breaking,
			AddedSchemes:              NonBreaking,
			DeletedSchemes:            Breaking,
			ChangedHostURL:            Breaking,
			ChangedBasePath:           Breaking,
			ChangedDescription:        NonBreaking,
			AddedDescription:          NonBreaking,
			DeletedDescription:        NonBreaking,
			ChangedTag:                NonBreaking,
			AddedTag:                  NonBreaking,
			DeletedTag:                NonBreaking,
			RefTargetChanged:          Breaking,
			RefTargetRenamed:          NonBreaking,
			AddedDefinition:           NonBreaking,
			DeletedDefinition:         NonBreaking,
			DeletedExtension:          Warning,
			AddedExtension:            Warning,
			ChangedExtensionValue:     Warning,
		},
	}
}

func getCompatibilityForChange(diffCode SpecChangeCode, where DataDirection) Compatibility {
	if compat, commonChange := compatibility.ForChange[diffCode]; commonChange {
		return compat
	}
	if where == Request {
		return compatibility.ForRequest[diffCode]
	}
	return compatibility.ForResponse[diffCode]
}
