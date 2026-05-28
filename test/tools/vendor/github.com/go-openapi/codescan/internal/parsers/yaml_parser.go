// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/go-openapi/loads/fmts"
	"go.yaml.in/yaml/v3"
)

type YAMLParserOption func(*YAMLParser)

func WithSetter(set func(json.RawMessage) error) YAMLParserOption {
	return func(p *YAMLParser) {
		p.set = set
	}
}

func WithMatcher(rx *regexp.Regexp) YAMLParserOption {
	return func(p *YAMLParser) {
		p.rx = rx
	}
}

func WithExtensionMatcher() YAMLParserOption {
	return func(p *YAMLParser) {
		p.rx = rxExtensions
	}
}

type YAMLParser struct {
	set func(json.RawMessage) error
	rx  *regexp.Regexp
}

func NewYAMLParser(opts ...YAMLParserOption) *YAMLParser {
	var y YAMLParser
	for _, apply := range opts {
		apply(&y)
	}

	return &y
}

func (y *YAMLParser) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	uncommented := make([]string, 0, len(lines))
	uncommented = append(uncommented, removeYamlIndent(lines)...)

	yamlContent := strings.Join(uncommented, "\n")
	var yamlValue any
	err := yaml.Unmarshal([]byte(yamlContent), &yamlValue)
	if err != nil {
		return err
	}

	var jsonValue json.RawMessage
	jsonValue, err = fmts.YAMLToJSON(yamlValue)
	if err != nil {
		return err
	}

	if y.set == nil {
		return nil
	}

	return y.set(jsonValue)
}

func (y *YAMLParser) Matches(line string) bool {
	if y.rx == nil {
		return false
	}

	return y.rx.MatchString(line)
}

// removes indent base on the first line.
//
// The difference with removeIndent is that lines shorter than the indentation are elided.
func removeYamlIndent(spec []string) []string {
	if len(spec) == 0 {
		return spec
	}

	loc := rxIndent.FindStringIndex(spec[0])
	if len(loc) < 2 || loc[1] <= 1 {
		return spec
	}

	s := make([]string, 0, len(spec))
	for i := range spec {
		if len(spec[i]) >= loc[1] {
			s = append(s, spec[i][loc[1]-1:])
		}
	}

	return s
}
