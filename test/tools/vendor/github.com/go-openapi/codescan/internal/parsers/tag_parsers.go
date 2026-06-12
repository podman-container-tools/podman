// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import "github.com/go-openapi/codescan/internal/ifaces"

// TagParser pairs a named tag with a [ifaces.ValueParser] that recognizes and
// extracts its value from comment lines.
//
// A TagParser operates in one of two modes:
//
//   - Single-line: the tag matches exactly one line (e.g. "maximum: 10").
//     The [SectionedParser] resets its current tagger after every single-line
//     match, so the next line is free to match a different tagger.
//
//   - Multi-line: the tag's first matching line is a header (e.g. "consumes:")
//     and all subsequent lines are collected as its body until a different
//     tagger matches or the comment block ends. The header line itself is NOT
//     included in Lines — only the body lines that follow it.
//
// SkipCleanUp controls whether the [SectionedParser] strips comment prefixes
// (// , *, etc.) from the collected Lines before calling Parse. YAML-based
// taggers set this to true because they need the original indentation intact.
//
// Lines is populated by the [SectionedParser] during its scan; after the scan
// completes, Parse is called with those lines to extract the value.
type TagParser struct {
	Name        string
	MultiLine   bool
	SkipCleanUp bool
	Lines       []string
	Parser      ifaces.ValueParser
}

// NewMultiLineTagParser creates a TagParser that collects all lines following
// the matching header until a different tag or annotation is encountered.
//
// Example usage (from [NewMetaParser]):
//
//	NewMultiLineTagParser("TOS",
//	    newMultilineDropEmptyParser(rxTOS, metaTOSSetter(info)),
//	    false, // clean up comment prefixes before parsing
//	)
//
// This creates a tagger that recognizes "Terms of Service:" and collects every
// subsequent line into the TOS field, stripping comment prefixes.
func NewMultiLineTagParser(name string, parser ifaces.ValueParser, skipCleanUp bool) TagParser {
	return TagParser{
		Name:        name,
		MultiLine:   true,
		SkipCleanUp: skipCleanUp,
		Parser:      parser,
	}
}

// NewSingleLineTagParser creates a TagParser that matches and parses exactly
// one line. After the match, the [SectionedParser] resets its current tagger
// so subsequent lines can match other taggers.
//
// Example usage (from [NewMetaParser]):
//
//	NewSingleLineTagParser("Version",
//	    &setMetaSingle{Spec: swspec, Rx: rxVersion, Set: setInfoVersion},
//	)
//
// This creates a tagger that recognizes "Version: 1.0.0" and writes the
// captured value into swspec.Info.Version.
func NewSingleLineTagParser(name string, parser ifaces.ValueParser) TagParser {
	return TagParser{
		Name:        name,
		MultiLine:   false,
		SkipCleanUp: false,
		Parser:      parser,
	}
}

// Matches delegates to the underlying Parser.
func (st *TagParser) Matches(line string) bool {
	return st.Parser.Matches(line)
}

// Parse delegates to the underlying Parser.
func (st *TagParser) Parse(lines []string) error {
	return st.Parser.Parse(lines)
}
