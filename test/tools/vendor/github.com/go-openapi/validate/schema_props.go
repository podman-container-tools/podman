// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"reflect"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
)

const (
	// maxCompositionDepth caps the nesting of allOf, anyOf, oneOf and not, followed through
	// their $ref, below one schema validator.
	maxCompositionDepth = 128

	// maxCompositionValidators caps the validators built for allOf, anyOf, oneOf and not, followed
	// through their $ref, below one schema validator.
	maxCompositionValidators = 10_000
)

// compositionBudget bounds the validators built for allOf, anyOf, oneOf and not.
//
// These validators are built with their parent, not when Validate walks the data, and each one
// expands its own $ref. go-openapi/spec bounds a single expansion, but a circular $ref stays in
// place and is expanded again one level down: A: {allOf: [{$ref: A}]} would build validators
// until the goroutine stack overflows, and A: {allOf: [{$ref: A}, {$ref: A}]} would build
// 2^depth of them.
//
// Validators built for properties, items and the like are built as Validate walks the data, so
// the depth of the data bounds them. They start their own budget.
type compositionBudget struct {
	left  int // validators left to build
	depth int // current nesting of allOf, anyOf, oneOf and not
}

// enter draws n validators one level deeper, and panics when either cap is exceeded,
// like other schemas that cannot be expanded.
func (b *compositionBudget) enter(n int, path pathSegments) {
	b.left -= n
	b.depth++

	if b.left < 0 || b.depth > maxCompositionDepth {
		panic(invalidSchemaProvidedMsg(
			tooManyCompositionValidatorsMsg(path.dotted(), maxCompositionDepth, maxCompositionValidators),
		).Error())
	}
}

func (b *compositionBudget) leave() {
	b.depth--
}

type schemaPropsValidator struct {
	Path            pathSegments
	In              string
	AllOf           []spec.Schema
	OneOf           []spec.Schema
	AnyOf           []spec.Schema
	Not             *spec.Schema
	Dependencies    spec.Dependencies
	anyOfValidators []*SchemaValidator
	allOfValidators []*SchemaValidator
	oneOfValidators []*SchemaValidator
	notValidator    *SchemaValidator
	Root            any
	KnownFormats    strfmt.Registry
	Options         *SchemaValidatorOptions
}

func newSchemaPropsValidator(
	path pathSegments, in string, allOf, oneOf, anyOf []spec.Schema, not *spec.Schema, deps spec.Dependencies, root any, formats strfmt.Registry,
	opts *SchemaValidatorOptions, budget *compositionBudget,
) *schemaPropsValidator {
	if opts == nil {
		opts = new(SchemaValidatorOptions)
	}

	children := len(anyOf) + len(allOf) + len(oneOf)
	if not != nil {
		children++
	}

	if children > 0 {
		if budget == nil {
			budget = &compositionBudget{left: maxCompositionValidators}
		}

		budget.enter(children, path)
		defer budget.leave()
	}

	anyValidators := make([]*SchemaValidator, 0, len(anyOf))
	for i := range anyOf {
		anyValidators = append(anyValidators, newSchemaValidatorWithBudget(&anyOf[i], root, path, formats, opts, budget))
	}
	allValidators := make([]*SchemaValidator, 0, len(allOf))
	for i := range allOf {
		allValidators = append(allValidators, newSchemaValidatorWithBudget(&allOf[i], root, path, formats, opts, budget))
	}
	oneValidators := make([]*SchemaValidator, 0, len(oneOf))
	for i := range oneOf {
		oneValidators = append(oneValidators, newSchemaValidatorWithBudget(&oneOf[i], root, path, formats, opts, budget))
	}

	var notValidator *SchemaValidator
	if not != nil {
		notValidator = newSchemaValidatorWithBudget(not, root, path, formats, opts, budget)
	}

	var s *schemaPropsValidator
	if opts.recycleValidators {
		s = validatorPools.schemaPropsValidators.Borrow()
	} else {
		s = new(schemaPropsValidator)
	}

	s.Path = path
	s.In = in
	s.AllOf = allOf
	s.OneOf = oneOf
	s.AnyOf = anyOf
	s.Not = not
	s.Dependencies = deps
	s.anyOfValidators = anyValidators
	s.allOfValidators = allValidators
	s.oneOfValidators = oneValidators
	s.notValidator = notValidator
	s.Root = root
	s.KnownFormats = formats
	s.Options = opts

	return s
}

func (s *schemaPropsValidator) Applies(source any, _ reflect.Kind) bool {
	_, isSchema := source.(*spec.Schema)
	return isSchema
}

func (s *schemaPropsValidator) Validate(data any) *Result {
	var mainResult *Result
	if s.Options.recycleResult {
		mainResult = validatorPools.results.Borrow()
	} else {
		mainResult = new(Result)
	}

	// Intermediary error results

	// IMPORTANT! messages from underlying validators
	var keepResultAnyOf, keepResultOneOf, keepResultAllOf *Result

	if s.Options.recycleValidators {
		defer func() {
			s.redeemChildren()
			s.redeem()

			// results are redeemed when merged
		}()
	}

	if len(s.anyOfValidators) > 0 {
		keepResultAnyOf = validatorPools.results.Borrow()
		s.validateAnyOf(data, mainResult, keepResultAnyOf)
	}

	if len(s.oneOfValidators) > 0 {
		keepResultOneOf = validatorPools.results.Borrow()
		s.validateOneOf(data, mainResult, keepResultOneOf)
	}

	if len(s.allOfValidators) > 0 {
		keepResultAllOf = validatorPools.results.Borrow()
		s.validateAllOf(data, mainResult, keepResultAllOf)
	}

	if s.notValidator != nil {
		s.validateNot(data, mainResult)
	}

	if len(s.Dependencies) > 0 && reflect.TypeOf(data).Kind() == reflect.Map {
		s.validateDependencies(data, mainResult)
	}

	mainResult.Inc()

	// In the end we retain best failures for schema validation
	// plus, if any, composite errors which may explain special cases (tagged as IMPORTANT!).
	return mainResult.Merge(keepResultAllOf, keepResultOneOf, keepResultAnyOf)
}

func (s *schemaPropsValidator) validateAnyOf(data any, mainResult, keepResultAnyOf *Result) {
	// Validates at least one in anyOf schemas
	var bestFailures *Result

	for i, anyOfSchema := range s.anyOfValidators {
		if s.Options.recycleValidators {
			s.anyOfValidators[i] = nil // Validate redeems it, even if it panics
		}
		result := anyOfSchema.Validate(data)
		// We keep inner IMPORTANT! errors no matter what MatchCount tells us
		keepResultAnyOf.Merge(result.keepRelevantErrors()) // merges (and redeems) a new instance of Result

		if result.IsValid() {
			if bestFailures != nil && bestFailures.wantsRedeemOnMerge {
				redeemResult(bestFailures)
			}

			_ = keepResultAnyOf.cleared()
			mainResult.Merge(result)

			return
		}

		// MatchCount is used to select errors from the schema with most positive checks
		if bestFailures == nil || result.MatchCount > bestFailures.MatchCount {
			if bestFailures != nil && bestFailures.wantsRedeemOnMerge {
				redeemResult(bestFailures)
			}
			bestFailures = result

			continue
		}

		if result.wantsRedeemOnMerge {
			redeemResult(result) // this result is ditched
		}
	}

	mainResult.addErrorsAt(s.Path, mustValidateAtLeastOneSchemaMsg(s.Path.dotted()))
	mainResult.Merge(bestFailures)
}

func (s *schemaPropsValidator) validateOneOf(data any, mainResult, keepResultOneOf *Result) {
	// Validates exactly one in oneOf schemas
	var (
		firstSuccess, bestFailures *Result
		validated                  int
	)

	for i, oneOfSchema := range s.oneOfValidators {
		if s.Options.recycleValidators {
			s.oneOfValidators[i] = nil // Validate redeems it, even if it panics
		}
		result := oneOfSchema.Validate(data)

		// We keep inner IMPORTANT! errors no matter what MatchCount tells us
		keepResultOneOf.Merge(result.keepRelevantErrors()) // merges (and redeems) a new instance of Result

		if result.IsValid() {
			validated++
			_ = keepResultOneOf.cleared()

			if firstSuccess == nil {
				firstSuccess = result
			} else if result.wantsRedeemOnMerge {
				redeemResult(result) // this result is ditched
			}

			continue
		}

		// MatchCount is used to select errors from the schema with most positive checks
		if validated == 0 && (bestFailures == nil || result.MatchCount > bestFailures.MatchCount) {
			if bestFailures != nil && bestFailures.wantsRedeemOnMerge {
				redeemResult(bestFailures)
			}
			bestFailures = result
		} else if result.wantsRedeemOnMerge {
			redeemResult(result) // this result is ditched
		}
	}

	switch validated {
	case 0:
		mainResult.addErrorsAt(s.Path, mustValidateOnlyOneSchemaMsg(s.Path.dotted(), "Found none valid"))
		mainResult.Merge(bestFailures)
		// firstSucess necessarily nil
	case 1:
		mainResult.Merge(firstSuccess)
		if bestFailures != nil && bestFailures.wantsRedeemOnMerge {
			redeemResult(bestFailures)
		}
	default:
		mainResult.addErrorsAt(s.Path, mustValidateOnlyOneSchemaMsg(s.Path.dotted(), fmt.Sprintf("Found %d valid alternatives", validated)))
		mainResult.Merge(bestFailures)
		if firstSuccess != nil && firstSuccess.wantsRedeemOnMerge {
			redeemResult(firstSuccess)
		}
	}
}

func (s *schemaPropsValidator) validateAllOf(data any, mainResult, keepResultAllOf *Result) {
	// Validates all of allOf schemas
	var validated int

	for i, allOfSchema := range s.allOfValidators {
		if s.Options.recycleValidators {
			s.allOfValidators[i] = nil // Validate redeems it, even if it panics
		}
		result := allOfSchema.Validate(data)
		// We keep inner IMPORTANT! errors no matter what MatchCount tells us
		keepResultAllOf.Merge(result.keepRelevantErrors())
		if result.IsValid() {
			validated++
		}
		mainResult.Merge(result)
	}

	switch validated {
	case 0:
		mainResult.addErrorsAt(s.Path, mustValidateAllSchemasMsg(s.Path.dotted(), ". None validated"))
	case len(s.allOfValidators):
	default:
		mainResult.addErrorsAt(s.Path, mustValidateAllSchemasMsg(s.Path.dotted(), ""))
	}
}

func (s *schemaPropsValidator) validateNot(data any, mainResult *Result) {
	notValidator := s.notValidator
	if s.Options.recycleValidators {
		s.notValidator = nil // Validate redeems it, even if it panics
	}
	result := notValidator.Validate(data)
	// We keep inner IMPORTANT! errors no matter what MatchCount tells us
	if result.IsValid() {
		mainResult.addErrorsAt(s.Path, mustNotValidatechemaMsg(s.Path.dotted()))
	}
	if result.wantsRedeemOnMerge {
		redeemResult(result) // this result is ditched
	}
}

func (s *schemaPropsValidator) validateDependencies(data any, mainResult *Result) {
	val := data.(map[string]any) //nolint:forcetypeassert // caller guarantees map[string]any
	for _, key := range sortedKeys(val) {
		dep, ok := s.Dependencies[key]
		if !ok {
			continue
		}

		if dep.Schema != nil {
			mainResult.Merge(
				newSchemaValidator(dep.Schema, s.Root, s.Path.child(key), s.KnownFormats, s.Options).Validate(data),
			)
			continue
		}

		if len(dep.Property) > 0 {
			for _, depKey := range dep.Property {
				if _, ok := val[depKey]; !ok {
					mainResult.addErrorsAt(s.Path, hasADependencyMsg(s.Path.dotted(), depKey))
				}
			}
		}
	}
}

func (s *schemaPropsValidator) setPath(path pathSegments) {
	s.Path = path
}

func (s *schemaPropsValidator) redeem() {
	validatorPools.schemaPropsValidators.Redeem(s)
}

func (s *schemaPropsValidator) redeemChildren() {
	for _, v := range s.anyOfValidators {
		if v == nil {
			continue
		}
		v.redeemChildren()
		v.redeem()
	}
	s.anyOfValidators = nil

	for _, v := range s.allOfValidators {
		if v == nil {
			continue
		}
		v.redeemChildren()
		v.redeem()
	}
	s.allOfValidators = nil

	for _, v := range s.oneOfValidators {
		if v == nil {
			continue
		}
		v.redeemChildren()
		v.redeem()
	}
	s.oneOfValidators = nil

	if s.notValidator != nil {
		s.notValidator.redeemChildren()
		s.notValidator.redeem()
		s.notValidator = nil
	}
}
