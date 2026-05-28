// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"regexp"
	"strconv"

	oaispec "github.com/go-openapi/spec"
)

const (
	// kvParts is the number of parts when splitting key:value pairs.
	kvParts = 2
)

// Many thanks go to https://github.com/yvasiyarov/swagger
// this is loosely based on that implementation but for swagger 2.0

type matchOnlyParam struct {
	rx *regexp.Regexp
}

func (mo *matchOnlyParam) Matches(line string) bool {
	return mo.rx.MatchString(line)
}

func (mo *matchOnlyParam) Parse(_ []string) error {
	return nil
}

type MatchParamIn struct {
	*matchOnlyParam
}

func NewMatchParamIn(_ *oaispec.Parameter) *MatchParamIn {
	return NewMatchIn()
}

// NewMatchIn returns a match-only tagger that claims `in: <location>`
// lines. The `in:` directive is extracted separately via
// parsers.ParamLocation; this tagger only prevents the line from
// being absorbed into the surrounding description by a SectionedParser.
func NewMatchIn() *MatchParamIn {
	return &MatchParamIn{
		matchOnlyParam: &matchOnlyParam{
			rx: rxIn,
		},
	}
}

type MatchParamRequired struct {
	*matchOnlyParam
}

func NewMatchParamRequired(_ *oaispec.Parameter) *MatchParamRequired {
	return &MatchParamRequired{
		matchOnlyParam: &matchOnlyParam{
			rx: rxRequired,
		},
	}
}

type SetDeprecatedOp struct {
	tgt *oaispec.Operation
}

func NewSetDeprecatedOp(operation *oaispec.Operation) *SetDeprecatedOp {
	return &SetDeprecatedOp{
		tgt: operation,
	}
}

func (su *SetDeprecatedOp) Matches(line string) bool {
	return rxDeprecated.MatchString(line)
}

func (su *SetDeprecatedOp) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	matches := rxDeprecated.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		req, err := strconv.ParseBool(matches[1])
		if err != nil {
			return err
		}
		su.tgt.Deprecated = req
	}

	return nil
}

type ConsumesDropEmptyParser struct {
	*multilineDropEmptyParser
}

func NewConsumesDropEmptyParser(set func([]string)) *ConsumesDropEmptyParser {
	return &ConsumesDropEmptyParser{
		multilineDropEmptyParser: &multilineDropEmptyParser{
			set: set,
			rx:  rxConsumes,
		},
	}
}

type ProducesDropEmptyParser struct {
	*multilineDropEmptyParser
}

func NewProducesDropEmptyParser(set func([]string)) *ProducesDropEmptyParser {
	return &ProducesDropEmptyParser{
		multilineDropEmptyParser: &multilineDropEmptyParser{
			set: set,
			rx:  rxProduces,
		},
	}
}

type multilineDropEmptyParser struct {
	set func([]string)
	rx  *regexp.Regexp
}

func newMultilineDropEmptyParser(rx *regexp.Regexp, set func([]string)) *multilineDropEmptyParser {
	return &multilineDropEmptyParser{
		set: set,
		rx:  rx,
	}
}

func (m *multilineDropEmptyParser) Matches(line string) bool {
	return m.rx.MatchString(line)
}

func (m *multilineDropEmptyParser) Parse(lines []string) error {
	m.set(removeEmptyLines(lines))

	return nil
}
