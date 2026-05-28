// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import "strings"

func JoinDropLast(lines []string) string {
	l := len(lines)
	lns := lines
	if l > 0 && len(strings.TrimSpace(lines[l-1])) == 0 {
		lns = lines[:l-1]
	}
	return strings.Join(lns, "\n")
}

// Setter sets a string field from a multi lines comment.
//
// Usage:
//
//	Setter(&op.Description)
//	Setter(&op.Summary)
//
// Replaces this idiom:
//
//	parsers.WithSetDescription(func(lines []string) { op.Description = parsers.JoinDropLast(lines) }),
func Setter(target *string) func([]string) {
	return func(lines []string) {
		*target = JoinDropLast(lines)
	}
}

func removeEmptyLines(lines []string) []string {
	notEmpty := make([]string, 0, len(lines))

	for _, l := range lines {
		if len(strings.TrimSpace(l)) > 0 {
			notEmpty = append(notEmpty, l)
		}
	}

	return notEmpty
}
