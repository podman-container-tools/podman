// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package fmts

import (
	"github.com/go-openapi/swag/loading"
	"github.com/go-openapi/swag/yamlutils"
)

var (
	// YAMLMatcher matches yaml.
	YAMLMatcher = loading.YAMLMatcher
	// YAMLToJSON converts YAML unmarshaled data into json compatible data.
	YAMLToJSON = yamlutils.YAMLToJSON
	// BytesToYAMLDoc converts raw bytes to a map[string]interface{}.
	BytesToYAMLDoc = yamlutils.BytesToYAMLDoc
	// YAMLDoc loads a yaml document from either http or a file and converts it to json.
	YAMLDoc = loading.YAMLDoc
	// YAMLData loads a yaml document from either http or a file.
	YAMLData = loading.YAMLData
)
