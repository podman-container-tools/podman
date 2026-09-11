// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package list

import "strings"

// matchPattern is the predicate for a `...` pattern.
//
// The rules are the go command's, and so is the implementation: see pkgpattern.go for why that one is copied rather
// than written here.
func matchPattern(pattern string) func(name string) bool {
	return MatchPattern(pattern)
}

// patternRoot splits a pattern into the literal directory prefix to walk from and whether it is recursive at all.
//
// Everything up to the last separator before the first wildcard is literal, so `./internal/pack...` walks `./internal`
// and matches names from there.
// Walking the wildcard-bearing element itself would miss `internal/packages`, since no directory is called `pack...`.
func patternRoot(pattern string) (root string, recursive bool) {
	head, _, found := strings.Cut(pattern, "...")
	if !found {
		return pattern, false
	}

	if j := strings.LastIndex(head, "/"); j >= 0 {
		head = head[:j]
	}
	if head == "" {
		head = "."
	}

	return head, true
}
