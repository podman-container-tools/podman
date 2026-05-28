// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	oaispec "github.com/go-openapi/spec"
)

const (
	// paramDescriptionKey indicates the tag used to define a parameter description in swagger:route.
	paramDescriptionKey = "description"
	// paramNameKey indicates the tag used to define a parameter name in swagger:route.
	paramNameKey = "name"
	// paramInKey indicates the tag used to define a parameter location in swagger:route.
	paramInKey = "in"
	// paramRequiredKey indicates the tag used to declare whether a parameter is required in swagger:route.
	paramRequiredKey = "required"
	// paramTypeKey indicates the tag used to define the parameter type in swagger:route.
	paramTypeKey = "type"
	// paramAllowEmptyKey indicates the tag used to indicate whether a parameter allows empty values in swagger:route.
	paramAllowEmptyKey = "allowempty"

	// schemaMinKey indicates the tag used to indicate the minimum value allowed for this type in swagger:route.
	schemaMinKey = "min"
	// schemaMaxKey indicates the tag used to indicate the maximum value allowed for this type in swagger:route.
	schemaMaxKey = "max"
	// schemaEnumKey indicates the tag used to specify the allowed values for this type in swagger:route.
	schemaEnumKey = "enum"
	// schemaFormatKey indicates the expected format for this field in swagger:route.
	schemaFormatKey = "format"
	// schemaDefaultKey indicates the default value for this field in swagger:route.
	schemaDefaultKey = "default"
	// schemaMinLenKey indicates the minimum length this field in swagger:route.
	schemaMinLenKey = "minlength"
	// schemaMaxLenKey indicates the minimum length this field in swagger:route.
	schemaMaxLenKey = "maxlength"

	// typeArray is the identifier for an array type in swagger:route.
	typeArray = "array"
	// typeNumber is the identifier for a number type in swagger:route.
	typeNumber = "number"
	// typeInteger is the identifier for an integer type in swagger:route.
	typeInteger = "integer"
	// typeBoolean is the identifier for a boolean type in swagger:route.
	typeBoolean = "boolean"
	// typeBool is the identifier for a boolean type in swagger:route.
	typeBool = "bool"
	// typeObject is the identifier for an object type in swagger:route.
	typeObject = "object"
	// typeString is the identifier for a string type in swagger:route.
	typeString = "string"
)

var (
	validIn    = []string{"path", "query", "header", "body", "form"}                             //nolint:gochecknoglobals // immutable lookup table
	basicTypes = []string{typeInteger, typeNumber, typeString, typeBoolean, typeBool, typeArray} //nolint:gochecknoglobals // immutable lookup table
)

type SetOpParams struct {
	set        func([]*oaispec.Parameter)
	parameters []*oaispec.Parameter
}

func NewSetParams(params []*oaispec.Parameter, setter func([]*oaispec.Parameter)) *SetOpParams {
	return &SetOpParams{
		set:        setter,
		parameters: params,
	}
}

func (s *SetOpParams) Matches(line string) bool {
	return rxParameters.MatchString(line)
}

func (s *SetOpParams) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	var current *oaispec.Parameter
	var extraData map[string]string

	for _, line := range lines {
		l := strings.TrimSpace(line)

		if strings.HasPrefix(l, "+") {
			s.finalizeParam(current, extraData)
			current = new(oaispec.Parameter)
			extraData = make(map[string]string)
			l = strings.TrimPrefix(l, "+")
		}

		kv := strings.SplitN(l, ":", kvParts)

		if len(kv) <= 1 {
			continue
		}

		key := strings.ToLower(strings.TrimSpace(kv[0]))
		value := strings.TrimSpace(kv[1])

		if current == nil {
			return fmt.Errorf("invalid route/operation schema provided: %w", ErrParser)
		}

		applyParamField(current, extraData, key, value)
	}

	s.finalizeParam(current, extraData)
	s.set(s.parameters)

	return nil
}

func applyParamField(current *oaispec.Parameter, extraData map[string]string, key, value string) {
	switch key {
	case paramDescriptionKey:
		current.Description = value
	case paramNameKey:
		current.Name = value
	case paramInKey:
		v := strings.ToLower(value)
		if contains(validIn, v) {
			current.In = v
		}
	case paramRequiredKey:
		if v, err := strconv.ParseBool(value); err == nil {
			current.Required = v
		}
	case paramTypeKey:
		if current.Schema == nil {
			current.Schema = new(oaispec.Schema)
		}
		if contains(basicTypes, value) {
			current.Type = strings.ToLower(value)
			if current.Type == typeBool {
				current.Type = typeBoolean
			}
		} else if ref, err := oaispec.NewRef("#/definitions/" + value); err == nil {
			current.Type = typeObject
			current.Schema.Ref = ref
		}
		current.Schema.Type = oaispec.StringOrArray{current.Type}
	case paramAllowEmptyKey:
		if v, err := strconv.ParseBool(value); err == nil {
			current.AllowEmptyValue = v
		}
	default:
		extraData[key] = value
	}
}

func (s *SetOpParams) finalizeParam(param *oaispec.Parameter, data map[string]string) {
	if param == nil {
		return
	}

	processSchema(data, param)

	// schema is only allowed for parameters in "body"
	// see https://swagger.io/specification/v2/#parameterObject
	switch {
	case param.In == "body":
		param.Type = ""

	case param.Schema != nil:
		// convert schema into validations
		param.SetValidations(param.Schema.Validations())
		param.Default = param.Schema.Default
		param.Format = param.Schema.Format
		param.Schema = nil
	}

	s.parameters = append(s.parameters, param)
}

func processSchema(data map[string]string, param *oaispec.Parameter) {
	if param.Schema == nil {
		return
	}

	var enumValues []string

	for key, value := range data {
		switch key {
		case schemaMinKey:
			if t := getType(param.Schema); t == typeNumber || t == typeInteger {
				v, _ := strconv.ParseFloat(value, 64)
				param.Schema.Minimum = &v
			}
		case schemaMaxKey:
			if t := getType(param.Schema); t == typeNumber || t == typeInteger {
				v, _ := strconv.ParseFloat(value, 64)
				param.Schema.Maximum = &v
			}
		case schemaMinLenKey:
			if getType(param.Schema) == typeArray {
				v, _ := strconv.ParseInt(value, 10, 64)
				param.Schema.MinLength = &v
			}
		case schemaMaxLenKey:
			if getType(param.Schema) == typeArray {
				v, _ := strconv.ParseInt(value, 10, 64)
				param.Schema.MaxLength = &v
			}
		case schemaEnumKey:
			enumValues = strings.Split(value, ",")
		case schemaFormatKey:
			param.Schema.Format = value
		case schemaDefaultKey:
			param.Schema.Default = convert(param.Type, value)
		}
	}

	if param.Description != "" {
		param.Schema.Description = param.Description
	}

	convertEnum(param.Schema, enumValues)
}

func convertEnum(schema *oaispec.Schema, enumValues []string) {
	if len(enumValues) == 0 {
		return
	}

	finalEnum := make([]any, 0, len(enumValues))
	for _, v := range enumValues {
		finalEnum = append(finalEnum, convert(schema.Type[0], strings.TrimSpace(v)))
	}
	schema.Enum = finalEnum
}

func convert(typeStr, valueStr string) any {
	switch typeStr {
	case typeInteger:
		fallthrough
	case typeNumber:
		if num, err := strconv.ParseFloat(valueStr, 64); err == nil {
			return num
		}
	case typeBoolean:
		fallthrough
	case typeBool:
		if b, err := strconv.ParseBool(valueStr); err == nil {
			return b
		}
	}
	return valueStr
}

func getType(schema *oaispec.Schema) string {
	if len(schema.Type) == 0 {
		return ""
	}
	return schema.Type[0]
}

func contains(arr []string, obj string) bool {
	return slices.Contains(arr, obj)
}
