// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"go/ast"
	"regexp"
	"slices"
	"strings"

	"github.com/go-openapi/codescan/internal/ifaces"
)

const minMatchCount = 2

func HasAnnotation(line string) bool {
	return rxSwaggerAnnotation.MatchString(line)
}

func IsAliasParam(prop ifaces.SwaggerTypable) bool {
	in := prop.In()
	return in == "query" || in == "path" || in == "formData"
}

func IsAllowedExtension(ext string) bool {
	return rxAllowedExtensions.MatchString(ext)
}

func ExtractAnnotation(line string) (string, bool) {
	matches := rxSwaggerAnnotation.FindStringSubmatch(line)
	if len(matches) < minMatchCount {
		return "", false
	}

	return matches[1], true
}

func AllOfMember(comments *ast.CommentGroup) bool {
	return commentMatcher(rxAllOf)(comments)
}

func FileParam(comments *ast.CommentGroup) bool {
	return commentMatcher(rxFileUpload)(comments)
}

func Ignored(comments *ast.CommentGroup) bool {
	return commentMatcher(rxIgnoreOverride)(comments)
}

func AliasParam(comments *ast.CommentGroup) bool {
	return commentMatcher(rxAlias)(comments)
}

func StrfmtName(comments *ast.CommentGroup) (string, bool) {
	return commentSubMatcher(rxStrFmt)(comments)
}

func ParamLocation(comments *ast.CommentGroup) (string, bool) {
	return commentSubMatcher(rxIn)(comments)
}

func EnumName(comments *ast.CommentGroup) (string, bool) {
	return commentSubMatcher(rxEnum)(comments)
}

func AllOfName(comments *ast.CommentGroup) (string, bool) {
	return commentSubMatcher(rxAllOf)(comments)
}

func NameOverride(comments *ast.CommentGroup) (string, bool) {
	return commentSubMatcher(rxName)(comments)
}

func DefaultName(comments *ast.CommentGroup) (string, bool) {
	return commentSubMatcher(rxDefault)(comments)
}

func TypeName(comments *ast.CommentGroup) (string, bool) {
	return commentSubMatcher(rxType)(comments)
}

func ModelOverride(comments *ast.CommentGroup) (string, bool) {
	return commentBlankSubMatcher(rxModelOverride)(comments)
}

func ResponseOverride(comments *ast.CommentGroup) (string, bool) {
	return commentBlankSubMatcher(rxResponseOverride)(comments)
}

func ParametersOverride(comments *ast.CommentGroup) ([]string, bool) {
	return commentMultipleSubMatcher(rxParametersOverride)(comments)
}

func commentMatcher(rx *regexp.Regexp) func(*ast.CommentGroup) bool {
	return func(comments *ast.CommentGroup) bool {
		if comments == nil {
			return false
		}

		return slices.ContainsFunc(comments.List, func(cmt *ast.Comment) bool {
			for ln := range strings.SplitSeq(cmt.Text, "\n") {
				if rx.MatchString(ln) {
					return true
				}
			}

			return false
		})
	}
}

func commentSubMatcher(rx *regexp.Regexp) func(*ast.CommentGroup) (string, bool) {
	return func(comments *ast.CommentGroup) (string, bool) {
		if comments == nil {
			return "", false
		}

		for _, cmt := range comments.List {
			for ln := range strings.SplitSeq(cmt.Text, "\n") {
				matches := rx.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					return strings.TrimSpace(matches[1]), true
				}
			}
		}

		return "", false
	}
}

// same as commentSubMatcher but returns true if a bare annotation is found, even without an empty submatch.
func commentBlankSubMatcher(rx *regexp.Regexp) func(*ast.CommentGroup) (string, bool) {
	return func(comments *ast.CommentGroup) (string, bool) {
		if comments == nil {
			return "", false
		}
		var found bool

		for _, cmt := range comments.List {
			for ln := range strings.SplitSeq(cmt.Text, "\n") {
				matches := rx.FindStringSubmatch(ln)
				if len(matches) > 1 && len(strings.TrimSpace(matches[1])) > 0 {
					return strings.TrimSpace(matches[1]), true
				}
				if len(matches) > 0 {
					found = true
				}
			}
		}

		return "", found
	}
}

func commentMultipleSubMatcher(rx *regexp.Regexp) func(*ast.CommentGroup) ([]string, bool) {
	return func(comments *ast.CommentGroup) ([]string, bool) {
		if comments == nil {
			return nil, false
		}

		var result []string
		for _, cmt := range comments.List {
			for ln := range strings.SplitSeq(cmt.Text, "\n") {
				matches := rx.FindStringSubmatch(ln)
				if len(matches) < minMatchCount {
					continue
				}
				trimmed := strings.TrimSpace(matches[1])
				if len(trimmed) == 0 {
					continue
				}

				result = append(result, trimmed)
			}
		}

		return result, len(result) > 0
	}
}
