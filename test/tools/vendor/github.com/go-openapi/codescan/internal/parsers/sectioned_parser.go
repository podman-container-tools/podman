// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"go/ast"
	"strings"

	"github.com/go-openapi/codescan/internal/ifaces"
)

// SectionedParserOption configures a [SectionedParser] via [NewSectionedParser].
type SectionedParserOption func(*SectionedParser)

// WithSetTitle provides a callback that receives the extracted title lines
// after parsing completes. If no title callback is set, the parser does not
// attempt to separate the title from the description.
func WithSetTitle(setTitle func([]string)) SectionedParserOption {
	return func(p *SectionedParser) {
		p.setTitle = setTitle
	}
}

// WithSetDescription provides a callback that receives the extracted
// description lines after parsing completes.
func WithSetDescription(setDescription func([]string)) SectionedParserOption {
	return func(p *SectionedParser) {
		p.setDescription = setDescription
	}
}

// WithTaggers registers the [TagParser] instances that this SectionedParser
// will try to match against each line after the header section ends.
func WithTaggers(taggers ...TagParser) SectionedParserOption {
	return func(p *SectionedParser) {
		p.taggers = taggers
	}
}

// SectionedParser is the core comment-block parser for go-swagger annotations.
// It processes an [ast.CommentGroup] and splits its content into three sections:
//
//  1. Header — free-form text at the top of the comment block, later split
//     into a title and description.
//  2. Tags — structured key:value lines (e.g. "minimum: 10", "consumes:",
//     "schemes: http, https") recognized by registered [TagParser] instances.
//  3. Annotation — an optional swagger:* annotation line (e.g. "swagger:model
//     Foo") handled by a dedicated [ifaces.ValueParser].
//
// # Parsing algorithm
//
// Parse walks each line of the comment block in order. For every line:
//
//  1. If the line contains a swagger:* annotation:
//     - "swagger:ignore" → mark as ignored, stop parsing.
//     - If an annotation parser is registered and matches → delegate to it.
//     - Otherwise → stop parsing (the annotation belongs to a different parser).
//
//  2. If any registered [TagParser] matches the line:
//     - For a single-line tagger: collect the line, then reset the current
//     tagger so the next line can match a different tag.
//     - For a multi-line tagger: the matching (header) line is consumed but NOT
//     collected; all subsequent lines are collected into that tagger until a
//     different tagger matches or the block ends.
//
//  3. Otherwise, if no tag has been seen yet, the line is appended to the
//     header (free-form text).
//
// After the line walk completes, three things happen:
//
//  1. The header is split into title + description (see [collectScannerTitleDescription]).
//  2. For each matched tagger, its collected lines are cleaned up (comment
//     prefixes stripped, unless SkipCleanUp is set) and passed to the
//     tagger's Parse method, which writes the extracted value into the target
//     spec object.
//  3. Title and description callbacks are invoked.
//
// # Example: Swagger meta block
//
// Given the comment block on a package doc.go:
//
//	// Petstore API.
//	//
//	// The purpose of this application is to provide an API for pets.
//	//
//	// Schemes: http, https
//	// Host: petstore.example.com
//	// BasePath: /v2
//	// Version: 1.0.0
//	// License: MIT http://opensource.org/licenses/MIT
//	// Contact: John Doe <john@example.com> http://john.example.com
//	//
//	// Consumes:
//	// - application/json
//	// - application/xml
//	//
//	// swagger:meta
//
// The SectionedParser (configured by [NewMetaParser]) will:
//
//   - Collect "Petstore API." as the title, and the next paragraph as the
//     description (header section, lines 1-3).
//   - Match "Schemes: http, https" via the single-line "Schemes" tagger.
//   - Match "Host: ...", "BasePath: ...", etc. via their respective single-line taggers.
//   - Match "Consumes:" via the multi-line "Consumes" tagger, collecting
//     "- application/json" and "- application/xml" as its body.
//   - Stop at "swagger:meta" (an annotation that doesn't match any registered
//     annotation parser, so it terminates the block).
type SectionedParser struct {
	header     []string
	matched    map[string]TagParser
	annotation ifaces.ValueParser

	seenTag        bool
	skipHeader     bool
	setTitle       func([]string)
	setDescription func([]string)
	workedOutTitle bool
	taggers        []TagParser
	currentTagger  *TagParser
	title          []string
	ignored        bool
}

// NewSectionedParser creates a SectionedParser configured by the given options.
//
// At minimum, callers should provide [WithSetTitle] and [WithTaggers]:
//
//	sp := NewSectionedParser(
//	    WithSetTitle(func(lines []string) { op.Summary = JoinDropLast(lines) }),
//	    WithSetDescription(func(lines []string) { op.Description = JoinDropLast(lines) }),
//	    WithTaggers(
//	        NewSingleLineTagParser("maximum", NewSetMaximum(builder)),
//	        NewMultiLineTagParser("consumes", NewConsumesDropEmptyParser(setter), false),
//	    ),
//	)
func NewSectionedParser(opts ...SectionedParserOption) *SectionedParser {
	var p SectionedParser

	for _, apply := range opts {
		apply(&p)
	}

	return &p
}

// Title returns the title lines extracted from the header. The title is
// separated from the description by the first blank line, or inferred from
// punctuation and markdown heading prefixes when there is no blank line.
//
// Title triggers lazy title/description splitting on first call.
func (st *SectionedParser) Title() []string {
	st.collectTitleDescription()
	return st.title
}

// Description returns the description lines extracted from the header (everything
// after the title). Like [SectionedParser.Title], it triggers lazy splitting on first call.
func (st *SectionedParser) Description() []string {
	st.collectTitleDescription()
	return st.header
}

// Ignored reports whether a "swagger:ignore" annotation was encountered.
func (st *SectionedParser) Ignored() bool {
	return st.ignored
}

// Parse processes an [ast.CommentGroup] through the sectioned parsing algorithm
// described in the type documentation. Returns an error if any matched tagger's
// Parse method fails.
func (st *SectionedParser) Parse(doc *ast.CommentGroup) error {
	if doc == nil {
		return nil
	}

COMMENTS:
	for _, c := range doc.List {
		for line := range strings.SplitSeq(c.Text, "\n") {
			if st.parseLine(line) {
				break COMMENTS
			}
		}
	}

	if st.setTitle != nil {
		st.setTitle(st.Title())
	}

	if st.setDescription != nil {
		st.setDescription(st.Description())
	}

	for _, mt := range st.matched {
		if !mt.SkipCleanUp {
			mt.Lines = cleanupScannerLines(mt.Lines, rxUncommentHeaders)
		}
		if err := mt.Parse(mt.Lines); err != nil {
			return err
		}
	}

	return nil
}

// parseLine processes a single comment line. It returns true when the
// caller should stop processing further comments (a swagger: annotation
// that doesn't belong to this parser, or swagger:ignore).
func (st *SectionedParser) parseLine(line string) (stop bool) {
	// Step 1: check for swagger:* annotations.
	if rxSwaggerAnnotation.MatchString(line) {
		if rxIgnoreOverride.MatchString(line) {
			st.ignored = true
			return true // an explicit ignore terminates this parser
		}
		if st.annotation == nil || !st.annotation.Matches(line) {
			return true // a new swagger: annotation terminates this parser
		}

		_ = st.annotation.Parse([]string{line})
		if len(st.header) > 0 {
			st.seenTag = true
		}
		return false
	}

	// Step 2: try to match a registered tagger.
	var matched bool
	for _, tg := range st.taggers {
		tagger := tg
		if tagger.Matches(line) {
			st.seenTag = true
			st.currentTagger = &tagger
			matched = true
			break
		}
	}

	// Step 3: no tagger active → accumulate as header (free-form text).
	if st.currentTagger == nil {
		if !st.skipHeader && !st.seenTag {
			st.header = append(st.header, line)
		}
		return false
	}

	// For multi-line taggers, the header line (the one that matched) is
	// consumed but not collected — only subsequent lines are body.
	if st.currentTagger.MultiLine && matched {
		return false
	}

	// Collect the line into the matched tagger's line buffer.
	ts, ok := st.matched[st.currentTagger.Name]
	if !ok {
		ts = *st.currentTagger
	}
	ts.Lines = append(ts.Lines, line)
	if st.matched == nil {
		st.matched = make(map[string]TagParser)
	}
	st.matched[st.currentTagger.Name] = ts

	// Single-line taggers reset immediately; multi-line taggers stay active.
	if !st.currentTagger.MultiLine {
		st.currentTagger = nil
	}
	return false
}

// collectTitleDescription lazily splits the accumulated header lines into
// title and description. The split is performed at most once.
//
// When setTitle is nil (no title callback registered), the header is only
// cleaned up (comment prefixes removed) but not split — everything stays
// in the description.
func (st *SectionedParser) collectTitleDescription() {
	if st.workedOutTitle {
		return
	}
	if st.setTitle == nil {
		st.header = cleanupScannerLines(st.header, rxUncommentHeaders)
		return
	}

	st.workedOutTitle = true
	st.title, st.header = collectScannerTitleDescription(st.header)
}
