// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the NOTICE file.
//
// SPDX-License-Identifier: BSD-3-Clause

// Derived from cmd/internal/pkgpattern/pkgpattern.go at go1.26.5. The three declarations below are byte-identical to
// upstream; what was dropped is the entry points codescan has no use for (MatchSimplePattern, TreeCanMatchPattern,
// hasPathPrefix).
//
// Copied rather than reimplemented because the semantics are subtler than they look — the vendor rules below are the
// part everyone gets wrong — and because the regexp construction is deliberate: the upstream comment notes that the
// obvious hand-written glob matcher "is too easy to make accidentally exponential", while this one is linear by
// construction.
//
// It is kept verbatim, and excluded from our linters in .golangci.yml, so that re-syncing with a future Go release
// stays a diff rather than a merge. pkgpattern_test.go runs the upstream table against it.

package list

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// MatchPattern(pattern)(name) reports whether name matches pattern.
//
// Pattern is a limited glob pattern in which '...' means 'any string' and there is no other special syntax.
// Unfortunately, there are two special cases.
// Quoting "go help packages":
//
// First, /... at the end of the pattern can match an empty string, so that net/... matches both net and packages in its
// subdirectories, like net/http.
//
// Second, any slash-separated pattern element containing a wildcard never participates in a match of the "vendor"
// element in the path of a vendored package, so that ./... does not match packages in subdirectories of ./vendor or
// ./mycode/vendor, but ./vendor/... and ./mycode/vendor/... do.
//
// Note, however, that a directory named vendor that itself contains code is not a vendored package: cmd/vendor would be
// a command named vendor, and the pattern cmd/... matches it.
func MatchPattern(pattern string) func(name string) bool {
	return matchPatternInternal(pattern, true)
}

func matchPatternInternal(pattern string, vendorExclude bool) func(name string) bool {
	// Convert pattern to regular expression.
	// The strategy for the trailing /... is to nest it in an explicit ? expression.
	// The strategy for the vendor exclusion is to change the unmatchable vendor strings to a disallowed code point
	// (vendorChar) and to use "(anything but that codepoint)*" as the implementation of the ... wildcard.
	//
	// This is a bit complicated but the obvious alternative, namely a hand-written search like in most shell glob
	// matchers, is too easy to make accidentally exponential.
	// Using package regexp guarantees linear-time matching.

	const vendorChar = "\x00"

	if vendorExclude && strings.Contains(pattern, vendorChar) || !utf8.ValidString(pattern) {
		return func(name string) bool { return false }
	}

	re := regexp.QuoteMeta(pattern)
	wild := `.*`
	if vendorExclude {
		wild = `[^` + vendorChar + `]*`
		re = replaceVendor(re, vendorChar)
		switch {
		case strings.HasSuffix(re, `/`+vendorChar+`/\.\.\.`):
			re = strings.TrimSuffix(re, `/`+vendorChar+`/\.\.\.`) + `(/vendor|/` + vendorChar + `/\.\.\.)`
		case re == vendorChar+`/\.\.\.`:
			re = `(/vendor|/` + vendorChar + `/\.\.\.)`
		}
	}
	if strings.HasSuffix(re, `/\.\.\.`) {
		re = strings.TrimSuffix(re, `/\.\.\.`) + `(/\.\.\.)?`
	}
	re = strings.ReplaceAll(re, `\.\.\.`, wild)

	reg := regexp.MustCompile(`^` + re + `$`)

	return func(name string) bool {
		if vendorExclude {
			if strings.Contains(name, vendorChar) {
				return false
			}
			name = replaceVendor(name, vendorChar)
		}
		return reg.MatchString(name)
	}
}

// replaceVendor returns the result of replacing non-trailing vendor path elements in x with repl.
func replaceVendor(x, repl string) string {
	if !strings.Contains(x, "vendor") {
		return x
	}
	elem := strings.Split(x, "/")
	for i := 0; i < len(elem)-1; i++ {
		if elem[i] == "vendor" {
			elem[i] = repl
		}
	}
	return strings.Join(elem, "/")
}
