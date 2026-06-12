// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"regexp"
	"strings"

	"github.com/go-openapi/loads/fmts"
	"go.yaml.in/yaml/v3"
)

// YAMLSpecScanner aggregates lines in header until it sees `---`,
// the beginning of a YAML spec.
type YAMLSpecScanner struct {
	header         []string
	yamlSpec       []string
	setTitle       func([]string)
	setDescription func([]string)
	workedOutTitle bool
	title          []string
	skipHeader     bool
}

func NewYAMLSpecScanner(setTitle func([]string), setDescription func([]string)) *YAMLSpecScanner {
	return &YAMLSpecScanner{
		setTitle:       setTitle,
		setDescription: setDescription,
	}
}

func (sp *YAMLSpecScanner) Title() []string {
	sp.collectTitleDescription()
	return sp.title
}

func (sp *YAMLSpecScanner) Description() []string {
	sp.collectTitleDescription()
	return sp.header
}

func (sp *YAMLSpecScanner) Parse(doc *ast.CommentGroup) error {
	if doc == nil {
		return nil
	}
	var startedYAMLSpec bool
COMMENTS:
	for _, c := range doc.List {
		for line := range strings.SplitSeq(c.Text, "\n") {
			if HasAnnotation(line) {
				break COMMENTS // a new swagger: annotation terminates this parser
			}

			if !startedYAMLSpec {
				if rxBeginYAMLSpec.MatchString(line) {
					startedYAMLSpec = true
					sp.yamlSpec = append(sp.yamlSpec, line)
					continue
				}

				if !sp.skipHeader {
					sp.header = append(sp.header, line)
				}

				// no YAML spec yet, moving on
				continue
			}

			sp.yamlSpec = append(sp.yamlSpec, line)
		}
	}
	if sp.setTitle != nil {
		sp.setTitle(sp.Title())
	}
	if sp.setDescription != nil {
		sp.setDescription(sp.Description())
	}
	return nil
}

func (sp *YAMLSpecScanner) UnmarshalSpec(u func([]byte) error) (err error) {
	specYaml := cleanupScannerLines(sp.yamlSpec, rxUncommentYAML)
	if len(specYaml) == 0 {
		return fmt.Errorf("no spec available to unmarshal: %w", ErrParser)
	}

	if !strings.Contains(specYaml[0], "---") {
		return fmt.Errorf("yaml spec has to start with `---`: %w", ErrParser)
	}

	// remove indentation
	specYaml = removeIndent(specYaml)

	// 1. parse yaml lines
	yamlValue := make(map[any]any)

	yamlContent := strings.Join(specYaml, "\n")
	err = yaml.Unmarshal([]byte(yamlContent), &yamlValue)
	if err != nil {
		return err
	}

	// 2. convert to json
	var jsonValue json.RawMessage
	jsonValue, err = fmts.YAMLToJSON(yamlValue)
	if err != nil {
		return err
	}

	// 3. unmarshal the json into an interface
	var data []byte
	data, err = jsonValue.MarshalJSON()
	if err != nil {
		return err
	}
	err = u(data)
	if err != nil {
		return err
	}

	// all parsed, returning...
	sp.yamlSpec = nil // spec is now consumed, so let's erase the parsed lines

	return nil
}

func (sp *YAMLSpecScanner) collectTitleDescription() {
	if sp.workedOutTitle {
		return
	}
	if sp.setTitle == nil {
		sp.header = cleanupScannerLines(sp.header, rxUncommentHeaders)
		return
	}

	sp.workedOutTitle = true
	sp.title, sp.header = collectScannerTitleDescription(sp.header)
}

// removes indent based on the first line.
func removeIndent(spec []string) []string {
	if len(spec) == 0 {
		return spec
	}

	loc := rxIndent.FindStringIndex(spec[0])
	if len(loc) < 2 || loc[1] <= 1 {
		return spec
	}

	s := make([]string, len(spec))
	copy(s, spec)

	for i := range s {
		if len(s[i]) < loc[1] {
			continue
		}

		s[i] = spec[i][loc[1]-1:] //nolint:gosec // G602: bounds already checked on line 445
		start := rxNotIndent.FindStringIndex(s[i])
		if len(start) < 2 || start[1] == 0 {
			continue
		}

		s[i] = strings.Replace(s[i], "\t", "  ", start[1])
	}

	return s
}

func cleanupScannerLines(lines []string, ur *regexp.Regexp) []string {
	// bail early when there is nothing to parse
	if len(lines) == 0 {
		return lines
	}

	seenLine := -1
	var lastContent int

	uncommented := make([]string, 0, len(lines))
	for i, v := range lines {
		str := ur.ReplaceAllString(v, "")
		uncommented = append(uncommented, str)
		if str != "" {
			if seenLine < 0 {
				seenLine = i
			}
			lastContent = i
		}
	}

	// fixes issue #50
	if seenLine == -1 {
		return nil
	}

	return uncommented[seenLine : lastContent+1]
}
