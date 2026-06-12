// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"go/ast"
	"regexp"
	"strings"
)

var (
	rxStripComments = regexp.MustCompile(`^[^\p{L}\p{N}\p{Pd}\p{Pc}\+]*`)
	rxSpace         = regexp.MustCompile(`\p{Zs}+`)
)

type ParsedPathContent struct {
	Method, Path, ID string
	Tags             []string
	Remaining        *ast.CommentGroup
}

func ParseOperationPathAnnotation(lines []*ast.Comment) (cnt ParsedPathContent) {
	return parsePathAnnotation(rxOperation, lines)
}

func ParseRoutePathAnnotation(lines []*ast.Comment) (cnt ParsedPathContent) {
	return parsePathAnnotation(rxRoute, lines)
}

func parsePathAnnotation(annotation *regexp.Regexp, lines []*ast.Comment) (cnt ParsedPathContent) {
	const routeTagsIndex = 3 // routeTagsIndex is the regex submatch index where route tags begin.
	var justMatched bool

	for _, cmt := range lines {
		txt := cmt.Text
		for line := range strings.SplitSeq(txt, "\n") {
			matches := annotation.FindStringSubmatch(line)
			if len(matches) > routeTagsIndex {
				cnt.Method, cnt.Path, cnt.ID = matches[1], matches[2], matches[len(matches)-1]
				cnt.Tags = rxSpace.Split(matches[3], -1)
				if len(matches[3]) == 0 {
					cnt.Tags = nil
				}
				justMatched = true

				continue
			}

			if cnt.Method == "" {
				continue
			}

			if cnt.Remaining == nil {
				cnt.Remaining = new(ast.CommentGroup)
			}

			if !justMatched || strings.TrimSpace(rxStripComments.ReplaceAllString(line, "")) != "" {
				cc := new(ast.Comment)
				cc.Slash = cmt.Slash
				cc.Text = line
				cnt.Remaining.List = append(cnt.Remaining.List, cc)
				justMatched = false
			}
		}
	}

	return cnt
}
