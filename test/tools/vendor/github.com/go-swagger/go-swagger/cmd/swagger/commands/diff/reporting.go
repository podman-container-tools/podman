// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/go-openapi/spec"
)

// ArrayType const for array.
var ArrayType = "array"

// ObjectType const for object.
var ObjectType = "object"

// Compare returns the result of analysing breaking and non breaking changes
// between to Swagger specs.
func Compare(spec1, spec2 *spec.Swagger) (diffs SpecDifferences, err error) {
	analyser := NewSpecAnalyser()
	err = analyser.Analyse(spec1, spec2)
	if err != nil {
		return nil, err
	}
	diffs = analyser.Diffs
	return diffs, nil
}

// PathItemOp - combines path and operation into a single keyed entity.
type PathItemOp struct {
	ParentPathItem *spec.PathItem  `json:"pathitem"`
	Operation      *spec.Operation `json:"operation"`
	Extensions     spec.Extensions `json:"extensions"`
}

// URLMethod - combines url and method into a single keyed entity.
type URLMethod struct {
	Path   string `json:"path"`
	Method string `json:"method"`
}

// DataDirection indicates the direction of change Request vs Response.
type DataDirection int

const (
	// Request Used for messages/param diffs in a request.
	Request DataDirection = iota
	// Response Used for messages/param diffs in a response.
	Response
)

func getParams(pathParams, opParams []spec.Parameter, location string) map[string]spec.Parameter {
	params := map[string]spec.Parameter{}
	// add shared path params
	for _, eachParam := range pathParams {
		if eachParam.In == location {
			params[eachParam.Name] = eachParam
		}
	}
	// add any overridden params
	for _, eachParam := range opParams {
		if eachParam.In == location {
			params[eachParam.Name] = eachParam
		}
	}
	return params
}

func getNameOnlyDiffNode(forLocation string) *Node {
	node := Node{
		Field: forLocation,
	}
	return &node
}

func primitiveTypeString(typeName, typeFormat string) string {
	if typeFormat != "" {
		return fmt.Sprintf("%s.%s", typeName, typeFormat)
	}
	return typeName
}

// TypeDiff - describes a primitive type change.
type TypeDiff struct {
	Change      SpecChangeCode `json:"change_type,omitempty"`
	Description string         `json:"description,omitempty"`
	FromType    string         `json:"from_type,omitempty"`
	ToType      string         `json:"to_type,omitempty"`
}

const (
	numberMappedAsInt32   = 0
	numberMappedAsInt64   = 1
	numberMappedAsFloat32 = 2
	numberMappedAsFloat64 = 3
)

// didn't use 'width' so as not to confuse with bit width.
var numberWideness = map[string]int{
	"number":        numberMappedAsFloat64,
	"number.double": numberMappedAsFloat64,
	"double":        numberMappedAsFloat64,
	"number.float":  numberMappedAsFloat32,
	"float":         numberMappedAsFloat32,
	"long":          numberMappedAsInt64,
	"integer.int64": numberMappedAsInt64,
	"integer":       numberMappedAsInt32,
	"integer.int32": numberMappedAsInt32,
}

func prettyprint(b []byte) (io.ReadWriter, error) {
	var out bytes.Buffer
	err := json.Indent(&out, b, "", "  ")
	return &out, err
}

// JSONMarshal allows the item to be correctly rendered to json.
func JSONMarshal(t interface{}) ([]byte, error) {
	buffer := &bytes.Buffer{}
	encoder := json.NewEncoder(buffer)
	encoder.SetEscapeHTML(false)
	err := encoder.Encode(t)
	return buffer.Bytes(), err
}
