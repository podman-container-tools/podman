// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package codescan

import (
	"strings"
)

// a shared function that can be used to split given headers
// into a title and description.
func collectScannerTitleDescription(headers []string) (title, desc []string) {
	hdrs := cleanupScannerLines(headers, rxUncommentHeaders)

	idx := -1
	for i, line := range hdrs {
		if strings.TrimSpace(line) == "" {
			idx = i
			break
		}
	}

	if idx > -1 {
		title = hdrs[:idx]
		if len(title) > 0 {
			title[0] = rxTitleStart.ReplaceAllString(title[0], "")
		}
		if len(hdrs) > idx+1 {
			desc = hdrs[idx+1:]
		} else {
			desc = nil
		}
		return title, desc
	}

	if len(hdrs) > 0 {
		line := hdrs[0]
		switch {
		case rxPunctuationEnd.MatchString(line):
			title = []string{line}
			desc = hdrs[1:]
		case rxTitleStart.MatchString(line):
			title = []string{rxTitleStart.ReplaceAllString(line, "")}
			desc = hdrs[1:]
		default:
			desc = hdrs
		}
	}

	return title, desc
}
