// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package parsers

import (
	"regexp"
	"strings"
)

type SetSchemes struct {
	set func([]string)
	rx  *regexp.Regexp
}

func NewSetSchemes(set func([]string)) *SetSchemes {
	return &SetSchemes{
		set: set,
		rx:  rxSchemes,
	}
}

func (ss *SetSchemes) Matches(line string) bool {
	return ss.rx.MatchString(line)
}

func (ss *SetSchemes) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	matches := ss.rx.FindStringSubmatch(lines[0])
	if len(matches) > 1 && len(matches[1]) > 0 {
		sch := strings.Split(matches[1], ", ")

		schemes := []string{}
		for _, s := range sch {
			ts := strings.TrimSpace(s)
			if ts != "" {
				schemes = append(schemes, ts)
			}
		}
		ss.set(schemes)
	}

	return nil
}

type SetSecurity struct {
	set func([]map[string][]string)
	rx  *regexp.Regexp
}

func newSetSecurity(rx *regexp.Regexp, setter func([]map[string][]string)) *SetSecurity {
	return &SetSecurity{
		set: setter,
		rx:  rx,
	}
}

func NewSetSecurityScheme(setter func([]map[string][]string)) *SetSecurity {
	return &SetSecurity{
		set: setter,
		rx:  rxSecuritySchemes,
	}
}

func (ss *SetSecurity) Matches(line string) bool {
	return ss.rx.MatchString(line)
}

func (ss *SetSecurity) Parse(lines []string) error {
	if len(lines) == 0 || (len(lines) == 1 && len(lines[0]) == 0) {
		return nil
	}

	var result []map[string][]string
	const kvParts = 2
	for _, line := range lines {
		kv := strings.SplitN(line, ":", kvParts)
		scopes := []string{}
		var key string

		if len(kv) > 1 {
			scs := strings.SplitSeq(kv[1], ",")
			for scope := range scs {
				tr := strings.TrimSpace(scope)
				if tr != "" {
					tr = strings.SplitAfter(tr, " ")[0]
					scopes = append(scopes, strings.TrimSpace(tr))
				}
			}

			key = strings.TrimSpace(kv[0])

			result = append(result, map[string][]string{key: scopes})
		}
	}

	ss.set(result)

	return nil
}
