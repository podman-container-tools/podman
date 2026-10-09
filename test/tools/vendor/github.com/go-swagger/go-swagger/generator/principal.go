// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"regexp"
	"strings"
)

var ifaceRex = regexp.MustCompile(`^interface\s\{\s*\}$`)

// principalIsNullable indicates whether the principal type used for authentication
// may be used as a pointer.
//
// It depends only on the configured principal type and whether the user provided
// a custom (non-nullable) interface.
func principalIsNullable(principal string, customIface bool) bool {
	return principal != iface && !customIface
}

// principalAlias returns an aliased type to the principal.
func principalAlias(principal string) string {
	_, alias, _ := resolvePrincipal(principal)
	return alias
}

// resolvePrincipal splits a configured principal type into its (alias, type, package) parts.
func resolvePrincipal(principal string) (string, string, string) {
	if ifaceRex.MatchString(principal) {
		return "", "any", ""
	}

	dotLocation := strings.LastIndex(principal, ".")
	if dotLocation < 0 {
		return "", principal, ""
	}

	// handle possible conflicts with injected principal package
	// NOTE(fred): we do not check here for conflicts with packages created from operation tags, only standard imports
	alias := deconflictPrincipal(importAlias(principal[:dotLocation]))
	return alias, alias + principal[dotLocation:], principal[:dotLocation]
}
