// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package grammar

import (
	"net/mail"
	"strings"
)

// Contact is the typed shape of a `contact:` inline value on a swagger:meta block.
//
// The convention is:
//
//	contact: <Name> <email> <URL>
//
// where each part is optional in the order written: the parser recognises a `Name <email>` head
// (Go's net/mail.ParseAddress form) followed by an optional URL.
// A bare email without a name is also accepted.
// Empty or unrecognised inputs return (Contact{}, false) from Block.Contact().
type Contact struct {
	Name, Email, URL string
}

// License is the typed shape of a `license:` inline value:
//
//	license: <Name> <URL>
//
// where Name is everything before the URL prefix and URL is the scheme-anchored remainder.
// A line without a URL keeps Name and leaves URL empty.
// Empty input returns (License{}, false) from Block.License().
type License struct {
	Name, URL string
}

// parseContact converts the raw contact: value into a typed Contact.
//
// Returns (Contact{}, nil) on empty input (treated as "no contact").
// A non-nil error signals a malformed `Name <email>` head — the caller decides whether to fail
// the build or downgrade to a warning.
// An isolated URL (no name/email) yields (Contact{URL: …}, nil).
func parseContact(line string) (Contact, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Contact{}, nil
	}
	nameEmail, url := splitURL(line)
	if nameEmail == "" {
		return Contact{URL: url}, nil
	}
	addr, err := mail.ParseAddress(nameEmail)
	if err != nil {
		return Contact{}, err
	}
	return Contact{Name: addr.Name, Email: addr.Address, URL: url}, nil
}

// parseLicense converts the raw license: value into a typed License.
//
// Returns (License{}, false) only when the input is empty; any non-empty input yields a (License,
// true) with Name and URL split on the URL prefix (Name may be empty if the line starts with the
// URL).
func parseLicense(line string) (License, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return License{}, false
	}
	name, url := splitURL(line)
	return License{Name: name, URL: url}, true
}

// urlSchemes lists the leading URL prefixes splitURL recognises.
// The set covers the schemes meta titles realistically carry.
//
//nolint:gochecknoglobals // immutable lookup table; read-only.
var urlSchemes = []string{"https://", "http://", "ftps://", "ftp://", "wss://", "ws://"}

// splitURL separates the leading non-URL prefix from the trailing URL on a single line.
//
// Returns ("", url) when the line begins with a URL scheme; (line, "") when no scheme is found
// anywhere.
func splitURL(line string) (notURL, url string) {
	str := strings.TrimSpace(line)
	idx := -1
	for _, scheme := range urlSchemes {
		if i := strings.Index(str, scheme); i >= 0 && (idx < 0 || i < idx) {
			idx = i
		}
	}
	if idx < 0 {
		if str != "" {
			notURL = str
		}
		return notURL, ""
	}
	notURL = strings.TrimSpace(str[:idx])
	url = strings.TrimSpace(str[idx:])
	return notURL, url
}
